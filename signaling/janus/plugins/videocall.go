/*
 * Copyright 2017 Kopano and its licensors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3,
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package plugins

import (
	"encoding/json"
	"fmt"
	"sync"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
	"stash.kopano.io/kwm/kwmserver/signaling/janus"
)

const (
	pluginNameVideoCall = "janus.plugin.videocall"
	videoCallQueueSize  = 100
)

type pluginVideoCall struct {
	sync.Mutex
	handleID int64

	users map[string]*videoCallUser
}

func newPluginVideoCall(id int64) janus.Plugin {
	return &pluginVideoCall{
		handleID: id,

		users: make(map[string]*videoCallUser),
	}
}

// VideoCallFactory returns the factory function to create new videoCall plugins.
func VideoCallFactory() (string, func(string, *janus.Manager) (janus.Plugin, error)) {
	return pluginNameVideoCall, func(name string, m *janus.Manager) (janus.Plugin, error) {
		if name != pluginNameVideoCall {
			return nil, fmt.Errorf("invalid plugin name, %s != %s", name, pluginNameVideoCall)
		}
		return newPluginVideoCall(m.NewHandle()), nil
	}
}

type videoCallResult struct {
	Result interface{} `json:"result"`
}

type videoCallUser struct {
	sync.Mutex
	username   string
	connection *connection.Connection
	queue      chan *janus.ResponseData
}

func (p *pluginVideoCall) Name() string {
	return pluginNameVideoCall
}

func (p *pluginVideoCall) HandleID() int64 {
	return p.handleID
}

func (p *pluginVideoCall) OnMessage(m *janus.Manager, c *connection.Connection, msg *janus.MessageMessageData) error {
	var body map[string]interface{}
	err := json.Unmarshal(*msg.Body, &body)
	if err != nil {
		return err
	}

	cr := c.Bound().(*janus.ConnectionRecord)

	request, _ := body["request"]
	switch request {
	case "register":
		un, _ := body["username"]
		username := un.(string)

		p.Lock()
		user, found := p.users[username]
		if !found {
			user = &videoCallUser{
				username:   username,
				connection: c,
			}
			p.users[username] = user
			p.Unlock()
		} else {
			p.Unlock()
			user.Lock()
			if user.connection != nil {
				// XXX(longsleep): For now, forget about old stuff.
				user.queue = nil
				m.Logger().Warnf("janus videocall plugin user already registered %v", username)
			}

			user.connection = c
			if user.queue != nil {
				close(user.queue)
				user.Unlock()
				// Flush queue now.
				m.Logger().Debugln("janus video call plugin flushing channel %v", username)
				for message := range user.queue {
					c.Send(message)
				}
			} else {
				user.Unlock()
			}
		}

		response := &janus.ResponseData{
			Type:   janus.TypeNameEvent,
			ID:     msg.ID,
			Sender: cr.Plugin.HandleID(),
			PluginData: &janus.PluginData{
				PluginName: cr.Plugin.Name(),
				Data: &videoCallResult{
					Result: map[string]interface{}{
						"event":    "registered",
						"username": username,
					},
				},
			},
		}
		err = c.Send(response)

	case "call":
		un, _ := body["username"]
		username := un.(string)

		p.Lock()
		user, found := p.users[username]
		if !found {
			// NOTE(longsleep): It takes a while for the other target to register ... crap!
			user = &videoCallUser{
				username: username,
				queue:    make(chan *janus.ResponseData, videoCallQueueSize),
			}
			p.users[username] = user
		}

		message := &janus.ResponseData{
			Type:   janus.TypeNameEvent,
			ID:     msg.ID,
			Sender: p.HandleID(),
			PluginData: &janus.PluginData{
				PluginName: p.Name(),
				Data: &videoCallResult{
					Result: map[string]interface{}{
						"event":    "incomingcall",
						"username": cr.Username,
					},
				},
			},
			JSEP: msg.JSEP,
		}
		p.Unlock()

		user.Lock()
		connection := user.connection
		if connection == nil {
			select {
			case user.queue <- message:
				//breaks
			default:
				m.Logger().Warnf("janus videocall plugin user queue full")
			}
			user.Unlock()
		} else {
			user.Unlock()
			connection.Send(message)
		}

	case "accept":
		// XXX(longsleep): Bind connections together rather than just using the other one.
		p.Lock()
		var connection *connection.Connection
		for _, user := range p.users {
			if user.connection != c {
				connection = user.connection
				break
			}
		}
		p.Unlock()

		if connection == nil {
			m.Logger().Warnf("janus videocall plugin user for accept not found")
			break
		}

		message := &janus.ResponseData{
			Type:   janus.TypeNameEvent,
			ID:     msg.ID,
			Sender: p.HandleID(),
			PluginData: &janus.PluginData{
				PluginName: p.Name(),
				Data: &videoCallResult{
					Result: map[string]interface{}{
						"event":    "accepted",
						"username": cr.Username,
					},
				},
			},
			JSEP: msg.JSEP,
		}
		connection.Send(message)

	default:
		m.Logger().Debugf("janus videocall plugin unknown request type %v", request)
	}

	return err
}

func (p *pluginVideoCall) OnDetach(m *janus.Manager, c *connection.Connection, msg *janus.EnvelopeData) error {
	cr := c.Bound().(*janus.ConnectionRecord)

	if cr.Username == "" {
		return nil
	}

	p.Lock()
	user, found := p.users[cr.Username]
	cr.Username = ""
	if !found {
		p.Unlock()
		return nil
	}
	delete(p.users, cr.Username)
	p.Unlock()

	user.Lock()
	if user.connection == c {
		user.connection = nil
		close(user.queue)
	}
	user.Unlock()

	return nil
}

func (p *pluginVideoCall) Attach(m *janus.Manager, c *connection.Connection, msg *janus.AttachMessageData, cb func(janus.Plugin), cleanup func(janus.Plugin)) error {
	if cb != nil {
		cb(p)
	}

	return nil
}

func (p *pluginVideoCall) OnAttached(m *janus.Manager, cb func(janus.Plugin)) error {
	if cb != nil {
		cb(p)
	}

	return nil
}

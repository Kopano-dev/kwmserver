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

package janus

import (
	"encoding/json"
	"sync"
)

// Consts for know plugins.
const (
	PluginVideoCallName = "janus.plugin.videocall"

	videoCallQueueSize = 100
)

type pluginVideoCall struct {
	sync.Mutex
	handleID int64

	users map[string]*videoCallUser
}

func newPluginVideoCall(id int64) *pluginVideoCall {
	return &pluginVideoCall{
		handleID: id,

		users: make(map[string]*videoCallUser),
	}
}

type videoCallResult struct {
	Result interface{} `json:"result"`
}

type videoCallUser struct {
	sync.Mutex
	username   string
	connection *Connection
	queue      chan *Response
}

func (p *pluginVideoCall) Name() string {
	return PluginVideoCallName
}

func (p *pluginVideoCall) HandleID() int64 {
	return p.handleID
}

func (p *pluginVideoCall) onMessage(m *Manager, c *Connection, msg *janusMessageMessage) error {
	var body map[string]interface{}
	err := json.Unmarshal(*msg.Body, &body)
	if err != nil {
		return err
	}

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
				m.logger.Warnf("janus videocall plugin user already registered %v", username)
			}

			user.connection = c
			if user.queue != nil {
				close(user.queue)
				user.Unlock()
				// Flush queue now.
				m.logger.Debugln("janus video call plugin flushing channel %v", username)
				for message := range user.queue {
					c.Send(message)
				}
			} else {
				user.Unlock()
			}
		}

		response := &Response{
			Type:   TypeNameEvent,
			ID:     msg.ID,
			Sender: c.plugin.HandleID(),
			PluginData: &PluginData{
				PluginName: c.plugin.Name(),
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
				queue:    make(chan *Response, videoCallQueueSize),
			}
			p.users[username] = user
		}

		message := &Response{
			Type:   TypeNameEvent,
			ID:     msg.ID,
			Sender: p.HandleID(),
			PluginData: &PluginData{
				PluginName: p.Name(),
				Data: &videoCallResult{
					Result: map[string]interface{}{
						"event":    "incomingcall",
						"username": c.username,
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
				m.logger.Warnf("janus videocall plugin user queue full")
			}
			user.Unlock()
		} else {
			user.Unlock()
			connection.Send(message)
		}

	case "accept":
		// XXX(longsleep): Bind connections together rather than just using the other one.
		p.Lock()
		var connection *Connection
		for _, user := range p.users {
			if user.connection != c {
				connection = user.connection
				break
			}
		}
		p.Unlock()

		if connection == nil {
			m.logger.Warnf("janus videocall plugin user for accept not found")
			break
		}

		message := &Response{
			Type:   TypeNameEvent,
			ID:     msg.ID,
			Sender: p.HandleID(),
			PluginData: &PluginData{
				PluginName: p.Name(),
				Data: &videoCallResult{
					Result: map[string]interface{}{
						"event":    "accepted",
						"username": c.username,
					},
				},
			},
			JSEP: msg.JSEP,
		}
		connection.Send(message)

	default:
		m.logger.Warnf("janus videocall plugin unknown request type %v", request)
	}

	return err
}

func (p *pluginVideoCall) onDetach(m *Manager, c *Connection, msg *janusEnvelope) error {
	if c.username == "" {
		return nil
	}

	p.Lock()
	user, found := p.users[c.username]
	c.username = ""
	if !found {
		p.Unlock()
		return nil
	}
	delete(p.users, c.username)
	p.Unlock()

	user.Lock()
	if user.connection == c {
		user.connection = nil
		close(user.queue)
	}
	user.Unlock()

	return nil
}

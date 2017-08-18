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
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/mcu"
	"stash.kopano.io/kwm/kwmserver/signaling/janus"
)

const (
	pluginNameChromiuMCU = "janus.plugin.kopano-chromiumcu"
)

type pluginChromiuMCU struct {
	sync.Mutex
	name     string
	handleID int64
	mcum     *mcu.Manager
	janus    *janus.Manager

	connection          *connection.Connection
	onAttachedCallbacks []func(janus.Plugin)
}

func newPluginChromiuMCU(name string, id int64, mcum *mcu.Manager, janusManager *janus.Manager) janus.Plugin {
	return &pluginChromiuMCU{
		name:     name,
		handleID: id,
		mcum:     mcum,
		janus:    janusManager,
	}
}

// ChromiuMCUFactory returns the factory function to create new chromiuMCU plugins.
func ChromiuMCUFactory(mcum *mcu.Manager) (string, func(string, *janus.Manager) (janus.Plugin, error)) {
	return pluginNameChromiuMCU, func(name string, m *janus.Manager) (janus.Plugin, error) {
		return newPluginChromiuMCU(name, m.NewHandle(), mcum, m), nil
	}
}

func (p *pluginChromiuMCU) Name() string {
	return p.name
}

func (p *pluginChromiuMCU) HandleID() int64 {
	return p.handleID
}

func (p *pluginChromiuMCU) OnMessage(m *janus.Manager, c *connection.Connection, msg *janus.MessageMessageData) error {
	p.Lock()
	mcu := p.connection
	p.Unlock()
	if mcu == nil {
		m.Logger().Errorf("no mcu connection available: %s", []byte(*msg.Body))
		return fmt.Errorf("no mcu connection")
	}

	return mcu.SendTransaction(msg, func(reply []byte) error {
		//m.logger.Debugf("received transaction reply %s, %v", reply, c.ID())
		return p.OnReply(c, reply)
	})
}

func (p *pluginChromiuMCU) OnReply(c *connection.Connection, msg []byte) error {
	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	var envelope janus.EnvelopeData
	err := json.Unmarshal(msg, &envelope)
	if err != nil {
		return err
	}

	if envelope.TargetSession == 0 {
		if envelope.TransactionID() == "" {
			return fmt.Errorf("no target session in reply without transaction")
		}
		return c.RawSend(msg)
	}

	connection := p.janus.GetConnectionBySessionID(envelope.TargetSession)
	if connection == nil {
		return fmt.Errorf("no connection for target session in reply without transaction")
	}

	return connection.RawSend(msg)
}

func (p *pluginChromiuMCU) OnText(c *connection.Connection, msg []byte) error {
	return p.OnReply(c, msg)
}

func (p *pluginChromiuMCU) OnDetach(m *janus.Manager, c *connection.Connection, msg *janus.EnvelopeData) error {
	return nil
}

func (p *pluginChromiuMCU) setConnection(c *connection.Connection, cb func(janus.Plugin)) {
	p.Lock()
	p.connection = c
	if cb != nil {
		cb(p)
	}
	p.Unlock()
}

func (p *pluginChromiuMCU) Attach(m *janus.Manager, c *connection.Connection, msg *janus.AttachMessageData, cb func(janus.Plugin), cleanup func(janus.Plugin)) error {
	mcu, err := p.mcum.Attach(p.Name(), p.HandleID(), func(conn *connection.Connection) error {
		p.setConnection(conn, func(_ janus.Plugin) {
			onAttachedCallbacks := p.onAttachedCallbacks
			p.onAttachedCallbacks = nil
			go func() {
				if cb != nil {
					cb(p)
				}
				for _, attachedCb := range onAttachedCallbacks {
					attachedCb(p)
				}
			}()
		})
		return nil
	}, p.OnText)
	if err != nil {
		m.Logger().Errorf("no mcu connection available for attaching")
		return err
	}

	if cleanup != nil {
		mcu.OnClosed(func(conn *connection.Connection) {
			cleanup(p)
		})
	}

	return nil
}

func (p *pluginChromiuMCU) OnAttached(m *janus.Manager, c *connection.Connection, msg *janus.AttachMessageData, cb func(janus.Plugin), cleanup func(janus.Plugin)) error {
	if cb == nil && cleanup == nil {
		return nil
	}

	p.Lock()
	mcu := p.connection

	if cleanup != nil {
		if mcu != nil {
			mcu.OnClosed(func(conn *connection.Connection) {
				cleanup(p)
			})
		} else {
			p.onAttachedCallbacks = append(p.onAttachedCallbacks, func(_ janus.Plugin) {
				p.Lock()
				attachedMcu := p.connection
				p.Unlock()
				attachedMcu.OnClosed(func(conn *connection.Connection) {
					cleanup(p)
				})
			})
		}
	}

	if cb != nil {
		if mcu != nil {
			p.Unlock()
			go cb(p)
		} else {
			p.onAttachedCallbacks = append(p.onAttachedCallbacks, cb)
			p.Unlock()
		}
	} else {
		p.Unlock()
	}

	return nil
}

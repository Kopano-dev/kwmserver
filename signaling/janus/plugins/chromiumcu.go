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

	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/mcu"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
	"stash.kopano.io/kwm/kwmserver/signaling/janus"
)

const (
	pluginNameChromiuMCU = "janus.plugin.kopano-chromiumcu"
)

type pluginChromiuMCU struct {
	sync.RWMutex

	name     string
	handleID int64

	logger logrus.FieldLogger

	mcum  *mcu.Manager
	janus *janus.Manager

	connection          *connection.Connection
	connecting          bool
	onAttachedCallbacks []func(janus.Plugin)
}

func newPluginChromiuMCU(name string, handleID int64, mcum *mcu.Manager, janusManager *janus.Manager) janus.Plugin {
	return &pluginChromiuMCU{
		name:     name,
		handleID: handleID,

		logger: janusManager.Logger().WithField("handle_id", handleID),

		mcum:  mcum,
		janus: janusManager,
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

func (p *pluginChromiuMCU) Logger() logrus.FieldLogger {
	return p.logger
}

func (p *pluginChromiuMCU) OnMessage(m *janus.Manager, c *connection.Connection, msg *janus.MessageMessageData) error {
	p.Lock()
	mcu := p.connection
	p.Unlock()
	if mcu == nil {
		p.Logger().Errorf("no mcu connection available: %s", []byte(*msg.Body))
		return fmt.Errorf("no mcu connection")
	}

	return mcu.SendTransaction(msg, func(reply []byte) error {
		//p.Logger().Debugf("received transaction reply %s, %v", reply, c.ID())
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

func (p *pluginChromiuMCU) Attach(m *janus.Manager, c *connection.Connection, msg *janus.AttachMessageData, cb func(janus.Plugin), cleanup func(janus.Plugin)) error {
	p.Lock()
	if p.connection != nil || p.connecting {
		p.Unlock()
		return p.onAttached(m, c, msg, cb, cleanup)
	}
	p.connecting = true
	p.Unlock()

	p.Logger().Debugln("attaching with mcu")
	mcu, err := p.mcum.Attach(p.Name(), p.HandleID(), func(conn *connection.Connection) error {
		// connect callback.
		p.Lock()
		if !p.connecting {
			p.Unlock()
			conn.Close()
			errAttaching := fmt.Errorf("lost mcu connection while attaching")
			p.Logger().Debugln(errAttaching)
			return errAttaching
		}

		p.Logger().WithField("mcu_connection", conn.ID()).Debugln("attached with mcu")
		p.connection = conn
		p.connecting = false
		onAttachedCallbacks := p.onAttachedCallbacks
		p.onAttachedCallbacks = nil
		p.Unlock()
		conn.OnClosed(func(closedConnection *connection.Connection) {
			// closed callback
			p.Logger().WithField("mcu_connection", closedConnection.ID()).Debugln("attached mcu connection(a) has closed")
			p.onClosed()
			if cleanup != nil {
				cleanup(p)
			}
		})

		go func() {
			if cb != nil {
				cb(p)
			}
			for _, attachedCb := range onAttachedCallbacks {
				attachedCb(p)
			}
		}()
		return nil
	}, p.OnText)
	if err != nil {
		p.Logger().Errorf("no mcu connection available for attaching")
		return err
	}

	mcu.OnClosed(func(conn *connection.Connection) {
		// closed callback.
		p.Logger().WithField("mcu_connection", conn.ID()).Warnln("mcu connection has closed")
		p.Lock()
		if p.connection != nil {
			p.connection.RawSend(nil) // This closes.
		} else {
			p.connecting = false
		}
		p.Unlock()
	})

	return nil
}

func (p *pluginChromiuMCU) onAttached(m *janus.Manager, c *connection.Connection, msg *janus.AttachMessageData, cb func(janus.Plugin), cleanup func(janus.Plugin)) error {
	if cb == nil && cleanup == nil {
		return nil
	}

	p.Lock()
	attachedConnection := p.connection
	if attachedConnection == nil && !p.connecting {
		p.Unlock()
		return p.Attach(m, c, msg, cb, cleanup)
	}

	if cleanup != nil {
		if attachedConnection != nil {
			attachedConnection.OnClosed(func(closedConnection *connection.Connection) {
				// closed callback.
				p.Logger().WithField("mcu_connection", closedConnection.ID()).Debugln("attached mcu  connection(b) has closed ")
				p.onClosed()
				if cleanup != nil {
					cleanup(p)
				}
			})
		} else {
			p.onAttachedCallbacks = append(p.onAttachedCallbacks, func(_ janus.Plugin) {
				// attached callback.
				p.RLock()
				attachedConnection2 := p.connection
				p.RUnlock()
				attachedConnection2.OnClosed(func(closedConnection *connection.Connection) {
					// closed callback.
					p.Logger().WithField("mcu_connection", closedConnection.ID()).Debugln("attached mcu  connection(c) has closed")
					p.onClosed()
					if cleanup != nil {
						cleanup(p)
					}
				})
			})
		}
	}

	if cb != nil {
		if attachedConnection != nil {
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

func (p *pluginChromiuMCU) onClosed() {
	p.Lock()
	p.connection = nil
	p.Unlock()
}

func (p *pluginChromiuMCU) Enabled() bool {
	p.RLock()
	defer p.RUnlock()
	return p.connection != nil || p.connecting
}

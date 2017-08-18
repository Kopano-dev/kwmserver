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

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// OnConnect is called for new connections.
func (m *Manager) OnConnect(c *connection.Connection) error {
	c.Logger().Debugln("websocket OnConnect")

	return nil
}

// OnDisconnect is called after a connection has closed.
func (m *Manager) OnDisconnect(c *connection.Connection) error {
	c.Logger().Debugln("websocket OnDisconnect")

	return nil
}

// OnBeforeDisconnect is called before a connection is closed. An indication why
// the connection will be closed is provided with the passed error.
func (m *Manager) OnBeforeDisconnect(c *connection.Connection, err error) error {
	//c.Logger().Debugln("websocket OnBeforeDisconnect", err)

	return nil
}

// OnText is called when the provided connection received a text message. The
// message payload is provided as []byte in the msg parameter.
func (m *Manager) OnText(c *connection.Connection, msg []byte) error {
	//c.Logger().Debugf("websocket OnText: %s", msg)
	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	var envelope EnvelopeData
	err := json.Unmarshal(msg, &envelope)
	if err != nil {
		return err
	}

	cr := c.Bound().(*ConnectionRecord)

	switch envelope.Type {
	case TypeNameCreate:
		// Send back success
		response := &ResponseData{
			Type: TypeNameSuccess,
			ID:   envelope.ID,
			Data: map[string]interface{}{
				"id": cr.Session,
			},
		}
		err = c.Send(response)

	case TypeNameAttach:
		var attach AttachMessageData
		err = json.Unmarshal(msg, &attach)
		if err != nil {
			break
		}

		m.plugins.Upsert(attach.PluginName, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
			if exist && valueInMap != nil {
				plugin := valueInMap.(Plugin)
				errPlugin := plugin.OnAttached(m, c, &attach, func(_ Plugin) {
					// Send back success
					response := &ResponseData{
						Type: TypeNameSuccess,
						ID:   envelope.ID,
						Data: map[string]interface{}{
							"id": plugin.HandleID(),
						},
					}
					errSuccess := c.Send(response)
					if errSuccess != nil {
						m.Logger().WithError(errSuccess).Errorf("failed to send success after plugin on attach")
					}
				}, func(p Plugin) {
					// Cleanup when plugin wants to.
					cr.Lock()
					cr.Plugin = nil
					cr.Unlock()
				})
				if errPlugin != nil {
					m.Logger().WithError(err).Errorf("failed to attach existing plugin")
					err = errPlugin
				} else {
					cr.Lock()
					cr.Plugin = plugin
					cr.Unlock()
				}

				return valueInMap
			}

			plugin, errPlugin := m.LaunchPlugin(attach.PluginName)
			if errPlugin != nil {
				m.Logger().WithError(err).Errorf("failed to launch plugin")
				err = errPlugin
				return nil
			}

			errPlugin = plugin.Attach(m, c, &attach, func(p Plugin) {
				// Send back success
				response := &ResponseData{
					Type: TypeNameSuccess,
					ID:   envelope.ID,
					Data: map[string]interface{}{
						"id": p.HandleID(),
					},
				}
				errSuccess := c.Send(response)
				if errSuccess != nil {
					m.Logger().WithError(errSuccess).Errorf("failed to send success after plugin attach")
				}
			}, func(p Plugin) {
				// Cleanup when plugin wants to.
				cr.Lock()
				cr.Plugin = nil
				cr.Unlock()
				m.plugins.Remove(attach.PluginName)
			})
			if errPlugin != nil {
				m.Logger().WithError(errPlugin).Errorf("failed to attach plugin")
				err = errPlugin
				return nil
			}

			cr.Lock()
			cr.Plugin = plugin
			cr.Unlock()

			return plugin
		})

	case TypeNameDestroy:
		// Close connection when done here.
		defer func() {
			// Send back for confirmation.
			response := &EnvelopeData{
				Type:    TypeNameDestroy,
				ID:      envelope.ID,
				Session: envelope.Session,
			}
			c.Send(response)
			c.RawSend(nil) // This closes, once everything has been sent.
		}()

		cr.RLock()
		if cr.Plugin == nil {
			cr.RUnlock()
			break
		}

		cr.RUnlock()
		// Fall through to detach if with plugin.
		fallthrough

	case TypeNameDetach:
		cr.RLock()
		if cr.Plugin == nil {
			cr.Unlock()
			m.logger.Warnln("janus detach without attached plugin")
			break
		}

		plugin := cr.Plugin
		cr.RUnlock()

		err = plugin.OnDetach(m, c, &envelope)
		if err != nil {
			cr.Lock()
			if cr.Plugin == plugin {
				cr.Plugin = nil
			}
			cr.Unlock()
		}

	case TypeNameMessage:
		var message MessageMessageData
		err = json.Unmarshal(msg, &message)
		if err != nil {
			break
		}

		cr.RLock()
		if cr.Plugin == nil {
			cr.RUnlock()
			m.logger.Warnln("janus message without attached plugin")
			break
		}

		plugin := cr.Plugin
		cr.RUnlock()

		err = plugin.OnMessage(m, c, &message)

	case TypeNameTrickle:
		var trickle TrickleMessageData
		err = json.Unmarshal(msg, &trickle)
		if err != nil {
			break
		}

		cr.RLock()
		if cr.Plugin == nil {
			cr.RUnlock()
			m.logger.Warnln("janus trickle without attached plugin")
			break
		}

		plugin := cr.Plugin
		cr.RUnlock()

		trickle.ID = "" // We do not want a transaction for trickle.
		err = plugin.OnMessage(m, c, &MessageMessageData{
			EnvelopeData: trickle.EnvelopeData,
			Body:         trickle.Candidate,
		})

	case TypeNameKeepAlive:
		// breaks

	default:
		m.logger.Warnf("janus unknown incoming janus type %v", envelope.Type)
	}

	return err
}

// OnError is called, when the provided connection has encountered an error. The
// provided error is the error encountered. Any return value other than nil,
// will result in a close of the connection.
func (m *Manager) OnError(c *connection.Connection, err error) error {
	return err
}

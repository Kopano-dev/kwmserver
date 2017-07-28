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
)

func (m *Manager) onConnect(c *Connection) error {
	c.logger.Debugln("janus websocket onConnect")

	return nil
}

func (m *Manager) onDisconnect(c *Connection) error {
	c.logger.Debugln("janus websocket onDisconnect")

	return nil
}

func (m *Manager) onBeforeDisconnect(c *Connection, err error) error {
	//c.logger.Debugln("websocket onBeforeDisconnect", err)

	return nil
}

func (m *Manager) onText(c *Connection, msg []byte) error {
	c.logger.Debugf("janus websocket onText: %s", msg)

	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	var envelope janusEnvelope
	err := json.Unmarshal(msg, &envelope)
	if err != nil {
		return err
	}

	switch envelope.Type {
	case TypeNameCreate:
		// Send back success
		response := &Response{
			Type: TypeNameSuccess,
			ID:   envelope.ID,
			Data: map[string]interface{}{
				"id": c.session,
			},
		}
		err = c.Send(response)

	case TypeNameAttach:
		var attach janusAttachMessage
		err = json.Unmarshal(msg, &attach)
		if err != nil {
			break
		}

		switch attach.PluginName {
		case PluginVideoCallName:
			m.plugins.Upsert(attach.PluginName, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
				// TODO(longsleep): Allow to attach multiple plugins to a connection.
				if exist {
					newValue.(*Connection).plugin = valueInMap.(Plugin)
					return valueInMap
				}
				c.plugin = newPluginVideoCall(c.session)
				return c.plugin
			})
		default:
			m.logger.Warnf("unknown janus plugin %v", attach.PluginName)
		}

		// Send back success
		response := &Response{
			Type: TypeNameSuccess,
			ID:   envelope.ID,
			Data: map[string]interface{}{
				"id": c.plugin.HandleID(),
			},
		}
		err = c.Send(response)

	case TypeNameDestroy:
		// TODO(longsleep): Close connection.

		// Fall through to detach if with plugin.
		if c.plugin == nil {
			break
		}
		fallthrough

	case TypeNameDetach:
		if c.plugin == nil {
			m.logger.Warnln("janus detach without attached plugin")
			break
		}
		err = c.plugin.onDetach(m, c, &envelope)
		if err != nil {
			c.plugin = nil
		}

	case TypeNameMessage:
		var message janusMessageMessage
		err = json.Unmarshal(msg, &message)
		if err != nil {
			break
		}

		if c.plugin == nil {
			m.logger.Warnln("janus message without attached plugin")
			break
		}
		err = c.plugin.onMessage(m, c, &message)

	case TypeNameKeepAlive:
		// breaks

	default:
		m.logger.Warnf("unknown incoming janus typ %v", envelope.Type)
	}

	return err
}

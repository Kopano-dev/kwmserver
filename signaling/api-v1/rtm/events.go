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

package rtm

import (
	"encoding/json"
	"fmt"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"

	"github.com/gorilla/websocket"
)

func (m *Manager) onConnect(c *Connection) error {
	//c.logger.Debugln("websocket onConnect")

	if c.user != nil {
		// Add user to table.
		m.users.Upsert(c.user.id, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
			if !exist {
				// No connection for that user.
				return []*Connection{newValue.(*Connection)}
			}

			connections := append(valueInMap.([]*Connection), newValue.(*Connection))
			return connections
		})
	}

	err := c.RawSend(rawRTMTypeHelloMessage)
	return err
}

func (m *Manager) onDisconnect(c *Connection) error {
	//c.logger.Debugln("websocket onDisconnect")

	if c.user != nil {
		// XXX(longsleep): This can remove the wrong connection if it was
		// already replaced.
		m.users.Pop(c.user.id)
	}

	return nil
}

func (m *Manager) onBeforeDisconnect(c *Connection, err error) error {
	//c.logger.Debugln("websocket onBeforeDisconnect", err)

	if err == nil {
		err = c.write(rawRTMTypeGoodbyeMessage, websocket.TextMessage)
		return err
	}

	return nil
}

func (m *Manager) onText(c *Connection, msg []byte) error {
	//c.logger.Debugf("websocket onText: %s", msg)

	// TODO(longsleep): Reuse RTMDataEnvelope / put into pool.
	var envelope api.RTMTypeEnvelope
	err := json.Unmarshal(msg, &envelope)
	if err != nil {
		return err
	}

	err = nil
	switch envelope.Type {
	case api.RTMTypeNamePing:
		// Ping, Pong.
		var ping api.RTMTypePingPong
		err = json.Unmarshal(msg, &ping)
		if err != nil {
			break
		}
		// Send back same data as pong.
		ping["type"] = api.RTMTypeNamePong
		err = c.Send(&ping)

	case api.RTMTypeNameWebRTC:
		// WebRTC.
		var webrtc api.RTMTypeWebRTC
		err = json.Unmarshal(msg, &webrtc)
		if err != nil {
			break
		}
		err = m.onWebRTC(c, &webrtc)

	default:
		return fmt.Errorf("unknown incoming type %v", envelope.Type)
	}

	return err
}

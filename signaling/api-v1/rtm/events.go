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
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// OnConnect is called for new connections.
func (m *Manager) OnConnect(c *connection.Connection) error {
	c.Logger().Debugln("websocket OnConnect")

	bound := c.Bound()
	if bound != nil {
		// Add user to table.
		nur := bound.(*userRecord)
		first := false
		nur.Lock()
		m.users.Upsert(nur.id, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
			if !exist {
				// No connection for that user.
				nur.connections = append(nur.connections, newValue.(*connection.Connection))
				nur.when = time.Now()
				first = true
				return nur
			}

			connection := newValue.(*connection.Connection)
			ur := valueInMap.(*userRecord)
			ur.Lock()
			ur.connections = append(ur.connections, connection)
			// TODO(longsleep): Limit maximum number of connections.
			ur.Unlock()

			// Overwrite the connections user record.
			connection.Bind(ur)

			return ur
		})
		nur.Unlock()

		if first {
			// This was the users first connection.
			m.logger.WithField("user_id", nur.id).Debugln("user is now active")
		}
	}

	err := c.RawSend(rawRTMTypeHelloMessage)
	return err
}

// OnDisconnect is called after a connection has closed.
func (m *Manager) OnDisconnect(c *connection.Connection) error {
	c.Logger().Debugln("websocket OnDisconnect")

	bound := c.Bound()
	if bound != nil {
		ur := bound.(*userRecord)
		m.users.Upsert(ur.id, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
			if !exist {
				m.logger.WithFields(logrus.Fields{
					"user_id":       ur.id,
					"connection_id": c.ID(),
				}).Warnln("disconnect user connection without user map entry - this should not happen")
				return &userRecord{
					id: c.ID(),
				}
			}

			ur := valueInMap.(*userRecord)
			ur.Lock()
			connections := make([]*connection.Connection, len(ur.connections)-1)
			offset := 0
			for idx, connection := range ur.connections {
				if connection == c {
					offset++
					continue
				}
				connections[idx-offset] = connection
			}
			if len(connections) == 0 {
				ur.exit = time.Now()
			}
			ur.connections = connections
			ur.Unlock()

			return ur
		})
	}

	c.Logger().Debugln("websocket onDisconnect done")
	return nil
}

// OnBeforeDisconnect is called before a connection is closed. An indication why
// the connection will be closed is provided with the passed error.
func (m *Manager) OnBeforeDisconnect(c *connection.Connection, err error) error {
	//c.Logger().Debugln("websocket OnBeforeDisconnect", err)

	if err == nil {
		err = c.Write(rawRTMTypeGoodbyeMessage, websocket.TextMessage)
		return err
	}

	return nil
}

// OnText is called when the provided connection received a text message. The
// message payload is provided as []byte in the msg parameter.
func (m *Manager) OnText(c *connection.Connection, msg []byte) error {
	//c.Logger().Debugf("websocket OnText: %s", msg)

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

// OnError is called, when the provided connection has encountered an error. The
// provided error is the error encountered. Any return value other than nil,
// will result in a close of the connection.
func (m *Manager) OnError(c *connection.Connection, err error) error {
	switch err.(type) {
	case *api.RTMTypeError:
		// Send out known errors to connection.
		c.Send(err)
		return nil
	default:
		// breaks
	}

	return err
}

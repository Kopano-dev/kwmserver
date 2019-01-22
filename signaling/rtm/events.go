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

	"github.com/sirupsen/logrus"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
)

// OnConnect is called for new connections.
func (m *Manager) OnConnect(c *connection.Connection) error {
	c.Logger().Debugln("websocket OnConnect")

	var self *api.Self
	bound := c.Bound()
	if bound != nil {
		// Add user to table.
		nur := bound.(*userRecord)
		first := false
		nur.Lock()
		entry := m.users.Upsert(nur.id, c, func(exist bool, valueInMap interface{}, newValue interface{}) interface{} {
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

		// Fill self with user record.
		ur := entry.(*userRecord)
		self = &api.Self{
			ID:   ur.id,
			Name: ur.auth.Name(),
		}

		if first {
			// This was the users first connection.
			m.logger.WithField("user_id", nur.id).Debugln("user is now active")
		}
	}

	// Send hello.
	msg := &api.RTMTypeHello{
		Type: api.RTMTypeNameHello,
		Self: self,
	}
	err := c.Send(msg)

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
		msg := &api.RTMTypeHello{
			Type: api.RTMTypeNameGoodbye,
		}

		err = c.Send(msg)
		return err
	}

	return nil
}

// OnText is called when the provided connection received a text message. The
// message payload is provided as []byte in the msg parameter.
func (m *Manager) OnText(c *connection.Connection, msg []byte) error {
	//c.Logger().Debugf("websocket OnText: %s", msg)

	// TODO(longsleep): Reuse RTMTypeTransaction / put into pool.
	var transaction api.RTMTypeTransaction
	err := json.Unmarshal(msg, &transaction)
	if err != nil {
		return err
	}
	if onReply, ok := c.Transaction(&transaction, func() (connection.TransactionCallbackFunc, bool) {
		// TODO(longsleep): Figure out what to do when the transaction was
		// not found.
		return nil, false
	}); ok {
		return onReply(msg)
	}

	return m.processTextMessage(c, &transaction, msg)
}

func (m *Manager) processTextMessage(c *connection.Connection, transaction *api.RTMTypeTransaction, msg []byte) error {
	var err error
	switch transaction.Type {
	case api.RTMTypeNamePing:
		// Ping, Pong.
		var ping api.RTMTypePingPong
		err = json.Unmarshal(msg, &ping)
		if err != nil {
			break
		}
		// Refresh user record data.
		bound := c.Bound()
		if bound == nil {
			// Do not reply to pings which do not have a user.
			return nil
		}
		ur := bound.(*userRecord)
		if ur.auth == nil {
			// Do not reply to pings without auth in user record.
			return nil
		}
		if m.adminm.RefreshAdminAuthToken(ur.auth) {
			if time.Unix(ur.auth.ExpiresAt, 0).Before(time.Now().Add(time.Minute * 15)) {
				// Inject updated auth
				newToken, errToken := m.adminm.SignAdminAuthToken(ur.auth)
				if errToken != nil {
					return fmt.Errorf("failed to create new signed token on ping reply: %v", errToken)
				}
				ping["auth"] = newToken
			}
		} else {
			// TODO(longsleep): Check for updated auth, if it has expired, close connection.
			if time.Unix(ur.auth.ExpiresAt, 0).Before(time.Now().Add(time.Minute * 1)) {
				//c.Logger().Debugln("websocket ping with expired auth")
			}
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
		return fmt.Errorf("unknown incoming type %v", transaction.Type)
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

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

package mcu

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// HandleWebsocketConnect checks the presence of the key in cache and returns a
// new connection if key is found.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, transaction string, rw http.ResponseWriter, req *http.Request) error {
	var ar *attachedRecord
	if transaction != "" {
		record, ok := m.attached.Get(transaction)
		if !ok {
			http.NotFound(rw, req)
			return nil
		}
		ar = record.(*attachedRecord)
		ar.Lock()
		defer ar.Unlock()
		if ar.connection != nil || ar.onConnect == nil {
			http.NotFound(rw, req)
			return nil
		}
	}

	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		m.logger.WithError(err).Debugln("websocket handshake error")
		return nil
	} else if err != nil {
		return err
	}
	if ws.Subprotocol() != websocketSubProtocolName {
		m.logger.Debugln("websocket bad subprotocol")
		return nil
	}

	id := strconv.FormatUint(atomic.AddUint64(&m.count, 1), 10)

	loggerFields := logrus.Fields{
		"mcu_connection": id,
	}

	c, err := connection.New(ctx, ws, m, m.logger.WithFields(loggerFields), id)
	if err != nil {
		return err
	}

	if ar == nil {
		go m.serveWebsocketConnection(c, id)
	} else {
		ar.connection = c
		ar.onConnect(c)
		ar.onConnect = nil
		c.OnClosed(func(conn *connection.Connection) {
			ar.Lock()
			ar.connection = nil
			ar.Unlock()
		})
		c.Bind(ar)
		go c.ServeWS(m.Context())
	}

	return nil
}

func (m *Manager) serveWebsocketConnection(c *connection.Connection, id string) {
	m.connectionsMutex.Lock()
	element := m.connections.PushBack(c)
	m.connectionsMutex.Unlock()

	c.ServeWS(m.Context())

	m.connectionsMutex.Lock()
	m.connections.Remove(element)
	m.connectionsMutex.Unlock()
}

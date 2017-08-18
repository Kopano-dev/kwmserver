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
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// ConnectionRecord is used as binder between janus data and connections.
type ConnectionRecord struct {
	sync.RWMutex

	Session  int64
	Username string
	Plugin   Plugin
}

// HandleWebsocketConnect handles Janus protocol websocket connections.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
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

	session := atomic.AddUint64(&m.count, 1)
	id := strconv.FormatUint(session, 10)

	loggerFields := logrus.Fields{
		"janus_connection": id,
	}

	c, err := connection.New(ctx, ws, m, m.logger.WithFields(loggerFields), id)
	if err != nil {
		return err
	}

	cr := &ConnectionRecord{
		Session: int64(session),
	}
	c.Bind(cr)
	go m.serveWebsocketConnection(c, id)

	return nil
}

func (m *Manager) serveWebsocketConnection(c *connection.Connection, id string) {
	m.connections.Set(id, c)
	c.ServeWS(m.Context())
	m.connections.Remove(id)
}

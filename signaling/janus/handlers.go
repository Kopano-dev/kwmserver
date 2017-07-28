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
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// HandleWebsocketConnect handles Janus protocol websocket connections.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		m.logger.WithError(err).Debugln("websocket handshake error")
		return nil
	} else if err != nil {
		return err
	}
	m.logger.Debugln("selected subprotocol", websocket.Subprotocols(req), ws.Subprotocol())
	if ws.Subprotocol() != websocketSubProtocolName {
		m.logger.Debugln("websocket bad subprotocol")
		return nil
	}

	session := atomic.AddUint64(&m.count, 1)
	id := strconv.FormatUint(session, 10)

	loggerFields := logrus.Fields{
		"janus_connection": id,
	}

	conn := &Connection{
		ws:      ws,
		ctx:     ctx,
		mgr:     m,
		logger:  m.logger.WithFields(loggerFields),
		id:      id,
		session: int64(session),
		start:   time.Now(),
		send:    make(chan []byte, 256),
		ping:    make(chan *pingRecord, 5),
	}

	m.connections.Set(id, conn)
	conn.ServeWS(m.Context())
	m.connections.Remove(id)

	return nil
}

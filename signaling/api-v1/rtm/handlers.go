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
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// HandleWebsocketConnect checks the presence of the key in cache and returns a
// new connection if key is found.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, key string, rw http.ResponseWriter, req *http.Request) error {
	record, ok := m.keys.Pop(key)
	if !ok {
		http.NotFound(rw, req)
		return nil
	}

	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		m.logger.WithError(err).Debugln("websocket handshake error")
		return nil
	} else if err != nil {
		return err
	}

	kr := record.(*keyRecord)
	id := strconv.FormatUint(atomic.AddUint64(&m.count, 1), 10)

	loggerFields := logrus.Fields{
		"rtm_connection": id,
	}
	if kr.user != nil {
		loggerFields["user"] = kr.user.id
	}
	conn := &Connection{
		ws:     ws,
		ctx:    ctx,
		mgr:    m,
		logger: m.logger.WithFields(loggerFields),
		id:     id,
		user:   kr.user,
		start:  time.Now(),
		send:   make(chan []byte, 256),
		ping:   make(chan *pingRecord, 5),
	}

	m.connections.Set(id, conn)
	conn.ServeWS(m.Context())
	m.connections.Remove(id)

	return nil
}

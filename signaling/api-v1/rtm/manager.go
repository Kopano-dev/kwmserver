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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
	"stash.kopano.io/kc/konnect/rndm"
)

// Manager handles RTM connect state.
type Manager struct {
	ID     string
	logger logrus.FieldLogger
	ctx    context.Context

	keys     cmap.ConcurrentMap
	upgrader *websocket.Upgrader

	count       uint64
	connections cmap.ConcurrentMap
}

type pingRecord struct {
	id   uint64
	when time.Time
}

type keyRecord struct {
	when time.Time
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger) *Manager {
	m := &Manager{
		ID:     id,
		logger: logger,
		ctx:    ctx,

		keys: cmap.New(),
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  websocketReadBufferSize,
			WriteBufferSize: websocketWriteBufferSize,
		},

		connections: cmap.New(),
	}

	// Cleanup function.
	go func() {
		ticker := time.NewTicker(connectCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.purgeExpiredKeys()
			case <-ctx.Done():
				return
			}

		}
	}()

	return m
}

func (m *Manager) purgeExpiredKeys() {
	expired := make([]string, 0)
	deadline := time.Now().Add(-connectExpiration)
	var record *keyRecord
	for entry := range m.keys.IterBuffered() {
		record = entry.Val.(*keyRecord)
		if record.when.Before(deadline) {
			expired = append(expired, entry.Key)
		}
	}
	for _, key := range expired {
		m.keys.Remove(key)
	}
}

// Connect adds a new connect sentry to the managers table with random key.
func (m *Manager) Connect(ctx context.Context) (string, error) {
	key, err := rndm.GenerateRandomString(connectKeySize)
	if err != nil {
		return "", err
	}

	// Add key to table.
	record := &keyRecord{
		when: time.Now(),
	}
	m.keys.Set(key, record)

	return key, nil
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() int {
	return m.connections.Count()
}

// HandleWebsocketConnect checks the presence of the key in cache and returns a
// new connection if key is found.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, key string, rw http.ResponseWriter, req *http.Request) error {
	if _, ok := m.keys.Pop(key); !ok {
		http.NotFound(rw, req)
		return nil
	}

	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		return nil
	} else if err != nil {
		return err
	}

	id := strconv.FormatUint(atomic.AddUint64(&m.count, 1), 10)
	conn := &Connection{
		ws:     ws,
		ctx:    ctx,
		mgr:    m,
		logger: m.logger.WithField("rtm_connection", id),

		id:    id,
		start: time.Now(),
		send:  make(chan []byte, 256),
		ping:  make(chan *pingRecord, 5),
	}

	m.connections.Set(id, conn)
	conn.ServeWS(m.Context())
	m.connections.Remove(id)

	return nil
}

func (m *Manager) onConnect(c *Connection) error {
	c.logger.Debugln("websocket onConnect")

	err := c.Send(api.RTMTypeHelloMessage)
	return err
}

func (m *Manager) onDisconnect(c *Connection) error {
	c.logger.Debugln("websocket onDisconnect")
	return nil
}

func (m *Manager) onText(c *Connection, msg []byte) error {
	c.logger.Debugf("websocket onText: %s", msg)

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
		err = c.Send(ping)

	default:
		return fmt.Errorf("unknown incoming type %v", envelope.Type)
	}

	return err
}

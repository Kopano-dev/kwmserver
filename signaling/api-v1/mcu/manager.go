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
	"container/list"
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
	"stash.kopano.io/kgol/rndm"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// Manager handles RTM connect state.
type Manager struct {
	id     string
	logger logrus.FieldLogger
	ctx    context.Context

	upgrader *websocket.Upgrader

	count            uint64
	handles          uint64
	connections      *list.List
	connectionsMutex sync.RWMutex
	connection       *list.Element

	attached cmap.ConcurrentMap
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger) *Manager {
	m := &Manager{
		id:     id,
		logger: logger.WithField("manager", "mcu"),
		ctx:    ctx,

		upgrader: &websocket.Upgrader{
			Subprotocols:    []string{websocketSubProtocolName},
			ReadBufferSize:  websocketReadBufferSize,
			WriteBufferSize: websocketWriteBufferSize,
			CheckOrigin: func(req *http.Request) bool {
				// TODO(longsleep): Check if its a good idea to allow all origins.
				return true
			},
		},

		connections: list.New(),
		attached:    cmap.New(),
	}

	// Cleanup function.
	go func() {
		ticker := time.NewTicker(connectCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.purgeExpiredAttached()
			case <-ctx.Done():
				return
			}
		}
	}()

	return m
}

type attachedRecord struct {
	sync.Mutex
	when       time.Time
	connection *connection.Connection
	onConnect  func(*connection.Connection) error
	onText     func(*connection.Connection, []byte) error
}

func (m *Manager) purgeExpiredAttached() {
	expired := make([]string, 0)
	deadline := time.Now().Add(-connectExpiration)
	var record *attachedRecord
	for entry := range m.attached.IterBuffered() {
		record = entry.Val.(*attachedRecord)
		record.Lock()
		if record.connection == nil && record.when.Before(deadline) {
			expired = append(expired, entry.Key)
			record.onConnect = nil
			record.onText = nil
		}
		record.Unlock()
	}
	for _, id := range expired {
		m.attached.Remove(id)
	}
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() uint64 {
	m.connectionsMutex.RLock()
	defer m.connectionsMutex.RUnlock()
	n := m.connections.Len()
	m.logger.Debugf("active connections: %d", n)
	return uint64(n)
}

// NewHandle returns the next available handle id of the accociated manager.
func (m *Manager) NewHandle() int64 {
	return int64(atomic.AddUint64(&m.handles, 1))
}

// GetConnection returns a connection from the accociated connections table
func (m *Manager) GetConnection() *connection.Connection {
	m.connectionsMutex.Lock()
	defer m.connectionsMutex.Unlock()

	if m.connection != nil {
		m.connection = m.connection.Next()
	}
	if m.connection == nil {
		m.connection = m.connections.Front()
	}
	if m.connection == nil {
		return nil
	}

	return m.connection.Value.(*connection.Connection)
}

// Attach sends the attach message with the provided parameters.
func (m *Manager) Attach(plugin string, handle int64, onConnect func(*connection.Connection) error, onText func(*connection.Connection, []byte) error) (*connection.Connection, error) {
	transaction := rndm.GenerateRandomString(12)

	c := m.GetConnection()
	if c == nil {
		return nil, fmt.Errorf("no mcu connection available for attaching")
	}

	// TODO(longsleep): Implement timeout.
	m.attached.Set(transaction, &attachedRecord{
		when:      time.Now(),
		onConnect: onConnect,
		onText:    onText,
	})
	err := c.Send(&WebsocketMessage{
		Type:   "attach",
		ID:     transaction,
		Plugin: plugin,
		Handle: handle,
	})

	return c, err
}

// Pipeline creates a Pipeline with the accociated manager and the provided
// properties.
func (m *Manager) Pipeline(plugin string, id string) *Pipeline {
	handle := m.NewHandle()

	p := &Pipeline{
		logger: m.logger.WithField("pipeline_handle", handle),

		plugin: plugin,
		handle: handle,
		id:     id,

		m: m,
	}
	p.logger.WithFields(logrus.Fields{
		"id":     id,
		"plugin": plugin,
	}).Debugln("pipeline create")

	return p
}

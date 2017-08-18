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
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/mcu"
)

// Manager handles Janus protocol websocket connect state.
type Manager struct {
	id        string
	logger    logrus.FieldLogger
	ctx       context.Context
	mcum      *mcu.Manager
	factories map[string]func(string, *Manager) (Plugin, error)

	upgrader *websocket.Upgrader

	count       uint64
	handles     uint64
	connections cmap.ConcurrentMap
	plugins     cmap.ConcurrentMap
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger, mcum *mcu.Manager, factories map[string]func(string, *Manager) (Plugin, error)) *Manager {
	m := &Manager{
		id:        id,
		logger:    logger.WithField("manager", "janus"),
		ctx:       ctx,
		mcum:      mcum,
		factories: factories,

		upgrader: &websocket.Upgrader{
			Subprotocols:    []string{websocketSubProtocolName},
			ReadBufferSize:  websocketReadBufferSize,
			WriteBufferSize: websocketWriteBufferSize,
			CheckOrigin: func(req *http.Request) bool {
				// TODO(longsleep): Check if its a good idea to allow all origins.
				return true
			},
		},

		connections: cmap.New(),
		plugins:     cmap.New(),
	}

	return m
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() uint64 {
	n := m.connections.Count()
	m.Logger().Debugf("active connections: %d", n)
	return uint64(n)
}

// GetConnectionBySessionID returns the connection identified by the provided
// session.
func (m *Manager) GetConnectionBySessionID(session int64) *connection.Connection {
	id := strconv.FormatInt(session, 10)
	c, found := m.connections.Get(id)
	if !found {
		return nil
	}

	return c.(*connection.Connection)
}

// Logger returns the accociated logger.
func (m *Manager) Logger() logrus.FieldLogger {
	return m.logger
}

// NewHandle returns the next available handle id of the accociated manager.
func (m *Manager) NewHandle() int64 {
	return int64(atomic.AddUint64(&m.handles, 1))
}

// LaunchPlugin creates a new instance of the requested plugin.
func (m *Manager) LaunchPlugin(name string) (Plugin, error) {
	factory, ok := m.factories[name]
	if !ok {
		// Try default with empty name.
		factory, _ = m.factories[""]
	}
	if factory != nil {
		return factory(name, m)
	}

	return nil, fmt.Errorf("unknown plugin: %s", name)
}

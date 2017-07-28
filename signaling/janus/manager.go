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

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
)

// Manager handles Janus protocol websocket connect state.
type Manager struct {
	id     string
	logger logrus.FieldLogger
	ctx    context.Context

	upgrader *websocket.Upgrader

	count       uint64
	connections cmap.ConcurrentMap
	plugins     cmap.ConcurrentMap
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger) *Manager {
	m := &Manager{
		id:     id,
		logger: logger,
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
func (m *Manager) NumActive() int {
	return m.connections.Count()
}

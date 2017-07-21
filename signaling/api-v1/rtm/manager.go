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
	"time"

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
	users       cmap.ConcurrentMap
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
		users:       cmap.New(),
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

// Connect adds a new connect entry to the managers table with random key.
func (m *Manager) Connect(ctx context.Context, userID string) (string, error) {
	key, err := rndm.GenerateRandomString(connectKeySize)
	if err != nil {
		return "", err
	}

	// Add key to table.
	record := &keyRecord{
		when: time.Now(),
	}
	if userID != "" {
		record.user = &userRecord{
			id: userID,
		}
	}
	m.keys.Set(key, record)

	return key, nil
}

// LookupConnectionsByUserID returns the active connections for a given user.
func (m *Manager) LookupConnectionsByUserID(userID string) ([]*Connection, bool) {
	connections, ok := m.users.Get(userID)
	if !ok {
		return nil, false
	}

	return connections.([]*Connection), true
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

type pingRecord struct {
	id   uint64
	when time.Time
}

type userRecord struct {
	id string
}

type keyRecord struct {
	when time.Time
	user *userRecord
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

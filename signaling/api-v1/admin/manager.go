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

package admin

import (
	"context"
	"sync"
	"time"

	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
)

const (
	tokenCleanupInterval = time.Duration(30) * time.Second
	tokenExpiration      = time.Duration(1) * time.Minute
)

// Manager handles admin state.
type Manager struct {
	id     string
	logger logrus.FieldLogger
	ctx    context.Context

	tokens             cmap.ConcurrentMap
	tokensKeys         map[string][]byte
	tokensSigningKeyID string
	tokensMutex        sync.RWMutex
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger) *Manager {
	m := &Manager{
		id:     id,
		logger: logger.WithField("manager", "admin"),
		ctx:    ctx,

		tokens:             cmap.New(),
		tokensKeys:         make(map[string][]byte),
		tokensSigningKeyID: "",
	}

	// Cleanup function.
	go func() {
		ticker := time.NewTicker(tokenCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.purgeExpiredTokens()
			case <-ctx.Done():
				return
			}
		}
	}()

	return m
}

type tokenRecord struct {
	when  time.Time
	token interface{}
}

func (m *Manager) purgeExpiredTokens() {
	m.tokensMutex.Lock()
	defer m.tokensMutex.Unlock()

	expired := make([]string, 0)
	deadline := time.Now().Add(-tokenExpiration)
	var record *tokenRecord
	for entry := range m.tokens.IterBuffered() {
		record = entry.Val.(*tokenRecord)
		if record.when.Before(deadline) {
			expired = append(expired, entry.Key)
		}
	}
	for _, id := range expired {
		m.RemoveToken(id)
	}
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() uint64 {
	return 0
}

// Logger returns the accociated logger.
func (m *Manager) Logger() logrus.FieldLogger {
	return m.logger
}

// AddTokenSigningKey adds the provided key value to the known signing keys
// of the accociated manager and sets the id as signing key id.
func (m *Manager) AddTokenSigningKey(id string, value []byte) {
	m.AddTokenKey(id, value)
	m.tokensSigningKeyID = id
}

// AddTokenKey adds the provided key value to the known singing keys of the
// accociated manager.
func (m *Manager) AddTokenKey(id string, value []byte) {
	m.tokensKeys[id] = value
}

// SetToken adds a token with the provided id and token value to the accociated
// manager.
func (m *Manager) SetToken(id string, token interface{}) error {
	//m.logger.Debugln("set token", id)
	m.tokens.Set(id, &tokenRecord{time.Now(), token})

	return nil
}

// GetToken returns the token identified by id from the accociated manager.
func (m *Manager) GetToken(id string) (interface{}, bool) {
	record, exists := m.tokens.Get(id)
	if !exists {
		return nil, false
	}

	return record.(*tokenRecord).token, true
}

// HasToken checks if the provided id is know as token to the accociated manager.
func (m *Manager) HasToken(id string) bool {
	return m.tokens.Has(id)
}

// RemoveToken removes the token identified by id from the accociated manager.
func (m *Manager) RemoveToken(id string) {
	//m.logger.Debugln("remove token", id)
	m.tokens.Remove(id)
}

// PopToken removes and returns the token identified by id from the accociated
// manager.
func (m *Manager) PopToken(id string) (interface{}, bool) {
	record, exists := m.tokens.Pop(id)
	if !exists {
		return nil, false
	}
	//m.logger.Debugln("pop token", id)

	return record.(*tokenRecord).token, true
}

// RefreshToken updates the timestamp of the token identified by id if that
// token is know to the accociated manager.
func (m *Manager) RefreshToken(id string) {
	//m.logger.Debugln("refreshing token", id)
	m.tokensMutex.RLock()
	m.tokens.Upsert(id, nil, func(exists bool, valueInMap interface{}, newValue interface{}) interface{} {
		if !exists {
			m.logger.Warnln("unknown token during refresh", id)
			return &tokenRecord{}
		}

		record := valueInMap.(*tokenRecord)
		record.when = time.Now()
		return record
	})
	m.tokensMutex.RUnlock()
}

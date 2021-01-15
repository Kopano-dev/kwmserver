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
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
	kcoidc "stash.kopano.io/kc/libkcoidc"
	"stash.kopano.io/kgol/rndm"

	"stash.kopano.io/kwm/kwmserver/signaling/admin"
	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
	"stash.kopano.io/kwm/kwmserver/signaling/guest"
	"stash.kopano.io/kwm/kwmserver/signaling/mcu"
	"stash.kopano.io/kwm/kwmserver/turn"
)

// Manager handles RTM connect state.
type Manager struct {
	id                    string
	insecure              bool
	requiredScopes        []string
	pipelineForcedPattern *regexp.Regexp

	logger  logrus.FieldLogger
	ctx     context.Context
	mcum    *mcu.Manager
	adminm  *admin.Manager
	guestm  *guest.Manager
	oidcp   *kcoidc.Provider
	turnsrv turn.Server

	keys     cmap.ConcurrentMap
	upgrader *websocket.Upgrader

	serverStatus atomic.Value

	count       uint64
	handles     uint64
	connections cmap.ConcurrentMap
	users       cmap.ConcurrentMap
	channels    cmap.ConcurrentMap
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, insecure bool, requiredScopes []string, pipelineForcedPatternString string, logger logrus.FieldLogger, mcum *mcu.Manager, adminm *admin.Manager, guestm *guest.Manager, oidcp *kcoidc.Provider, turnsrv turn.Server) *Manager {
	m := &Manager{
		id:             id,
		insecure:       insecure,
		requiredScopes: requiredScopes,

		logger:  logger.WithField("manager", "rtm"),
		ctx:     ctx,
		mcum:    mcum,
		adminm:  adminm,
		guestm:  guestm,
		oidcp:   oidcp,
		turnsrv: turnsrv,

		keys: cmap.New(),
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  websocketReadBufferSize,
			WriteBufferSize: websocketWriteBufferSize,
			CheckOrigin: func(req *http.Request) bool {
				// TODO(longsleep): Check if its a good idea to allow all origins.
				return true
			},
		},

		connections: cmap.New(),
		users:       cmap.New(),
		channels:    cmap.New(),
	}

	m.serverStatus.Store(&api.ServerStatus{})

	if pipelineForcedPatternString != "" {
		if pipelineForcedPattern, err := regexp.Compile(pipelineForcedPatternString); err == nil {
			m.pipelineForcedPattern = pipelineForcedPattern
			m.logger.Infoln("pattern", pipelineForcedPattern.String(), "forced pipline channels enabled")
		} else {
			m.logger.WithError(err).Errorln("failed to parse forced pipeline pattern regexp - forced pipeline channels not enabled")
		}
	}

	// Cleanup function.
	go func() {
		ticker := time.NewTicker(connectCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.purgeExpiredKeys()
				m.purgeEmptyChannels()
				m.purgeInactiveUsers()
			case <-ctx.Done():
				return
			}
		}
	}()

	return m
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

type userRecord struct {
	sync.RWMutex
	id          string
	auth        *api.AdminAuthToken
	when        time.Time
	exit        time.Time
	connections []*connection.Connection
}

func (m *Manager) purgeInactiveUsers() {
	now := time.Now()
	empty := make([]*userRecord, 0)
	deadline := now.Add(-channelExpiration)
	var record *userRecord
	for entry := range m.users.IterBuffered() {
		record = entry.Val.(*userRecord)
		record.Lock()
		if len(record.connections) == 0 {
			m.logger.WithFields(logrus.Fields{
				"user_id":     record.id,
				"connections": len(record.connections),
				"duration":    now.Sub(record.when),
			}).Debugf("user inactive")
			if record.exit.Before(deadline) {
				empty = append(empty, record)
			}
		}
		record.Unlock()
	}
	for _, record := range empty {
		record.Lock()
		if len(record.connections) == 0 {
			m.users.Remove(record.id)
			record.Unlock()
			m.logger.WithFields(logrus.Fields{
				"user_id":  record.id,
				"duration": record.exit.Sub(record.when),
			}).Debugln("user offline")
			userCleanup.WithLabelValues(m.id).Inc()
		} else {
			record.Unlock()
		}
	}
}

type channelRecord struct {
	when    time.Time
	channel *Channel
}

func (m *Manager) purgeEmptyChannels() {
	empty := make([]cmap.Tuple, 0)
	deadline := time.Now().Add(-channelExpiration)
	var record *channelRecord
	for entry := range m.channels.IterBuffered() {
		record = entry.Val.(*channelRecord)
		if record.channel.CanBeCleanedUp() {
			if record.when.Before(deadline) {
				// Kill channels which can be cleaned up.
				empty = append(empty, entry)
			}
		}
	}
	for _, entry := range empty {
		record = entry.Val.(*channelRecord)
		if record.channel.Cleanup() {
			m.logger.WithField("channel", entry.Key).Debugln("channel purge")
			m.channels.Remove(entry.Key)
		}
	}
}

func (m *Manager) getServerStatus() *api.ServerStatus {
	serverStatus := m.serverStatus.Load()
	if serverStatus == nil {
		return nil
	}
	return serverStatus.(*api.ServerStatus)
}

func (m *Manager) setServerStatus(serverStatus *api.ServerStatus) {
	m.serverStatus.Store(serverStatus)
}

// Connect adds a new connect entry to the managers table with random key.
func (m *Manager) Connect(ctx context.Context, userID string, auth *api.AdminAuthToken) (string, error) {
	key := rndm.GenerateRandomString(connectKeySize)

	// Add key to table.
	record := &keyRecord{
		when: time.Now(),
	}
	if userID != "" {
		record.user = &userRecord{
			id:   userID,
			auth: auth,
		}
	}
	m.keys.Set(key, record)

	return key, nil
}

// LookupConnectionsByID returns a copy slice of the active connections for the
// user accociated with the provided ID.
func (m *Manager) LookupConnectionsByID(id string) ([]*connection.Connection, bool) {
	// Find connections provided by the id, potentially transforming the provided
	// id to something useful.
	ur, ok := m.lookupUserEntryByID(id)
	if !ok {
		return nil, false
	}

	ur.RLock()
	connections := make([]*connection.Connection, len(ur.connections))
	copy(connections, ur.connections)
	ur.RUnlock()

	return connections, true
}

func (m *Manager) lookupUserEntryByID(id string) (*userRecord, bool) {
	var entry interface{}
	var ok bool
	for {
		if entry, ok = m.users.Get(id); ok {
			// Fastpath.
			break
		}

		return nil, false
	}

	if entry == nil {
		if entry, ok = m.users.Get(id); !ok {
			return nil, false
		}
	}

	return entry.(*userRecord), true
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() uint64 {
	n := m.connections.Count()
	m.logger.Debugf("active connections: %d", n)
	return uint64(n)
}

// NewHandle returns the next available handle id of the accociated manager.
func (m *Manager) NewHandle() int64 {
	return int64(atomic.AddUint64(&m.handles, 1))
}

// Pipeline returns a pipleline which fits to the accociated Manager.
func (m *Manager) Pipeline(scope string, id string) Pipeline {
	if m.mcum == nil {
		return nil
	}

	switch scope {
	case mcu.PluginIDKWMRTMChannel:
		withPipeline := false
		if m.pipelineForcedPattern != nil {
			if m.pipelineForcedPattern.MatchString(id) {
				withPipeline = true
			}
		}
		if !withPipeline {
			return nil
		}
		if m.mcum == nil {
			m.logger.WithField("scope", scope).Debugln("mcu is disabled - ignoring pipeline request")
			return nil
		}

		// Route calls through mcum pipeline.
		return m.mcum.Pipeline(scope, id)

	default:
		m.logger.WithField("scope", scope).Warnln("ignoring pipeline request for unknown scope")
	}

	return nil
}

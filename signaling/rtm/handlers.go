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
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
)

// HandleWebsocketConnect checks the presence of the key in cache and starts
// the websocket connection in a new go routine. Returns with nil when the
// websocket connection was successfully started, otherwise with error.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, key string, rw http.ResponseWriter, req *http.Request) error {
	record, ok := m.keys.Pop(key)
	if !ok {
		http.NotFound(rw, req)
		return nil
	}

	kr := record.(*keyRecord)
	if kr.user == nil || kr.user.auth == nil {
		http.Error(rw, "", http.StatusForbidden)
		return nil
	}

	// Validate cached token to ensure it is still valid.
	var err error
	switch kr.user.auth.Type {
	case api.AdminAuthTokenTypeToken:
		if !m.adminm.IsValidAdminAuthToken(kr.user.auth) {
			err = errors.New("invalid or expired token")
			break
		}

	case "Bearer":
		if m.oidcp == nil {
			err = errors.New("bearer auth not enabled")
			break
		}
		// NOTE(longsleep): Ensure that access token in our record is still
		// valid.
		_, _, _, validateErr := m.oidcp.ValidateTokenString(ctx, kr.user.auth.Value)
		if validateErr != nil {
			err = validateErr
			break
		}
	}

	// Fail with error.
	if err != nil {
		m.logger.WithError(err).Debugln("websocket connect forbidden")
		http.Error(rw, err.Error(), http.StatusForbidden)
		return nil
	}

	// All good, initiate websocket.
	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		m.logger.WithError(err).Debugln("websocket handshake error")
		return nil
	} else if err != nil {
		return err
	}

	// Refresh token.
	m.adminm.RefreshAdminAuthToken(kr.user.auth)

	// Prepare connection and bind user.
	id := strconv.FormatUint(atomic.AddUint64(&m.count, 1), 10)
	loggerFields := logrus.Fields{
		"rtm_connection": id,
	}
	if kr.user != nil {
		loggerFields["user_id"] = kr.user.id
	}
	c, err := connection.New(ctx, ws, m, m.logger.WithFields(loggerFields), id)
	if err != nil {
		return err
	}
	c.Bind(kr.user)
	go m.serveWebsocketConnection(c, id)

	return nil
}

func (m *Manager) serveWebsocketConnection(c *connection.Connection, id string) {
	connectionAdd.WithLabelValues(m.id).Inc()
	m.connections.Set(id, c)
	c.ServeWS(m.Context())
	m.connections.Remove(id)
	connectionRemove.WithLabelValues(m.id).Inc()
}

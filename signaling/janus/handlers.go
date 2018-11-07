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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/connection"
)

const (
	maxRequestSize = 1024 * 5

	janusTokenTokensRecordIDPrefix = "janus"
)

func getJanusTokenTokensRecordID(value string) string {
	return fmt.Sprintf("%s-%s", janusTokenTokensRecordIDPrefix, value)
}

// ConnectionRecord is used as binder between janus data and connections.
type ConnectionRecord struct {
	sync.RWMutex

	Session  int64
	Username string
	Plugin   Plugin
}

// HandleAdmin handles Janus admin connections.
func (m *Manager) HandleAdmin(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	msg, err := ioutil.ReadAll(io.LimitReader(req.Body, maxRequestSize))
	if err != nil {
		m.Logger().WithError(err).Debugln("janus failed to read admin request body")
		http.Error(rw, fmt.Errorf("failed to read request: %v", err).Error(), http.StatusBadRequest)
		return nil
	}

	var envelope EnvelopeData
	err = json.Unmarshal(msg, &envelope)
	if err != nil {
		m.Logger().WithError(err).Debugln("janus failed to parse admin request")
		http.Error(rw, fmt.Errorf("failed to parse: %v", err).Error(), http.StatusBadRequest)
		return nil
	}

	//TODO(longsleep): Validate admin_secret.

	switch envelope.Type {
	case TypeNameAdminAddToken:
		var addToken AdminAddTokenData
		err = json.Unmarshal(msg, &addToken)
		if err != nil {
			m.Logger().WithError(err).Debugln("janus failed to parse admin add_token request")
			http.Error(rw, fmt.Errorf("failed to parse: %v", err).Error(), http.StatusBadRequest)
			return nil
		}

		m.adminm.SetToken(getJanusTokenTokensRecordID(addToken.Token), nil)

		response := &ResponseData{
			Type: TypeNameSuccess,
			ID:   addToken.ID,
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)

	default:
		m.logger.Warnf("janus unknown incoming admin janus type %v", envelope.Type)
		http.Error(rw, "unknown janus type", http.StatusBadRequest)
	}

	return nil
}

// HandleWebsocketConnect handles Janus protocol websocket connections.
func (m *Manager) HandleWebsocketConnect(ctx context.Context, rw http.ResponseWriter, req *http.Request) error {
	ws, err := m.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		m.logger.WithError(err).Debugln("websocket handshake error")
		return nil
	} else if err != nil {
		return err
	}
	if ws.Subprotocol() != websocketSubProtocolName {
		m.logger.Debugln("websocket bad subprotocol")
		return nil
	}

	session := atomic.AddUint64(&m.count, 1)
	id := strconv.FormatUint(session, 10)

	loggerFields := logrus.Fields{
		"janus_connection": id,
	}

	c, err := connection.New(ctx, ws, m, m.logger.WithFields(loggerFields), id)
	if err != nil {
		return err
	}

	cr := &ConnectionRecord{
		Session: int64(session),
	}
	c.Bind(cr)
	go m.serveWebsocketConnection(c, id)

	return nil
}

func (m *Manager) serveWebsocketConnection(c *connection.Connection, id string) {
	m.connections.Set(id, c)
	c.ServeWS(m.Context())
	m.connections.Remove(id)
}

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

package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"

	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"github.com/sirupsen/logrus"
	"stash.kopano.io/kc/konnect/rndm"
)

const (
	rtmConnectExpiration      = time.Duration(30) * time.Second
	rtmConnectCleanupInterval = time.Duration(1) * time.Minute
	rtmConnectKeySize         = 24
	rtmConnectionIDSize       = 24

	// Buffer sizes.
	websocketReadBufferSize  = 1024
	websocketWriteBufferSize = 1024

	// Maximum message size allowed from peer.
	websocketMaxMessageSize = 1048576 // 100 KiB

	// Time allowed to write a message to the peer.
	websocketWriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	websocketPongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	websocketPingPeriod = (websocketPongWait * 9) / 10
)

// RTMManager handles RTMP connect state.
type RTMManager struct {
	ID     string
	logger logrus.FieldLogger
	ctx    context.Context

	keys     cmap.ConcurrentMap
	upgrader *websocket.Upgrader

	connections cmap.ConcurrentMap
}

// RTMConnection binds the websocket connection to the manager.
type RTMConnection struct {
	ID string

	ws  *websocket.Conn
	ctx context.Context
	mgr *RTMManager

	// TODO(longsleep): Make this a doubly link list.
	send chan []byte
}

// readPump reads from the underlaying websocket connection until close.
func (c *RTMConnection) readPump(ctx context.Context) error {
	defer func() {
		c.mgr.onDisconnect(c)
		c.Close()
	}()

	c.ws.SetReadLimit(websocketMaxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(websocketPongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(websocketPongWait))
		return nil
	})

	c.mgr.onConnect(c)

	err := func() error {
		for {
			op, r, err := c.ws.NextReader()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					c.mgr.logger.WithError(err).Debugln("websocket read error")
					return err
				}
				return nil
			}
			switch op {
			case websocket.TextMessage:
				// TODO(longsleep): Reuse []byte, probably put into bytes.Buffer.
				var b []byte
				b, err = ioutil.ReadAll(io.LimitReader(r, websocketMaxMessageSize))
				if err != nil {
					c.mgr.logger.Debugln("websocket read text error", err)
					break
				}
				err = c.mgr.onText(c, b)
				if err != nil {
					c.mgr.logger.Debugln("websocket text error", err)
					break
				}
			}

			if err != nil {
				return err
			}
		}

		return nil
	}()

	return err
}

// writePump writes to the underlaying websocket connection.
func (c *RTMConnection) writePump(ctx context.Context) error {
	ticker := time.NewTicker(websocketPingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(websocketWriteWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return nil
			}

			w, err := c.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				c.mgr.logger.WithError(err).Debugln("websocket write pump error")
				return err
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				c.mgr.logger.WithError(err).Debugln("websocket write error")
				return err
			}

		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(websocketWriteWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				c.mgr.logger.WithError(err).Debugln("websocket write ping error")
				return err
			}
		}
	}
}

// Close closes the underlaying websocket connection.
func (c *RTMConnection) Close() error {
	return c.ws.Close()
}

// Send places the message into the send queue in a non-blocking way
func (c *RTMConnection) Send(message interface{}) error {
	b, err := json.MarshalIndent(message, "", "\t")
	if err != nil {
		c.mgr.logger.Errorln("websocket send marshal failed: %v", err)
		return err
	}

	select {
	case c.send <- b:
		// ok
	default:
		// channel full?
		c.mgr.logger.Warnln("websocket send channel full")
		return fmt.Errorf("queue full")
	}

	return nil
}

// ServeWS serves the Websocket protocol for the accociated client connections
// and returns once either of the connections are closed.
func (c *RTMConnection) ServeWS(ctx context.Context) {
	go func() {
		err := c.writePump(ctx)
		if err != nil {
			c.mgr.logger.WithError(err).Warn("websocket write pump exit")
		}
	}()
	err := c.readPump(ctx)
	if err != nil {
		c.mgr.logger.WithError(err).Warn("websocket read pump exit")
	}
}

type rtmRecord struct {
	when time.Time
}

// NewRTMManager creates a new RTMManager with an id.
func NewRTMManager(ctx context.Context, id string, logger logrus.FieldLogger) *RTMManager {
	rtm := &RTMManager{
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
		ticker := time.NewTicker(rtmConnectCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rtm.purgeExpired()
			case <-ctx.Done():
				return
			}

		}
	}()

	return rtm
}

func (rtm *RTMManager) purgeExpired() {
	expired := make([]string, 0)
	deadline := time.Now().Add(-rtmConnectExpiration)
	var record *rtmRecord
	for entry := range rtm.keys.IterBuffered() {
		record = entry.Val.(*rtmRecord)
		if record.when.Before(deadline) {
			expired = append(expired, entry.Key)
		}
	}
	for _, key := range expired {
		rtm.keys.Remove(key)
	}
}

// Connect adds a new connect sentry to the managers table with random key.
func (rtm *RTMManager) Connect(ctx context.Context) (string, error) {
	key, err := rndm.GenerateRandomString(rtmConnectKeySize)
	if err != nil {
		return "", err
	}

	// Add key to table.
	record := &rtmRecord{
		when: time.Now(),
	}
	rtm.keys.Set(key, record)

	return key, nil
}

// Context Returns the Context of the associated manager.
func (rtm *RTMManager) Context() context.Context {
	return rtm.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (rtm *RTMManager) NumActive() int {
	return rtm.connections.Count()
}

// HandleWebsocketConnect checks the presence of the key in cache and returns a
// new connection if key is found.
func (rtm *RTMManager) HandleWebsocketConnect(ctx context.Context, key string, rw http.ResponseWriter, req *http.Request) error {
	if _, ok := rtm.keys.Pop(key); !ok {
		http.NotFound(rw, req)
		return nil
	}

	ws, err := rtm.upgrader.Upgrade(rw, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		return nil
	} else if err != nil {
		return err
	}

	id, err := rndm.GenerateRandomString(rtmConnectionIDSize)
	if err != nil {
		return err
	}

	conn := &RTMConnection{
		ID: id,

		ws:  ws,
		ctx: ctx,
		mgr: rtm,

		send: make(chan []byte, 256),
	}

	rtm.connections.Set(id, conn)
	conn.ServeWS(rtm.Context())
	rtm.connections.Remove(id)

	return nil
}

func (rtm *RTMManager) onConnect(c *RTMConnection) error {
	rtm.logger.Debugln("websocket onConnect")

	err := c.Send(api.RTMTypeHelloMessage)
	return err
}

func (rtm *RTMManager) onDisconnect(c *RTMConnection) error {
	rtm.logger.Debugln("websocket onDisconnect")
	return nil
}

func (rtm *RTMManager) onText(c *RTMConnection, msg []byte) error {
	rtm.logger.Debugf("websocket onText: %s", msg)

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

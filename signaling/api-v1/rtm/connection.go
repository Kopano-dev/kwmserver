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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

// Connection binds the websocket connection to the manager.
type Connection struct {
	ws     *websocket.Conn
	ctx    context.Context
	mgr    *Manager
	logger logrus.FieldLogger

	// TODO(longsleep): Make this a doubly link list.
	send chan []byte

	id       string
	start    time.Time
	duration time.Duration
	ping     chan *pingRecord

	user *userRecord

	onClosedCallbacks []connectionClosedFunc
}

type connectionClosedFunc func(*Connection)

type pingRecord struct {
	id   uint64
	when time.Time
}

// readPump reads from the underlaying websocket connection until close.
func (c *Connection) readPump(ctx context.Context) error {
	defer func() {
		c.mgr.onDisconnect(c)
		c.Close()
	}()

	c.ws.SetReadLimit(websocketMaxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(websocketPongWait))
	c.ws.SetPongHandler(func(payload string) error {
		if payload == "" {
			return nil
		}
		var lastPing *pingRecord
		func() {
			for {
				// Drain channel.
				select {
				case lastPing = <-c.ping:
					// Ping from channel.
				default:
					return
				}
			}
		}()
		if lastPing == nil {
			return nil
		}
		payloadInt := binary.LittleEndian.Uint64([]byte(payload))
		if payloadInt != lastPing.id {
			// Ignore everything which does not match our last sent ping.
			return nil
		}
		c.ws.SetReadDeadline(time.Now().Add(websocketPongWait))
		return nil
	})

	c.mgr.onConnect(c)

	for {
		// Wait on incoming data from websocket.
		op, r, err := c.ws.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.logger.WithError(err).Debugln("websocket read error")
				return err
			}
			break
		}

		// Process data based on op.
		switch op {
		case websocket.TextMessage:
			// TODO(longsleep): Reuse []byte, probably put into bytes.Buffer.
			// Rread incoming message into memory.
			var b []byte
			b, err = ioutil.ReadAll(io.LimitReader(r, websocketMaxMessageSize))
			if err != nil {
				c.logger.Debugln("websocket read text error", err)
				return err
			}
			// Process incoming text message..
			err = c.mgr.onText(c, b)
			if err != nil {
				switch err.(type) {
				case *api.RTMTypeError:
					// Send out known errors to connection.
					c.Send(err)
					// breaks
				default:
					// Exit for all other errors.
					c.logger.Debugln("websocket text error", err)
					return err
				}
			}

		default:
			c.logger.Warnln("websocket received unsupported op: %v", op)
		}
	}

	return nil
}

// writePump writes to the underlaying websocket connection.
func (c *Connection) writePump(ctx context.Context) error {
	var err error

	ticker := time.NewTicker(websocketPingPeriod)
	defer func() {
		ticker.Stop()
		c.mgr.onBeforeDisconnect(c, err)
		c.ws.Close()
	}()

	ping := &pingRecord{}
	for {
		select {
		case <-ctx.Done():
			err = nil
			return nil

		case payload, ok := <-c.send:
			if !ok {
				c.ws.SetWriteDeadline(time.Now().Add(websocketWriteWait))
				c.ws.WriteMessage(websocket.CloseMessage, rawZeroBytes)
				err = errors.New("send channel closed")
				return nil
			}

			err = c.write(payload, websocket.TextMessage)
			if err != nil {
				c.logger.WithError(err).Debugln("websocket write pump error")
				return err
			}

		case <-ticker.C:
			ping.id++
			payload := make([]byte, 8)
			binary.LittleEndian.PutUint64(payload, ping.id)
			thisPing := &pingRecord{
				id:   ping.id,
				when: time.Now(),
			}
			select {
			case c.ping <- thisPing:
				// ok
			default:
				// last ping was not received, hang up.
				c.logger.Debugln("websocket ping still pending")
				return nil
			}

			c.ws.SetWriteDeadline(thisPing.when.Add(websocketWriteWait))
			if err = c.ws.WriteMessage(websocket.PingMessage, payload); err != nil {
				c.logger.WithError(err).Debugln("websocket write ping error")
				return err
			}
		}
	}
}

// Close closes the underlaying websocket connection.
func (c *Connection) Close() {
	c.ws.Close()
	close(c.send)
	c.duration = time.Since(c.start)
	for _, cb := range c.onClosedCallbacks {
		cb(c)
	}
	c.onClosedCallbacks = nil
}

// OnClosed registers a callback to be caled after the connection has closed.
func (c *Connection) OnClosed(cb connectionClosedFunc) {
	c.onClosedCallbacks = append(c.onClosedCallbacks, cb)
}

// Duration returns the duration since the start of the connection until the
// client was closed or until now when the accociated connection is not yet
// closed.
func (c *Connection) Duration() time.Duration {
	if c.duration > 0 {
		return c.duration
	}
	return time.Since(c.start)
}

// Send encodes the provided message with JSON and then adds the encoded message
// into the send queue in a non-blocking way.
func (c *Connection) Send(message interface{}) error {
	b, err := json.MarshalIndent(message, "", "\t")
	if err != nil {
		c.logger.Errorln("websocket send marshal failed: %v", err)
		return err
	}

	return c.RawSend(b)
}

// RawSend adds the pprovided payload data into the send queue in a non blocking
// way.
func (c *Connection) RawSend(payload []byte) error {
	select {
	case c.send <- payload:
		// ok
	default:
		// channel full?
		c.logger.Warnln("websocket send channel full")
		return fmt.Errorf("queue full")
	}

	return nil
}

func (c *Connection) write(payload []byte, messageType int) error {
	c.ws.SetWriteDeadline(time.Now().Add(websocketWriteWait))

	w, err := c.ws.NextWriter(messageType)
	if err != nil {
		return fmt.Errorf("failed to get writer: %v", err)
	}
	w.Write(payload)
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	return nil
}

// ServeWS serves the Websocket protocol for the accociated client connections
// and returns once either of the connections are closed.
func (c *Connection) ServeWS(ctx context.Context) {
	go func() {
		err := c.writePump(ctx)
		if err != nil {
			c.logger.WithError(err).Warn("websocket write pump exit")
		}
	}()
	err := c.readPump(ctx)
	if err != nil {
		c.logger.WithError(err).Warn("websocket read pump exit")
	}
}

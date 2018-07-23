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

package connection

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	// Maximum message size allowed from peer.
	websocketMaxMessageSize = 1048576 // 100 KiB

	// Time allowed to write a message to the peer.
	websocketWriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	websocketPongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	websocketPingPeriod = (websocketPongWait * 9) / 10
)

// A Connection binds the websocket connection to a manager.
type Connection struct {
	ws     *websocket.Conn
	mgr    Manager
	logger logrus.FieldLogger

	// TODO(longsleep): Make this a doubly link list.
	send   chan []byte
	mutex  sync.RWMutex
	closed bool

	id       string
	start    time.Time
	duration time.Duration
	ping     chan *pingRecord

	binder interface{}

	onClosedCallbacks []ClosedFunc

	transactions      map[string]TransactionCallbackFunc
	transactionsMutex sync.Mutex
}

// New creates a new Connection with the provided options and settings.
func New(ctx context.Context, ws *websocket.Conn, mgr Manager, logger logrus.FieldLogger, id string) (*Connection, error) {
	return &Connection{
		ws:     ws,
		mgr:    mgr,
		logger: logger,
		id:     id,

		start: time.Now(),
		send:  make(chan []byte, 256),
		ping:  make(chan *pingRecord, 5),

		transactions: make(map[string]TransactionCallbackFunc),
	}, nil
}

// ClosedFunc is a type for functions usable as closed callback.
type ClosedFunc func(*Connection)

// TransactionCallbackFunc is a tyoe for functions usable as transaction callback.
type TransactionCallbackFunc func([]byte) error

// PayloadWithTransactionID is an interface for data with a transaction ID.
type PayloadWithTransactionID interface {
	TransactionID() string
}

type pingRecord struct {
	id   uint64
	when time.Time
}

// readPump reads from the underlaying websocket connection until close.
func (c *Connection) readPump(ctx context.Context) error {
	defer func() {
		c.Close()
		c.mgr.OnDisconnect(c)
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

	c.mgr.OnConnect(c)

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
			err = c.mgr.OnText(c, b)
			if err != nil {
				err = c.mgr.OnError(c, err)
				if err != nil {
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
		c.mgr.OnBeforeDisconnect(c, err)
		errClose := c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""), time.Now().Add(websocketWriteWait))
		if errClose != nil {
			c.Logger().WithError(errClose).Debugln("websocket close write error")
		}

		c.ws.Close()
	}()

	ping := &pingRecord{}
	for {
		select {
		case <-ctx.Done():
			err = nil
			return nil

		case payload, ok := <-c.send:
			if !ok || payload == nil {
				c.logger.Debugln("websocket send channel closed or nil sent")
				err = errors.New("send channel closed")
				return nil
			}

			err = c.Write(payload, websocket.TextMessage)
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
func (c *Connection) Close() error {
	c.logger.Debugln("close() called")
	c.mutex.Lock()
	if c.closed {
		c.logger.Debugln("close() already closed")
		c.mutex.Unlock()
		return nil
	}
	c.closed = true
	c.mutex.Unlock()

	// Close send channel, this aborts writePump, which closes our underlaying
	// websocket which then will result in abort of readPump.
	close(c.send)

	c.mutex.Lock()
	c.duration = time.Since(c.start)
	onClosedCallbacks := c.onClosedCallbacks
	c.onClosedCallbacks = nil
	c.mutex.Unlock()

	for _, cb := range onClosedCallbacks {
		cb(c)
	}

	return nil
}

// OnClosed registers a callback to be caled after the connection has closed.
func (c *Connection) OnClosed(cb ClosedFunc) {
	c.mutex.Lock()
	c.onClosedCallbacks = append(c.onClosedCallbacks, cb)
	c.mutex.Unlock()
}

// IsClosed returns whever or not the accociated Connection is closed.
func (c *Connection) IsClosed() bool {
	c.mutex.RLock()
	closed := c.closed
	c.mutex.RUnlock()

	return closed
}

// Duration returns the duration since the start of the connection until the
// client was closed or until now when the accociated connection is not yet
// closed.
func (c *Connection) Duration() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
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

// SendTransaction encodes the provided transaction message with JSON, registers
// the transaction ID with the provided callback. When the transaction ID is
// seen again, for the incoming messagge, the callback is run.
func (c *Connection) SendTransaction(message PayloadWithTransactionID, cb TransactionCallbackFunc) error {
	tid := message.TransactionID()
	if tid == "" {
		return c.Send(message)
	}

	c.transactionsMutex.Lock()
	c.transactions[tid] = cb
	c.transactionsMutex.Unlock()

	return c.Send(message)
}

// RawSend adds the pprovided payload data into the send queue in a non blocking
// way.
func (c *Connection) RawSend(payload []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return fmt.Errorf("send to closed connection")
	}

	select {
	case c.send <- payload:
		// ok
	default:
		c.mutex.RUnlock()
		// channel full?
		c.logger.Warnln("websocket send channel full")
		return fmt.Errorf("queue full")
	}
	c.mutex.RUnlock()

	return nil
}

func (c *Connection) Write(payload []byte, messageType int) error {
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

// Transaction returns the accociated transaction callback for the provided id.
func (c *Connection) Transaction(msg PayloadWithTransactionID) (TransactionCallbackFunc, bool) {
	tid := msg.TransactionID()
	if tid == "" {
		return nil, false
	}

	c.transactionsMutex.Lock()
	cb, ok := c.transactions[tid]
	if ok {
		delete(c.transactions, tid)
	}
	c.transactionsMutex.Unlock()

	if !ok {
		return nil, false
	}
	return cb, true
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

// Logger returns the accociated connection's logger.
func (c *Connection) Logger() logrus.FieldLogger {
	return c.logger
}

// Bound returns the accociated connection's binder property which was set by
// a previous call to Bind().
func (c *Connection) Bound() interface{} {
	return c.binder
}

// Bind stores the provided binder with the connection. The stored value can
// be received with a calll to Bound().
func (c *Connection) Bind(binder interface{}) error {
	c.binder = binder
	return nil
}

// ID returns the connection's ID.
func (c *Connection) ID() string {
	return c.id
}

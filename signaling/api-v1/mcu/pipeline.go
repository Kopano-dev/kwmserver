/*
 * Copyright 2018 Kopano and its licensors
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

package mcu

import (
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

type onConnectCallbackFunc func(*connection.Connection) error

const pipelineModeMCUForward = "mcu-forward"

// Pipeline is a pipeline to forward rtm messages to the mcu.
type Pipeline struct {
	mutex  sync.RWMutex
	logger logrus.FieldLogger

	plugin string
	handle int64
	id     string

	m *Manager

	onConnectHandler func() error
	onTextHandler    func([]byte) error

	connecting  bool
	closed      bool
	reconnector *time.Timer
	connection  *connection.Connection
}

// ID returns the accociated Pipeline's ID.
func (p *Pipeline) ID() string {
	return p.id
}

// Mode returns the accociated Pipeline's Mode.
func (p *Pipeline) Mode() string {
	return pipelineModeMCUForward
}

// Connect sends the attach message with the provided parameters.
func (p *Pipeline) Connect(onConnect func() error, onText func([]byte) error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return errors.New("pipeline is closed")
	}

	p.onConnectHandler = onConnect
	p.onTextHandler = onText

	// Ask manager to connect.
	p.connecting = true
	_, err := p.m.Attach(p.plugin, p.handle, p.onConnect, p.onText)
	if err != nil {
		p.logger.WithError(err).Warnln("failed to connect pipeline")
		p.reconnect()
	}

	return nil
}

func (p *Pipeline) onConnect(conn *connection.Connection) error {
	p.mutex.Lock()
	p.connecting = false
	p.connection = conn
	p.mutex.Unlock()
	conn.OnClosed(p.onClosed)

	p.logger.Debugln("pipleline connect")
	return p.onConnectHandler()
}

func (p *Pipeline) onClosed(conn *connection.Connection) {
	p.mutex.Lock()
	if conn != p.connection {
		// Wrong connection.
		p.mutex.Unlock()
		return
	}

	p.logger.Debugln("pipeline connection closed")

	if p.connecting || p.closed {
		p.mutex.Unlock()
		return
	}

	p.reconnect()
	p.mutex.Unlock()
}

func (p *Pipeline) reconnect() {
	p.logger.Debugln("scheduling pipeline reconnect")

	if p.reconnector != nil {
		p.reconnector.Stop()
	}

	p.reconnector = time.AfterFunc(3*time.Second, func() {
		p.mutex.Lock()
		if p.connecting || p.closed {
			p.mutex.Unlock()
			return
		}

		// Ask manager to connect.
		p.connecting = true
		_, err := p.m.Attach(p.plugin, p.handle, p.onConnect, p.onText)
		// TODO(longsleep): Implement background retry on error or timeout.
		if err != nil {
			p.logger.WithError(err).Warnln("failed to reconnect pipeline")
			p.reconnect()
		}

		p.mutex.Unlock()
	})
}

func (p *Pipeline) onText(conn *connection.Connection, data []byte) error {
	p.mutex.RLock()
	if conn != p.connection {
		// Wrong connection.
		p.mutex.RUnlock()
		return nil
	}
	closed := p.closed
	p.mutex.RUnlock()

	if closed {
		return nil
	}

	return p.onTextHandler(data)
}

// Send implements the rtm.Pipeline interface.
func (p *Pipeline) Send(msg interface{}) error {
	p.mutex.RLock()
	conn := p.connection
	closed := p.closed
	p.mutex.RUnlock()

	if closed {
		return nil
	}

	if conn == nil {
		p.logger.Errorln("pipeline send without connection")
		return nil
	}

	return conn.Send(msg)
}

// Close implements the rtm.Pipeline interface.
func (p *Pipeline) Close() error {
	p.mutex.Lock()
	conn := p.connection
	p.connection = nil
	p.closed = true
	if p.reconnector != nil {
		p.reconnector.Stop()
		p.reconnector = nil
	}
	p.mutex.Unlock()

	if conn != nil {
		return conn.Close()
	}
	return nil
}

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

	"stash.kopano.io/kwm/kwmserver/signaling/connection"
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
	onResetHandler   func(error) error

	connecting  bool
	closed      bool
	reconnector *time.Timer
	watcher     *time.Timer
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
func (p *Pipeline) Connect(onConnect func() error, onText func([]byte) error, onReset func(err error) error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return errors.New("pipeline is closed")
	}

	p.onConnectHandler = onConnect
	p.onTextHandler = onText
	p.onResetHandler = onReset

	// Ask manager to connect.
	_, err := p.m.Attach(p.plugin, p.handle, p.onConnect, p.onText)
	if err != nil {
		p.logger.WithError(err).Warnln("failed to establish pipeline control")
		go p.reconnect()
	} else {
		p.logger.Debugln("pipeline control established")
	}
	go p.watch()

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

	p.connection = nil

	go p.reconnect()
	p.mutex.Unlock()
}

func (p *Pipeline) watch() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.watcher != nil {
		p.watcher.Stop()
	}

	p.watcher = time.AfterFunc(10*time.Second, func() { // TODO(longsleep): Make timeout configuration.
		p.mutex.Lock()
		defer p.mutex.Unlock()

		p.watcher = nil

		if p.closed {
			return
		}

		// Ensure to reconnect when no connection.
		if p.connection == nil && !p.connecting {
			p.logger.Warnln("pipeline connection watcher timeout")
			defer func() {
				go p.reconnect()
			}()
		}

		defer func() {
			go p.watch()
		}()
	})
}

func (p *Pipeline) reconnect() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.connecting {
		return
	}

	p.logger.Infoln("pipeline scheduling control reestablish")

	if p.reconnector != nil {
		p.reconnector.Stop()
	}

	p.connecting = true
	p.reconnector = time.AfterFunc(3*time.Second, func() {
		p.mutex.Lock()
		defer p.mutex.Unlock()

		p.reconnector = nil

		if !p.connecting || p.closed {
			return
		}

		// Ask manager to connect.
		_, err := p.m.Attach(p.plugin, p.handle, p.onConnect, p.onText)
		// TODO(longsleep): Implement background retry on error or timeout.
		if err != nil {
			p.logger.WithError(err).Warnln("pipeline failed to reestablish control")
			p.connecting = false
			defer func() {
				go p.reconnect()
			}()
		} else {
			p.logger.Infoln("pipeline control reestablished")
			p.connecting = false
			if p.watcher != nil {
				// Restart watcher, to restart timeout.
				p.watcher.Stop()
				defer func() {
					go p.watch()
				}()
			}
			if p.onResetHandler != nil {
				err = p.onResetHandler(nil)
				if err != nil {
					p.logger.WithError(err).Warnln("pipeline reset handler failed")
				}
			}
		}
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
	if p.watcher != nil {
		p.watcher.Stop()
		p.watcher = nil
	}
	p.mutex.Unlock()

	if conn != nil {
		err := p.m.Detach(p.plugin, p.handle)
		if err != nil {
			p.logger.WithError(err).Warnln("pipeline failed to detach")
		}

		return conn.Close()
	}
	return nil
}

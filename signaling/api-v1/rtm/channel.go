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
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	"stash.kopano.io/kc/konnect/rndm"
)

// Channel binds connections together.
type Channel struct {
	sync.RWMutex
	logger logrus.FieldLogger

	id string

	connections map[string]*Connection
}

// NewChannel initializes a new channel with id.
func NewChannel(id string, logger logrus.FieldLogger) *Channel {
	logger.Debugln("new channel")
	return &Channel{
		id:     id,
		logger: logger,

		connections: make(map[string]*Connection),
	}
}

// CreateChannel creates a new channel with random id.
func CreateChannel(m *Manager) (*Channel, error) {
	id, err := rndm.GenerateRandomString(channelIDSize)
	if err != nil {
		return nil, err
	}

	return NewChannel(id, m.logger.WithField("channel", id)), nil
}

// Add adds the provided connection to the channel identified by id.
func (c *Channel) Add(id string, connection *Connection) error {
	c.Lock()
	if _, ok := c.connections[id]; ok {
		c.Unlock()
		return errors.New("id already exists")
	}

	c.connections[id] = connection
	c.Unlock()

	connection.OnClosed(func(connection *Connection) {
		c.Remove(id)
	})

	c.logger.WithField("id", id).Debugln("channel add")
	return nil
}

// Remove removes the connection identified by the provided id.
func (c *Channel) Remove(id string) error {
	c.Lock()
	delete(c.connections, id)
	c.Unlock()

	c.logger.WithField("id", id).Debugln("channel remove")
	return nil
}

// Get retrieves the connection identified by the provided id.
func (c *Channel) Get(id string) (*Connection, bool) {
	c.RLock()
	connection, ok := c.connections[id]
	c.RUnlock()

	return connection, ok
}

// Size returns the number of connections in this channel.
func (c *Channel) Size() int {
	c.RLock()
	size := len(c.connections)
	c.RUnlock()

	return size
}

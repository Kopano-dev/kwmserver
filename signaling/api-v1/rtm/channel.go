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
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"stash.kopano.io/kgol/rndm"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// Channel ID prefixes
const (
	ChannelPrefixStandard   = "*"
	ChannelPrefixNamedGroup = "@"
)

// A ChannelOp defines channel operations.
type ChannelOp int

// Channel operations
const (
	ChannelOpAdd ChannelOp = iota
	ChannelOpRemove
)

// Channel binds connections together.
type Channel struct {
	sync.RWMutex
	logger logrus.FieldLogger

	id     string
	config *ChannelConfig

	connections map[string]*connection.Connection
}

// ChannelConfig adds extra configuration for a Channel.
type ChannelConfig struct {
	Group string

	AfterAddOrRemove func(channel *Channel, op ChannelOp, cid string)
}

// ChannelDefaultConfig holds a Channel's default extra configuration.
var ChannelDefaultConfig = &ChannelConfig{}

// NewChannel initializes a new channel with id.
func NewChannel(id string, logger logrus.FieldLogger, config *ChannelConfig) *Channel {
	logger.WithField("channel", id).Debugln("channel create")
	if config == nil {
		config = ChannelDefaultConfig
	}

	return &Channel{
		id:     id,
		logger: logger,
		config: config,

		connections: make(map[string]*connection.Connection),
	}
}

// CreateRandomChannel creates a new channel with random id.
func CreateRandomChannel(m *Manager, config *ChannelConfig) (*Channel, error) {
	id := fmt.Sprintf("%s%s", ChannelPrefixStandard, rndm.GenerateRandomString(channelIDSize))

	return NewChannel(id, m.logger.WithField("channel", id), config), nil
}

// CreateKnownChannel creates a new channel with known id.
func CreateKnownChannel(id string, m *Manager, config *ChannelConfig) (*Channel, error) {
	return NewChannel(id, m.logger.WithField("channel", id), config), nil
}

// Add adds the provided connection to the channel identified by id.
func (c *Channel) Add(id string, conn *connection.Connection) error {
	c.Lock()
	if existingConn, ok := c.connections[id]; ok {
		c.Unlock()
		if conn == existingConn {
			return nil
		}
		return errors.New("id already exists")
	}

	c.connections[id] = conn
	c.Unlock()

	conn.OnClosed(func(connection *connection.Connection) {
		c.Remove(id)
	})

	c.logger.WithField("id", id).Debugln("channel add")
	if c.config.AfterAddOrRemove != nil {
		go c.config.AfterAddOrRemove(c, ChannelOpAdd, id)
	}
	return nil
}

// Remove removes the connection identified by the provided id.
func (c *Channel) Remove(id string) error {
	c.Lock()
	delete(c.connections, id)
	c.Unlock()

	c.logger.WithField("id", id).Debugln("channel remove")
	if c.config.AfterAddOrRemove != nil {
		go c.config.AfterAddOrRemove(c, ChannelOpRemove, id)
	}
	return nil
}

// Get retrieves the connection identified by the provided id.
func (c *Channel) Get(id string) (*connection.Connection, bool) {
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

// CanBeCleanedUp up returns true if the channel can be cleaned up.
func (c *Channel) CanBeCleanedUp() bool {
	size := c.Size()
	if c.config != nil {
		// Special channels clean up when no connections remain.
		return size <= 0
	}

	// Normal channels can be cleaned up when only a single connection is set.
	return size <= 1
}

// Connections returns a array the currenct connection ids and an array of the
// current connections of this channel.
func (c *Channel) Connections() ([]string, []*connection.Connection) {
	c.RLock()
	ids := make([]string, len(c.connections))
	connections := make([]*connection.Connection, len(c.connections))
	idx := 0
	for id, connection := range c.connections {
		ids[idx] = id
		connections[idx] = connection
		idx++
	}
	c.RUnlock()

	return ids, connections
}

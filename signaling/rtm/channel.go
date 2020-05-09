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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"stash.kopano.io/kgol/rndm"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
	"stash.kopano.io/kwm/kwmserver/signaling/mcu"
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
	ChannelOpReset
)

// Channel binds connections together.
type Channel struct {
	sync.RWMutex
	logger logrus.FieldLogger

	closed bool

	id     string
	m      *Manager
	config *ChannelConfig

	connections map[string]*connection.Connection

	pipeline Pipeline
}

// ChannelConfig adds extra configuration for a Channel.
type ChannelConfig struct {
	Group string

	Replace          func(channel *Channel, cid string, oldConn *connection.Connection, newConn *connection.Connection)
	AfterAddOrRemove func(channel *Channel, op ChannelOp, cid string)
	AfterReset       func(channel *Channel)
}

// ChannelDefaultConfig holds a Channel's default extra configuration.
var ChannelDefaultConfig = &ChannelConfig{}

// NewChannel initializes a new channel with id.
func NewChannel(id string, m *Manager, logger logrus.FieldLogger, config *ChannelConfig) *Channel {
	if config == nil {
		config = ChannelDefaultConfig
	}

	channel := &Channel{
		id:     id,
		m:      m,
		logger: logger,
		config: config,

		connections: make(map[string]*connection.Connection),
	}
	channel.logger.Debugln("channel create")
	channelNew.WithLabelValues(m.id).Inc()

	pipeline := m.Pipeline(mcu.PluginIDKWMRTMChannel, id)
	if pipeline != nil {
		channel.pipeline = pipeline

		go pipeline.Connect(func() error {
			logger.Debugln("channel pipeline connect")

			var extra json.RawMessage
			extra, err := json.MarshalIndent(&api.RTMDataWebRTCChannelExtra{
				Pipeline: &api.RTMDataWebRTCChannelPipeline{
					Pipeline: pipeline.ID(),
					Mode:     pipeline.Mode(),
				},
			}, "", "\t")
			if err != nil {
				return fmt.Errorf("failed to encode channel extra data for pipeline: %v", err)
			}

			// Send channel to mcu to populate.
			err = pipeline.Send(&api.RTMTypeWebRTCReply{
				RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
					Type:    api.RTMTypeNameWebRTC,
					Subtype: api.RTMSubtypeNameWebRTCChannel,
				},
				Channel: id,
				Version: currentWebRTCPayloadVersion,
				Data:    extra,
			})
			if err != nil {
				return fmt.Errorf("failed to send channel data to pipeline: %v", err)
			}

			logger.Debugln("channel pipeline registered")
			return nil
		}, func(data []byte) error {
			var msg api.RTMTypeWebRTC
			err := json.Unmarshal(data, &msg)
			if err != nil {
				return fmt.Errorf("failed to decode JSON data from pipeline: %v", err)
			}

			err = channel.deliver(&msg)
			if err != nil {
				logger.WithError(err).Warnln("channel deliver from pipeline failed")
			}

			return nil
		}, func(err error) error {
			if err != nil {
				logger.WithError(err).Warnln("channel pipeline reset due to error")
			} else {
				logger.Debugln("channel pipeline reset")
			}
			// Reset channel if handler set.
			if channel.config.AfterReset != nil {
				go channel.config.AfterReset(channel)
			}
			return nil
		})
	}

	return channel
}

// CreateRandomChannel creates a new channel with random id.
func CreateRandomChannel(m *Manager, config *ChannelConfig) *Channel {
	id := fmt.Sprintf("%s%s", ChannelPrefixStandard, rndm.GenerateRandomString(channelIDSize))

	return NewChannel(id, m, m.logger.WithField("channel", id), config)
}

// CreateKnownChannel creates a new channel with known id.
func CreateKnownChannel(id string, m *Manager, config *ChannelConfig) *Channel {
	return NewChannel(id, m, m.logger.WithField("channel", id), config)
}

// Add adds the provided connection to the channel identified by id.
func (c *Channel) Add(id string, conn *connection.Connection) error {
	c.Lock()
	if c.closed {
		c.Unlock()
		return errors.New("channel is closed")
	}

	existingConn, replacing := c.connections[id]
	if replacing {
		if conn == existingConn {
			// Nothing to do, added conn is already added.
			c.Unlock()
			return nil
		}
		if c.config.Replace == nil {
			// No replace support registered.
			return errors.New("id already exists")
		}
		// Replace.
		c.logger.WithFields(logrus.Fields{
			"id":      id,
			"channel": c.id,
		}).Debugln("channel replace")
		// Trigger replace handler synchronously.
		c.config.Replace(c, id, existingConn, conn)
	}

	c.connections[id] = conn
	c.Unlock()

	conn.OnClosed(func(connection *connection.Connection) {
		c.Lock()
		c.remove(id, conn) // Remove exact matches only.
		c.Unlock()
	})

	c.logger.WithFields(logrus.Fields{
		"id":      id,
		"channel": c.id,
	}).Debugln("channel add")
	channelAdd.WithLabelValues(c.m.id).Inc()

	if c.config.AfterAddOrRemove != nil {
		go c.config.AfterAddOrRemove(c, ChannelOpAdd, id)
	}
	return nil
}

// Remove removes the connection identified by the provided id.
func (c *Channel) Remove(id string) error {
	c.Lock()
	defer c.Unlock()
	return c.remove(id, nil)
}

func (c *Channel) remove(id string, conn *connection.Connection) error {
	if conn != nil {
		// Validate that conn is the one we have.
		existingConn, _ := c.connections[id]
		if existingConn != conn {
			return errors.New("conn does not match")
		}
	}
	delete(c.connections, id)
	c.logger.WithFields(logrus.Fields{
		"id":      id,
		"channel": c.id,
	}).Debugln("channel remove")
	channelRemove.WithLabelValues(c.m.id).Inc()

	if c.config.AfterAddOrRemove != nil {
		go c.config.AfterAddOrRemove(c, ChannelOpRemove, id)
	}
	return nil
}

// Get retrieves the connection identified by the provided id.
func (c *Channel) Get(id string) (*connection.Connection, bool) {
	c.RLock()
	conn, ok := c.connections[id]
	c.RUnlock()

	// NOTE(longsleep): Return direclty accociated connection. Always returns
	// ok when the accociated channel has a pipeline.
	return conn, ok || c.pipeline != nil
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
	c.RLock()
	result := c.canBeCleanedUp()
	c.RUnlock()

	return result
}

func (c *Channel) canBeCleanedUp() bool {
	size := len(c.connections)
	if c.config != nil {
		// Special channels clean up when no connections remain.
		return size <= 0
	}

	// Normal channels can be cleaned up when only a single connection is set.
	return size <= 1
}

// Cleanup closes the associated channels resources and marks the channel closed.
func (c *Channel) Cleanup() bool {
	c.Lock()
	if !c.canBeCleanedUp() {
		c.logger.Debugln("channel cleanup rejected")
		c.Unlock()
		return false
	}

	pipeline := c.pipeline
	c.closed = true
	c.pipeline = nil
	size := len(c.connections)
	c.Unlock()

	if pipeline != nil {
		go pipeline.Close()
	}

	c.logger.WithField("connections", size).Debugln("channel cleaned up")
	channelCleanup.WithLabelValues(c.m.id).Inc()

	return true
}

// Connections returns a array the currenct connection ids and an array of the
// current connections of this channel.
func (c *Channel) Connections() ([]string, []*connection.Connection) {
	c.RLock()
	ids := make([]string, len(c.connections))
	connections := make([]*connection.Connection, len(c.connections))
	idx := 0
	for id, conn := range c.connections {
		ids[idx] = id
		connections[idx] = conn
		idx++
	}
	c.RUnlock()

	return ids, connections
}

// Pipeline returns the attached Channel's Pipeline.
func (c *Channel) Pipeline() Pipeline {
	pipeline := c.pipeline

	return pipeline
}

// Forward takes care of sending the provided message to the channels assigned
// target.
func (c *Channel) Forward(source string, target string, conn *connection.Connection, msg *api.RTMTypeWebRTC) error {
	msg.Source = source

	// Send through pipeline if any.
	if c.pipeline != nil {
		return c.pipeline.Send(msg)
	}

	if conn == nil {
		conn, _ = c.Get(target)
	}
	if conn == nil {
		return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
	}

	msg.ID = 0
	return conn.Send(msg)
}

func (c *Channel) deliver(msg *api.RTMTypeWebRTC) error {
	if msg.Source == "" || msg.Target == "" {
		return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "invalid target", msg.ID)
	}

	conn, _ := c.Get(msg.Target)
	if conn == nil {
		return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
	}

	return conn.Send(msg)
}

func (c *Channel) checkWebRTCMessage(source string, msg *api.RTMTypeWebRTC) error {
	// check channel with group.
	if msg.Group != "" {
		if c.config == nil || c.config.Group != msg.Group {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid channel for group", msg.ID)
		}
	}

	// check hash
	if msg.Hash == "" {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "missing hash", msg.ID)
	}
	hash, err := base64.StdEncoding.DecodeString(msg.Hash)
	if err != nil {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "hash decode error", msg.ID)
	}

	// Target is by default the msg target.
	target := msg.Target
	switch {
	case msg.Group != "":
		// Group messages use group hash.
		target = msg.Group
	case target == "":
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "missing target", msg.ID)
	case c.pipeline != nil:
		// TODO(longsleep): Add hash validation when running through pipeline.
		// Validation is left to the pipeline implementation.
		return nil
	}

	// Validate hash with target and channel.
	if checkWebRTCChannelHash(hash, msg.Type, source, target, msg.Channel) {
		return nil
	}

	// Either always also allow the directly targeted messages.
	if checkWebRTCChannelHash(hash, msg.Type, source, msg.Target, msg.Channel) {
		return nil
	}

	return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid hash", msg.ID)
}

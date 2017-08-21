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
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

// Plugin is thje interface for janus plugin implementations.
type Plugin interface {
	OnMessage(m *Manager, c *connection.Connection, msg *MessageMessageData) error
	OnDetach(m *Manager, c *connection.Connection, msg *EnvelopeData) error
	Name() string
	HandleID() int64
	Attach(m *Manager, c *connection.Connection, msg *AttachMessageData, cb func(Plugin), cleanup func(Plugin)) error
	Enabled() bool
}

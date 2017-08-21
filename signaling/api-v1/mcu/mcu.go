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

package mcu

import (
	"time"
)

const (
	connectExpiration      = time.Duration(30) * time.Second
	connectCleanupInterval = time.Duration(1) * time.Minute

	// Buffer sizes.
	websocketReadBufferSize  = 1024
	websocketWriteBufferSize = 1024

	websocketSubProtocolName = "kwmmcu-protocol"
)

type transactionMessage struct {
	ID string `json:"transaction"`
}

func (m *transactionMessage) TransactionID() string {
	return m.ID
}

// WebsocketMessage is the container for basic mcu websocket messages.
type WebsocketMessage struct {
	Type   string `json:"type"`
	ID     string `json:"transaction"`
	Plugin string `json:"plugin"`
	Handle int64  `json:"handle_id"`
}

// TransactionID returns the transaction ID of the accociated message.
func (m *WebsocketMessage) TransactionID() string {
	return m.ID
}

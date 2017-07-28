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
	"encoding/json"
	"time"
)

const (
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

	websocketSubProtocolName = "janus-protocol"
)

var rawZeroBytes []byte

// Public constants.
const (
	TypeNameSuccess   = "success"
	TypeNameCreate    = "create"
	TypeNameAttach    = "attach"
	TypeNameDetach    = "detach"
	TypeNameEvent     = "event"
	TypeNameMessage   = "message"
	TypeNameKeepAlive = "keepalive"
	TypeNameDestroy   = "destroy"
)

// Response is a JSON response with status.
type Response struct {
	Type       string      `json:"janus"`
	ID         string      `json:"transaction,omitempty"`
	Sender     int64       `json:"sender"`
	Data       interface{} `json:"data,omitempty"`
	PluginData *PluginData `json:"plugindata,omitempty"`
	JSEP       interface{} `json:"jsep,omitempty"`
}

// PluginData is a JSON plugin respoinse data.
type PluginData struct {
	PluginName string      `json:"plugin"`
	Data       interface{} `json:"data"`
}

type janusEnvelope struct {
	Type    string           `json:"janus"`
	ID      string           `json:"transaction"`
	Token   string           `json:"token"`
	Session int64            `json:"session_id"`
	Handle  int64            `json:"handle_id"`
	JSEP    *json.RawMessage `json:"jsep"`
}

type janusCreateMessage struct {
	*janusEnvelope
}

type janusAttachMessage struct {
	*janusEnvelope
	PluginName string `json:"plugin"`
}

type janusMessageMessage struct {
	*janusEnvelope
	Body *json.RawMessage `json:"body"`
}

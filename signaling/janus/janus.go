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
	tokenCleanupInterval = time.Duration(30) * time.Second
	tokenExpiration      = time.Duration(1) * time.Minute

	// Buffer sizes.
	websocketReadBufferSize  = 1024
	websocketWriteBufferSize = 1024

	websocketSubProtocolName = "janus-protocol"
)

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
	TypeNameTrickle   = "trickle"

	TypeNameAdminAddToken = "add_token"
)

// ResponseData is a JSON response with status.
type ResponseData struct {
	Type       string      `json:"janus"`
	ID         string      `json:"transaction,omitempty"`
	Sender     int64       `json:"sender"`
	Data       interface{} `json:"data,omitempty"`
	PluginData *PluginData `json:"plugindata,omitempty"`
	JSEP       interface{} `json:"jsep,omitempty"`
}

// TransactionID is a getter for the transaction ID.
func (r *ResponseData) TransactionID() string {
	return r.ID
}

// PluginData is a JSON plugin respoinse data.
type PluginData struct {
	PluginName string      `json:"plugin"`
	Data       interface{} `json:"data"`
}

// EnvelopeData is the base Janus JSON payload container.
type EnvelopeData struct {
	Type    string           `json:"janus"`
	ID      string           `json:"transaction"`
	Token   string           `json:"token"`
	Session int64            `json:"session_id"`
	Handle  int64            `json:"handle_id"`
	JSEP    *json.RawMessage `json:"jsep,omitempty"`

	// Extra data, MCU backend specific.
	TargetSession int64 `json:"target_session_id,omitempty"`
}

// TransactionID is a getter for the transaction ID.
func (je *EnvelopeData) TransactionID() string {
	return je.ID
}

// CreateMessageData is the Janus JSON payload for create messages.
type CreateMessageData struct {
	*EnvelopeData
}

// AttachMessageData is the Janus JSON payload for attach messages.
type AttachMessageData struct {
	*EnvelopeData
	PluginName string `json:"plugin"`
}

// MessageMessageData is the Janus JSON payload for message messages.
type MessageMessageData struct {
	*EnvelopeData
	Body *json.RawMessage `json:"body"`
}

// TrickleMessageData is the Janus JSON payload for trickle messages.
type TrickleMessageData struct {
	*EnvelopeData
	Candidate *json.RawMessage `json:"candidate"`
}

// AdminAddTokenData is the Janus JSON payload for admin add_token messages.
type AdminAddTokenData struct {
	*EnvelopeData
	AdminSecret string `json:"admin_secret"`
	Token       string `json:"token"`
}

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

package api

import (
	"encoding/json"
)

// Type names for RTM payloads.
const (
	RTMTypeNameHello   = "hello"
	RTMTypeNameError   = "error"
	RTMTypeNamePing    = "ping"
	RTMTypeNamePong    = "pong"
	RTMTypeNameGoodbye = "goodbye"

	RTMTypeNameWebRTC = "webrtc"

	RTMSubtypeNameWebRTCCall   = "webrtc_call"
	RTMSubtypeNameWebRTCSignal = "webrtc_signal"

	RTMErrorIDServerError      = "server_error"
	RTMErrorIDBadMessage       = "bad_message"
	RTMErrorIDNoSessionForUser = "no_session_for_user"
)

// RTMConnectResponse is the response returned from rtm.connect.
type RTMConnectResponse struct {
	ResponseOK

	URL  string `json:"url"`
	Self *Self  `json:"self"`
}

// RTMTypeEnvelope is the envelope with type key for RTM JSON data messages.
type RTMTypeEnvelope struct {
	ID   uint64 `json:"id,omitempty"`
	Type string `json:"type"`
}

// RTMTypeSubtypeEnvelope is the envelope with type and subtype key for RTM JSON
// data messages.
type RTMTypeSubtypeEnvelope struct {
	ID      uint64 `json:"id,omitempty"`
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
}

// RTMTypeEnvelopeReply is the envelope with type key and reply for RTM JSON
// data reply messages.
type RTMTypeEnvelopeReply struct {
	Type    string `json:"type"`
	ReplyTo uint64 `json:"reply_to,omitempty"`
}

// RTMTypeHelloMessage is the hello message sent to clients after connect.
var RTMTypeHelloMessage = &RTMTypeEnvelope{
	Type: RTMTypeNameHello,
}

// RTMTypeGoodbyeMessage is the goodbye message sent to clients before the
// server is going to disconnect.
var RTMTypeGoodbyeMessage = &RTMTypeEnvelope{
	Type: RTMTypeNameGoodbye,
}

// RTMTypeError is the error reply.
type RTMTypeError struct {
	*RTMTypeEnvelopeReply
	ErrorData *RTMDataError `json:"error"`
}

// Error implements the error interface.
func (err *RTMTypeError) Error() string {
	if (err.ErrorData) == nil {
		return RTMErrorIDServerError
	}
	return err.ErrorData.Code
}

// RTMDataError is the error payload data.
type RTMDataError struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// NewRTMTypeError creates a new RTMTypeError with the provided parameters.
func NewRTMTypeError(code string, msg string, replyTo uint64) *RTMTypeError {
	return &RTMTypeError{
		&RTMTypeEnvelopeReply{
			Type:    RTMTypeNameError,
			ReplyTo: replyTo,
		},
		&RTMDataError{
			Code: code,
			Msg:  msg,
		},
	}
}

// RTMTypePingPong is the ping/pong message.
type RTMTypePingPong map[string]interface{}

// RTMTypeWebRTC defines webrtc related messages.
type RTMTypeWebRTC struct {
	*RTMTypeSubtypeEnvelope
	Target    string          `json:"target"`
	Source    string          `json:"source"`
	Initiator bool            `json:"initiator,omitempty"`
	State     string          `json:"state,omitempty"`
	Channel   string          `json:"channel,omitempty"`
	Hash      string          `json:"hash,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

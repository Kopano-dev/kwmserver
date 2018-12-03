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

	"stash.kopano.io/kwm/kwmserver/turn"
)

// Type names for RTM payloads.
const (
	RTMTypeNameHello   = "hello"
	RTMTypeNameError   = "error"
	RTMTypeNamePing    = "ping"
	RTMTypeNamePong    = "pong"
	RTMTypeNameGoodbye = "goodbye"

	RTMTypeNameWebRTC = "webrtc"

	RTMSubtypeNameWebRTCCall    = "webrtc_call"
	RTMSubtypeNameWebRTCChannel = "webrtc_channel"
	RTMSubtypeNameWebRTCHangup  = "webrtc_hangup"
	RTMSubtypeNameWebRTCSignal  = "webrtc_signal"

	RTMSubtypeNameWebRTCGroup = "webrtc_group"

	RTMErrorIDServerError      = "server_error"
	RTMErrorIDBadMessage       = "bad_message"
	RTMErrorIDNoSessionForUser = "no_session_for_user"
)

// RTMConnectResponse is the response returned from rtm.connect.
type RTMConnectResponse struct {
	ResponseOK

	URL  string `json:"url"`
	Self *Self  `json:"self"`

	TURN *turn.ClientConfig `json:"turn,omitempty"`
}

// RTMTURNResponse is the response returned from rtm.turn.
type RTMTURNResponse struct {
	ResponseOK

	TURN *turn.ClientConfig `json:"turn"`
}

// RTMTypeTransaction is the envelope with type key and optional transaction for
// RTM JSON data messages.
type RTMTypeTransaction struct {
	Type        string `json:"type"`
	Transaction string `json:"transaction,omitempty"`
}

// TransactionID returns the transaction ID of the accociated message.
func (e *RTMTypeTransaction) TransactionID() string {
	return e.Transaction
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

// RTMTypeSubtypeEnvelopeReply is the envelope with type and subtype and reply
// for RTM JSON data reply messages
type RTMTypeSubtypeEnvelopeReply struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	ReplyTo uint64 `json:"reply_to,omitempty"`
}

// RTMTypeHello is the message sent to clients after connect and before the
// server is going to disconnect.
type RTMTypeHello struct {
	Type string `json:"type"`
	Self *Self  `json:"self,omitempty"`
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
	Target      string          `json:"target"`
	Source      string          `json:"source"`
	Initiator   bool            `json:"initiator,omitempty"`
	State       string          `json:"state,omitempty"`
	Channel     string          `json:"channel,omitempty"`
	Group       string          `json:"group,omitempty"`
	Hash        string          `json:"hash,omitempty"`
	Version     uint64          `json:"v"`
	Transaction string          `json:"transaction,omitempty"`
	Pcid        string          `json:"pcid,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// TransactionID returns the transaction ID of the accociated message.
func (e *RTMTypeWebRTC) TransactionID() string {
	return e.Transaction
}

// RTMTypeWebRTCReply defines webrtc related replies..
type RTMTypeWebRTCReply struct {
	*RTMTypeSubtypeEnvelopeReply
	State   string          `json:"state,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Hash    string          `json:"hash,omitempty"`
	Version uint64          `json:"v"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// RTMDataWebRTCAccept defines webrtc extra accept data.
type RTMDataWebRTCAccept struct {
	Accept bool   `json:"accept"`
	State  string `json:"state,omitempty"`
	Reason string `json:"reason"`
}

// RTMDataWebRTCChannelExtra defines webrtc channel extra data.
type RTMDataWebRTCChannelExtra struct {
	Group    *RTMTDataWebRTCChannelGroup   `json:"group,omitempty"`
	Pipeline *RTMDataWebRTCChannelPipeline `json:"pipeline,omitempty"`
	Replaced bool                          `json:"replaced,omitempty"`
}

// RTMTDataWebRTCChannelGroup defnes webrtc channel group details.
type RTMTDataWebRTCChannelGroup struct {
	Group   string   `json:"group"`
	Members []string `json:"members"`
}

// RTMDataWebRTCChannelPipeline defines webrtc channel pipeline details.
type RTMDataWebRTCChannelPipeline struct {
	Pipeline string `json:"pipeline"`
	Mode     string `json:"mode"`
}

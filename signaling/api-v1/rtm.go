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

// Type names for RTM payloads.
const (
	RTMTypeNameHello = "hello"
	RTMTypeNameError = "error"
	RTMTypeNamePing  = "ping"
	RTMTypeNamePong  = "pong"
)

// RTMConnectResponse is the response returned from rtm.connect.
type RTMConnectResponse struct {
	ResponseOK

	URL  string `json:"url"`
	Self *Self  `json:"self"`
}

// RTMTypeEnvelope is the envelope with type key for RTM JSON data messages.
type RTMTypeEnvelope struct {
	Type string `json:"type"`
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

// RTMTypeError is the error reply.
type RTMTypeError struct {
	*RTMTypeEnvelopeReply
	Error *RTMDataError `json:"error"`
}

// RTMDataError is the error payload data.
type RTMDataError struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
}

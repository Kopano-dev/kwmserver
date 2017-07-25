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
	"encoding/json"
	"time"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

const (
	connectExpiration      = time.Duration(30) * time.Second
	connectCleanupInterval = time.Duration(1) * time.Minute
	connectKeySize         = 24
	connectionIDSize       = 24
	channelIDSize          = 24
	channelExpiration      = time.Duration(1) * time.Minute

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
)

var (
	rawZeroBytes             []byte
	rawRTMTypeHelloMessage   []byte
	rawRTMTypeGoodbyeMessage []byte
)

func init() {
	helloMessage, err := json.MarshalIndent(api.RTMTypeHelloMessage, "", "\t")
	if err != nil {
		panic(err)
	}
	rawRTMTypeHelloMessage = helloMessage

	goodbyeMessage, err := json.MarshalIndent(api.RTMTypeGoodbyeMessage, "", "\t")
	if err != nil {
		panic(err)
	}
	rawRTMTypeGoodbyeMessage = goodbyeMessage
}

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
	"fmt"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"

	"stash.kopano.io/kc/konnect/rndm"
)

func (m *Manager) onWebRTC(c *Connection, msg *api.RTMTypeWebRTC) error {
	switch msg.Subtype {
	case api.RTMSubtypeNameWebRTCCall:
		// Connection must have a user.
		if c.user == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "connection has no user", msg.ID)
		}
		// Target must always be not empty.
		if msg.Target == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target is empty", msg.ID)
		}
		// Target cannot be the same as source.
		if msg.Target == c.user.id {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target same as source", msg.ID)
		}
		// State must always be not empty.
		if msg.State == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "state is empty", msg.ID)
		}
		// Source must always be empty when received here.
		if msg.Source != "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "source must be empty", msg.ID)
		}
		// Check if this is a request or response.
		// Ff initiator is true, it must be a request, thus channel, hash
		// and source must be empty.
		if msg.Initiator {
			// Must be a request.
			if msg.Channel != "" || msg.Hash != "" {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel and hash must be empty", msg.ID)
			}
			if msg.Data != nil {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "data must be empty", msg.ID)
			}
			// Add source, channel and hash.
			msg.Source = c.user.id
			channel, err := rndm.GenerateRandomString(channelIDSize)
			if err != nil {
				return fmt.Errorf("failed to generate channel id: %v", err)
			}
			msg.Channel = channel
			msg.Hash = "lala"
			msg.ID = 0

			// TODO(longsleep): make hash
			// Lookup target and send modified message.
			connections, ok := m.LookupConnectionsByUserID(msg.Target)
			if !ok {
				return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
			}
			for _, c := range connections {
				c.Send(msg)
			}

		} else {
			// Must be a response.
			if msg.Channel == "" || msg.Hash == "" || msg.Data == nil {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel, hash or data is empty", msg.ID)
			}
			// check hash
			if msg.Hash != "lala" {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid hash", msg.ID)
			}

			// Add source and send modified message.
			msg.Source = c.user.id
			msg.ID = 0

			// Lookup target and send modified message.
			connections, ok := m.LookupConnectionsByUserID(msg.Target)
			if !ok {
				return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
			}
			for _, c := range connections {
				c.Send(msg)
			}
		}

	case api.RTMSubtypeNameWebRTCOffer:
		fallthrough
	case api.RTMSubtypeNameWebRTCAnswer:
		fallthrough
	case api.RTMSubtypeNameWebRTCCandidate:
		// Connection must have a user.
		if c.user == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "connection has no user", msg.ID)
		}
		// Target must always be not empty.
		if msg.Target == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target is empty", msg.ID)
		}
		// Target cannot be the same as source.
		if msg.Target == c.user.id {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target same as source", msg.ID)
		}
		// State must always be not empty.
		if msg.State == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "state is empty", msg.ID)
		}
		// Source must always be empty when received here.
		if msg.Source != "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "source must be empty", msg.ID)
		}
		if msg.Channel == "" || msg.Hash == "" || msg.Data == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel hash or data is empty", msg.ID)
		}

		// check hash
		if msg.Hash != "lala" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid hash", msg.ID)
		}

		// Add source and send modified message.
		msg.Source = c.user.id
		msg.ID = 0

		// Lookup and send modified message.
		c.Send(msg)

	default:
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "unknown subtype", msg.ID)
	}

	return nil
}

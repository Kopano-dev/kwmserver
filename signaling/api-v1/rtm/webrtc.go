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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"stash.kopano.io/kc/konnect/rndm"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

var webrtcChannelHashKey []byte

func init() {
	var err error
	webrtcChannelHashKey, err = rndm.GenerateRandomBytes(32)
	if err != nil {
		panic(err)
	}
}

func computeWebRTCChannelHash(msgType, source, target, channel string) []byte {
	h := hmac.New(sha256.New, webrtcChannelHashKey)
	h.Write([]byte(msgType))
	if source < target {
		h.Write([]byte(source))
		h.Write([]byte(target))
	} else {
		h.Write([]byte(target))
		h.Write([]byte(source))
	}
	h.Write([]byte(channel))
	return h.Sum(nil)
}

func checkWebRTCChannelHash(source string, msg *api.RTMTypeWebRTC) error {
	// check hash
	if msg.Hash == "" {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "missing hash", msg.ID)
	}
	hash, err := base64.StdEncoding.DecodeString(msg.Hash)
	if err != nil {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "hash decode error", msg.ID)
	}
	if !hmac.Equal(hash, computeWebRTCChannelHash(msg.Type, source, msg.Target, msg.Channel)) {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid hash", msg.ID)
	}

	return nil
}

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

			// Create channel annd add user with connection.
			channel, err := CreateChannel(m)
			if err != nil {
				return fmt.Errorf("failed to create channel: %v", err)
			}
			channel.Add(c.user.id, c)
			record := &channelRecord{
				when:    time.Now(),
				channel: channel,
			}
			m.channels.SetIfAbsent(channel.id, record)

			// Create hash for channel.
			hash := computeWebRTCChannelHash(msg.Type, c.user.id, msg.Target, channel.id)

			// Add source, channel and hash.
			msg.Source = c.user.id
			msg.Channel = channel.id
			msg.Hash = base64.StdEncoding.EncodeToString(hash)

			// Send to self to populate channel and hash.
			c.Send(&api.RTMTypeWebRTCReply{
				RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
					Type:    api.RTMTypeNameWebRTC,
					Subtype: api.RTMSubtypeNameWebRTCChannel,
					ReplyTo: msg.ID,
				},
				Channel: msg.Channel,
				Hash:    msg.Hash,
			})

			// Reset id for sending to target.
			msg.ID = 0

			// Lookup target and send modified message.
			connections, ok := m.LookupConnectionsByUserID(msg.Target)
			if !ok {
				return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
			}
			for _, connection := range connections {
				connection.Send(msg)
			}

		} else {
			// Must be a response.
			if msg.Channel == "" || msg.Hash == "" || msg.Data == nil {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel, hash or data is empty", msg.ID)
			}

			// check hash
			err := checkWebRTCChannelHash(c.user.id, msg)
			if err != nil {
				return err
			}

			// check extra data
			var msgData *api.RTMDataWebRTCAccept
			err = json.Unmarshal(msg.Data, &msgData)
			if err != nil {
				return err
			}

			// Get channel and add user with connection.
			record, ok := m.channels.Get(msg.Channel)
			if !ok {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel not found", msg.ID)
			}
			channel := record.(*channelRecord).channel

			if msgData.Accept {
				// Add to channel when accept.
				err = channel.Add(c.user.id, c)
				if err != nil {
					return api.NewRTMTypeError(api.RTMErrorIDBadMessage, err.Error(), msg.ID)
				}
			}

			if c.user != nil {
				// Notify users other connetions, which might have received the call.
				connections, exists := m.LookupConnectionsByUserID(c.user.id)
				if exists {
					clearedMsg := &api.RTMTypeWebRTC{
						RTMTypeSubtypeEnvelope: &api.RTMTypeSubtypeEnvelope{
							Type:    api.RTMTypeNameWebRTC,
							Subtype: api.RTMSubtypeNameWebRTCCall,
						},
						Initiator: true,
						Channel:   msg.Channel,
					}
					for _, connection := range connections {
						if connection == c {
							continue
						}
						connection.Send(clearedMsg)
					}
				}
			}

			// Add source and send modified message.
			msg.Source = c.user.id
			msg.ID = 0

			// Lookup target and send modified message.
			connection, ok := channel.Get(msg.Target)
			if !ok {
				return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
			}
			connection.Send(msg)
		}

	case api.RTMSubtypeNameWebRTCHangup:
		fallthrough

	case api.RTMSubtypeNameWebRTCSignal:
		// Connection must have a user.
		if c.user == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "connection has no user", msg.ID)
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
		err := checkWebRTCChannelHash(c.user.id, msg)
		if err != nil {
			return err
		}

		// Get channel and add user with connection.
		record, ok := m.channels.Get(msg.Channel)
		if !ok {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel not found", msg.ID)
		}
		channel := record.(*channelRecord).channel

		// Add source and send modified message.
		msg.Source = c.user.id
		msg.ID = 0

		// Lookup target and send modified message.
		connection, ok := channel.Get(msg.Target)
		if msg.Subtype == api.RTMSubtypeNameWebRTCHangup {
			// XXX(longsleep): Find a better way to remove ourselves from channels.
			channel.Remove(c.user.id)
			if !ok {
				// Lookup target and send hangup to all user connections.
				connections, exists := m.LookupConnectionsByUserID(msg.Target)
				if !exists {
					return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
				}
				for _, connection := range connections {
					connection.Send(msg)
				}
				break
			}
		}
		if !ok {
			return api.NewRTMTypeError(api.RTMErrorIDNoSessionForUser, "target not found", msg.ID)
		}

		connection.Send(msg)

	default:
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "unknown subtype", msg.ID)
	}

	return nil
}

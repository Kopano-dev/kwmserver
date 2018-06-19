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

	"stash.kopano.io/kgol/rndm"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/connection"
)

var webrtcChannelHashKey []byte

func init() {
	webrtcChannelHashKey = rndm.GenerateRandomBytes(32)
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
	target := msg.Target
	if msg.Group != "" {
		// Validate group when set, instead of target.
		target = msg.Group
	} else if target == "" {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "missing target", msg.ID)
	}
	if !hmac.Equal(hash, computeWebRTCChannelHash(msg.Type, source, target, msg.Channel)) {
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid hash", msg.ID)
	}

	return nil
}

func (m *Manager) onWebRTC(c *connection.Connection, msg *api.RTMTypeWebRTC) error {
	switch msg.Subtype {
	case api.RTMSubtypeNameWebRTCGroup:
		// Group query or create.
		// Connection must have a user.
		bound := c.Bound()
		if bound == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "connection has no user", msg.ID)
		}
		ur := bound.(*userRecord)
		// Target must always be not empty.
		if msg.Target == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target is empty", msg.ID)
		}
		// TODO(longsleep): Target is the group's public ID - find a way to validate.
		if msg.Target != msg.Group {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target and group mismatch", msg.ID)
		}
		// State must always be not empty.
		if msg.State == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "state is empty", msg.ID)
		}
		// Source must always be empty when received here.
		if msg.Source != "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "source must be empty", msg.ID)
		}

		// Create consistent group channel ID.
		channelID, err := CreateNamedGroupChannelID(msg.Group, m)
		if err != nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, err.Error(), msg.ID)
		}
		// Get or create channel with ID.
		record := m.channels.Upsert(channelID, nil, func(exists bool, valueInMap interface{}, newValue interface{}) interface{} {
			if exists && valueInMap != nil {
				return valueInMap
			}
			newChannel, newChannelErr := CreateKnownChannel(channelID, m, &ChannelConfig{
				Group: msg.Group,

				AfterAddOrRemove: m.onAfterGroupAddOrRemove,
			})
			if newChannelErr != nil {
				m.logger.WithError(newChannelErr).WithField("channel", channelID).Errorln("failed to create known channel")
				return nil
			}
			return &channelRecord{
				when:    time.Now(),
				channel: newChannel,
			}
		})
		if record == nil {
			// We are fucked.
			err = fmt.Errorf("channel upsert without result")
			m.logger.WithError(err).WithField("channel", channelID).Errorln("failed to create channel for group")
			return err
		}

		channel := record.(*channelRecord).channel

		// Add user connection.
		err = channel.Add(ur.id, c)
		if err != nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, err.Error(), msg.ID)
		}

		// Create hash for channel.
		//m.logger.Debugln("webrtc_group hash", channel.id, ur.id, msg.Group)
		hash := computeWebRTCChannelHash(msg.Type, ur.id, msg.Group, channel.id)

		// Add source, channel and hash.
		msg.Source = ur.id
		msg.Channel = channel.id
		msg.Hash = base64.StdEncoding.EncodeToString(hash)

		// Get IDs of memmbers in channel.
		members, _ := channel.Connections()

		extra, err := json.MarshalIndent(&api.RTMDataWebRTCChannelExtra{
			RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
				Type:    api.RTMSubtypeNameWebRTCGroup,
				ReplyTo: msg.ID,
			},
			Group: &api.RTMTDataWebRTCChannelGroup{
				Group:   msg.Group,
				Members: members,
			},
		}, "", "\t")
		if err != nil {
			return fmt.Errorf("failed to encode group data: %v", err)
		}

		// Send to self to populate channel and hash.
		c.Send(&api.RTMTypeWebRTCReply{
			RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
				Type:    api.RTMTypeNameWebRTC,
				Subtype: api.RTMSubtypeNameWebRTCChannel,
				ReplyTo: msg.ID,
			},
			Channel: msg.Channel,
			Hash:    msg.Hash,
			Data:    extra,
		})

	case api.RTMSubtypeNameWebRTCCall:
		// Connection must have a user.
		bound := c.Bound()
		if bound == nil {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "connection has no user", msg.ID)
		}
		ur := bound.(*userRecord)
		// Target must always be not empty.
		if msg.Target == "" {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "target is empty", msg.ID)
		}
		// Target cannot be the same as source.
		if msg.Target == ur.id {
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
			channel, err := CreateRandomChannel(m, nil)
			if err != nil {
				return fmt.Errorf("failed to create channel: %v", err)
			}
			err = channel.Add(ur.id, c)
			if err != nil {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, err.Error(), msg.ID)
			}
			record := &channelRecord{
				when:    time.Now(),
				channel: channel,
			}
			m.channels.SetIfAbsent(channel.id, record)

			// Create hash for channel.
			hash := computeWebRTCChannelHash(msg.Type, ur.id, msg.Target, channel.id)

			// Add source, channel and hash.
			msg.Source = ur.id
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
			err := checkWebRTCChannelHash(ur.id, msg)
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

			if msg.Group != "" {
				if !msgData.Accept {
					return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "accept required for group call", msg.ID)
				}

				// Group accept.
				// NOTE(longsleep): Incoming group hash, replace with targets
				// group hash before sending out.
				hash := computeWebRTCChannelHash(msg.Type, ur.id, msg.Target, channel.id)
				msg.Hash = base64.StdEncoding.EncodeToString(hash)
				//m.logger.Debugln("group accept hash", ur.id, msg.Target, channel.id, msg.Hash)

			} else {
				// Normal call accept.
				if msgData.Accept {
					// Add to channel when accept.
					err = channel.Add(ur.id, c)
					if err != nil {
						return api.NewRTMTypeError(api.RTMErrorIDBadMessage, err.Error(), msg.ID)
					}
				}

				if ur != nil {
					// Notify users other connetions, which might have received the call.
					connections, exists := m.LookupConnectionsByUserID(ur.id)
					if exists {
						clearedMsg := &api.RTMTypeWebRTC{
							RTMTypeSubtypeEnvelope: &api.RTMTypeSubtypeEnvelope{
								Type:    api.RTMTypeNameWebRTC,
								Subtype: api.RTMSubtypeNameWebRTCCall,
							},
							Initiator: true,
							Channel:   msg.Channel,
							Source:    msg.Target,
						}
						for _, connection := range connections {
							if connection == c {
								continue
							}
							connection.Send(clearedMsg)
						}
					}
				}
			}

			// Add source and send modified message.
			msg.Source = ur.id
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
		bound := c.Bound()
		// Connection must have a user.
		if bound == nil {
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

		ur := bound.(*userRecord)

		// check hash
		if msg.Group == "" {
			err := checkWebRTCChannelHash(ur.id, msg)
			if err != nil {
				return err
			}
		} else {
			// TODO(longsleep): Hash check disabled for groups because group calls send group hash.
		}

		// Get channel and add user with connection.
		record, ok := m.channels.Get(msg.Channel)
		if !ok {
			return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "channel not found", msg.ID)
		}
		channel := record.(*channelRecord).channel

		// Validate channel with group.
		if msg.Group != "" {
			if channel.config == nil || channel.config.Group != msg.Group {
				return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "invalid channel for group", msg.ID)
			}
		}

		// Add source and modify message to prepare sending.
		msg.Source = ur.id
		msg.ID = 0

		// Check modifiers.
		ok = false
		var c *connection.Connection
		if msg.Group != "" && msg.Target == msg.Group {
			// Group mode, allow to continue without target connection.
			ok = true
		} else if msg.Target != "" {
			// Targeted hangup. Lookup and forward message.
			c, ok = channel.Get(msg.Target)
		}

		if msg.Subtype == api.RTMSubtypeNameWebRTCHangup {
			// XXX(longsleep): Find a better way to remove ourselves from channels.
			channel.Remove(ur.id)
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

		if c != nil {
			// Forward message to target if connection was found.
			c.Send(msg)
		}

	default:
		return api.NewRTMTypeError(api.RTMErrorIDBadMessage, "unknown subtype", msg.ID)
	}

	return nil
}

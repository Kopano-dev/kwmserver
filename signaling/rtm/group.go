/*
 * Copyright 2018 Kopano and its licensors
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
	"fmt"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/connection"
)

// CreateNamedGroupChannelID creates consistent channel IDs from
// input parameters.
func CreateNamedGroupChannelID(id string, m *Manager) (string, error) {
	return fmt.Sprintf("%s%s", ChannelPrefixNamedGroup, id), nil
}

func (m *Manager) onGroupReplace(channel *Channel, id string, oldConn *connection.Connection, newConn *connection.Connection) {
	data := &api.RTMDataWebRTCChannelExtra{}
	data.Replaced = true
	extra, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		oldConn.Logger().WithField("channel", channel.id).Errorln("failed to encode group replace data")
		return
	}

	err = oldConn.Send(&api.RTMTypeWebRTCReply{
		RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
			Type:    api.RTMTypeNameWebRTC,
			Subtype: api.RTMSubtypeNameWebRTCChannel,
		},
		Channel: channel.id,
		Data:    extra,
		Version: currentWebRTCPayloadVersion,
	})
	if err != nil {
		oldConn.Logger().WithError(err).WithField("channel", channel.id).Errorln("failed to send group replace to connection")
	}
}

func (m *Manager) onAfterGroupAddOrRemove(channel *Channel, op ChannelOp, id string) {
	members, connections := channel.Connections()

	data := &api.RTMDataWebRTCChannelExtra{}
	data.Group = &api.RTMTDataWebRTCChannelGroup{
		Group:   channel.config.Group,
		Members: members,
		Reset:   op == ChannelOpReset,
	}
	extra, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		m.logger.WithError(err).WithField("channel", channel.id).Errorln("failed to encode group data")
		return
	}

	payload, err := json.MarshalIndent(&api.RTMTypeWebRTCReply{
		RTMTypeSubtypeEnvelopeReply: &api.RTMTypeSubtypeEnvelopeReply{
			Type:    api.RTMTypeNameWebRTC,
			Subtype: api.RTMSubtypeNameWebRTCChannel,
		},
		Channel: channel.id,
		Data:    extra,
		Version: currentWebRTCPayloadVersion,
	}, "", "\t")
	if err != nil {
		m.logger.WithError(err).WithField("channel", channel.id).Errorln("failed to encode group channel data")
		return
	}

	idx := 0
	for _, cid := range members {
		c := connections[idx]
		idx++

		if cid == id {
			continue
		}

		err = c.RawSend(payload)
		if err != nil {
			c.Logger().WithError(err).WithField("channel", channel.id).Errorln("failed to send group channel to connection")
		}
	}
}

func (m *Manager) onAfterGroupChannelReset(channel *Channel) {
	members, connections := channel.Connections()

	m.logger.Debugln("onAfterGroupChannelReset", len(connections), len(members))
	// TODO(longsleep): Send data to each and every member so it reestablishes its connections.
	m.onAfterGroupAddOrRemove(channel, ChannelOpReset, "")
}

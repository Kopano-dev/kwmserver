/*
 * Copyright 2019 Kopano and its licensors
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
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsSubsystem = "rtm"
)

var (
	channelNew = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_created_channels",
			Help:      "Total number of created RTM channels",
		},
		[]string{"id"},
	)
	channelCleanup = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_cleanedup_channels",
			Help:      "Total number of cleaned up RTM channels",
		},
		[]string{"id"},
	)
	channelAdd = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_added_channel_connections",
			Help:      "Total number of connections added to RTM channels",
		},
		[]string{"id"},
	)
	channelRemove = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_removed_channel_connections",
			Help:      "Total number of connections removed from RTM channels",
		},
		[]string{"id"},
	)
	connectionAdd = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_connects",
			Help:      "Total number of connects to RTM signaling",
		},
		[]string{"id"},
	)
	connectionRemove = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_disconnects",
			Help:      "Total number of disconnects to RTM signaling",
		},
		[]string{"id"},
	)
	userNew = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_connected_distinct_users",
			Help:      "Total number of distinct user connects to RTM signaling",
		},
		[]string{"id"},
	)
	userCleanup = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "total_cleanedup_distinct_users",
			Help:      "Total number of distinct user cleanups to RTM signaling",
		},
		[]string{"id"},
	)
)

// MustRegister registers all rtm metrics with the provided registerer and
// panics upon the first registration that causes an error.
func MustRegister(reg prometheus.Registerer, cs ...prometheus.Collector) {
	reg.MustRegister(
		channelNew,
		channelCleanup,
		channelAdd,
		channelRemove,
		connectionAdd,
		connectionRemove,
		userNew,
		userCleanup,
	)
	reg.MustRegister(cs...)
}

type managerCollector struct {
	m *Manager

	channelsCountDesc *prometheus.Desc

	groupChannelsCountDesc            *prometheus.Desc
	groupChannelsConnectionsCountDesc *prometheus.Desc

	connectionsCountDesc *prometheus.Desc

	usersCountDesc *prometheus.Desc
}

// NewManagerCollector return as a collector that exports metrics of the
// provided Manager,
func NewManagerCollector(manager *Manager) prometheus.Collector {
	return &managerCollector{
		m: manager,

		channelsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_channels"),
			"Current number of RTM channels",
			[]string{"id"},
			nil,
		),

		groupChannelsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_group_channels"),
			"Current number of RTM group channels",
			[]string{"id"},
			nil,
		),
		groupChannelsConnectionsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_group_channels_connections"),
			"Current number of RTM group channel connections",
			[]string{"id"},
			nil,
		),

		connectionsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_connections"),
			"Current number of RTM connections",
			[]string{"id"},
			nil,
		),

		usersCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_users"),
			"Current number of RTM users",
			[]string{"id"},
			nil,
		),
	}
}

// Describe is implemented with DescribeByCollect. That's possible because the
// Collect method will always return the same two metrics with the same two
// descriptors.
func (mc *managerCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(mc, ch)
}

// Collect first gathers the associated managers collectors managers data. Then
// it creates constant metrics based on the returned data.
func (mc *managerCollector) Collect(ch chan<- prometheus.Metric) {
	numConnections := float64(mc.m.connections.Count())
	numUsers := float64(mc.m.users.Count())

	ch <- prometheus.MustNewConstMetric(
		mc.connectionsCountDesc,
		prometheus.GaugeValue,
		numConnections,
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.usersCountDesc,
		prometheus.GaugeValue,
		numUsers,
		mc.m.id,
	)

	var cr *channelRecord
	var numAllChannels uint64
	var numGroupChannels uint64
	var numGroupChannelsConnections uint64
	for entry := range mc.m.channels.IterBuffered() {
		cr = entry.Val.(*channelRecord)
		if cr.channel.config.Group != "" {
			numGroupChannels++
			cr.channel.RLock()
			numGroupChannelsConnections += uint64(len(cr.channel.connections))
			cr.channel.RUnlock()
		}
		numAllChannels++
	}
	ch <- prometheus.MustNewConstMetric(
		mc.channelsCountDesc,
		prometheus.GaugeValue,
		float64(numAllChannels),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsCountDesc,
		prometheus.GaugeValue,
		float64(numGroupChannels),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsConnectionsCountDesc,
		prometheus.GaugeValue,
		float64(numGroupChannelsConnections),
		mc.m.id,
	)
}

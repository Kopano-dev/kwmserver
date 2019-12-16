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
	"stash.kopano.io/kgol/ksurveyclient-go/counter"
)

const (
	metricsSubsystem = "rtm"
)

var (
	channelNew = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "channels_created_total",
			Help:      "Total number of created RTM channels",
		},
		[]string{"id"},
	)
	channelCleanup = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "channels_cleanedup_total",
			Help:      "Total number of cleaned up RTM channels",
		},
		[]string{"id"},
	)
	channelAdd = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "channels_connections_added_total",
			Help:      "Total number of connections added to RTM channels",
		},
		[]string{"id"},
	)
	channelRemove = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "channels_connections_removed_total",
			Help:      "Total number of connections removed from RTM channels",
		},
		[]string{"id"},
	)
	connectionAdd = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "connections_connected_total",
			Help:      "Total number of connects to RTM",
		},
		[]string{"id"},
	)
	connectionRemove = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "connections_disconnected_total",
			Help:      "Total number of disconnects from RTM",
		},
		[]string{"id"},
	)
	userNew = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "distinct_users_connected_total",
			Help:      "Total number of distinct user connects to RTM",
		},
		[]string{"id"},
	)
	userCleanup = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "distinct_users_cleanedup_total",
			Help:      "Total number of distinct user cleanups of RTM",
		},
		[]string{"id"},
	)
	httpRequestSuccessConnect = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "http_connect_success_total",
			Help:      "Total number of successful calls to RTM connect HTTP endpoint",
		},
		[]string{"id"},
	)
	httpRequestSuccessTURN = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "http_turn_success_total",
			Help:      "Total number of successful calls to RTM turn HTTP endpoint",
		},
		[]string{"id"},
	)
	httpRequestSuccessWebsocket = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "http_websocket_success_total",
			Help:      "Total number of successful calls to RTM websocket HTTP endpoint",
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
		httpRequestSuccessConnect,
		httpRequestSuccessTURN,
		httpRequestSuccessWebsocket,
	)
	reg.MustRegister(cs...)
}

type managerCollector struct {
	m *Manager

	channelsCountDesc       *prometheus.Desc
	channelsCountMaxDesc    *prometheus.Desc
	channelsCountMaxCounter counter.UintMinMax

	groupChannelsCountDesc       *prometheus.Desc
	groupChannelsCountMaxDesc    *prometheus.Desc
	groupChannelsCountMaxCounter counter.UintMinMax

	groupChannelsConnectionsCountDesc    *prometheus.Desc
	groupChannelsConnectionsMaxCountDesc *prometheus.Desc
	groupChannelsConnectionsMaxCounter   counter.UintMinMax

	connectionsCountDesc       *prometheus.Desc
	connectionsMaxCountDesc    *prometheus.Desc
	connectionsCountMaxCounter counter.UintMinMax

	usersCountDesc       *prometheus.Desc
	usersCountMaxDesc    *prometheus.Desc
	usersCountMaxCounter counter.UintMinMax
}

// NewManagerCollector return as a collector that exports metrics of the
// provided Manager,
func NewManagerCollector(manager *Manager) prometheus.Collector {
	return &managerCollector{
		m: manager,

		channelsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "channels_created_current"),
			"Current number of concurrent RTM channels",
			[]string{"id"},
			nil,
		),
		channelsCountMaxDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "channels_created_max"),
			"Maximum number of concurrent RTM channels",
			[]string{"id"},
			nil,
		),
		channelsCountMaxCounter: counter.GetUintMax(),

		groupChannelsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "group_channels_created_current"),
			"Current number of concurrent RTM group channels",
			[]string{"id"},
			nil,
		),
		groupChannelsCountMaxDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "group_channels_created_max"),
			"Maximum number of concurrent RTM group channels",
			[]string{"id"},
			nil,
		),
		groupChannelsCountMaxCounter: counter.GetUintMax(),

		groupChannelsConnectionsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "group_channels_connections_connected_current"),
			"Current number of concurrent connections attached to RTM group channels",
			[]string{"id"},
			nil,
		),
		groupChannelsConnectionsMaxCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "group_channels_connections_connected_max"),
			"Maximum number of concurrent connections to RTM group channels",
			[]string{"id"},
			nil,
		),
		groupChannelsConnectionsMaxCounter: counter.GetUintMax(),

		connectionsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "connections_connected_current"),
			"Current number of concurrent connections to RTM",
			[]string{"id"},
			nil,
		),
		connectionsMaxCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "connections_connected_max"),
			"Maximum number of concurrent connections to RTM",
			[]string{"id"},
			nil,
		),
		connectionsCountMaxCounter: counter.GetUintMax(),

		usersCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "distinct_users_connected_current"),
			"Current number of concurrent distinct users connected to RTM",
			[]string{"id"},
			nil,
		),
		usersCountMaxDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "distinct_users_connected_max"),
			"Maximum number of concurrent distinct users connected to RTM",
			[]string{"id"},
			nil,
		),
		usersCountMaxCounter: counter.GetUintMax(),
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
	numConnections := uint64(mc.m.connections.Count())
	mc.connectionsCountMaxCounter.Set(numConnections)

	ch <- prometheus.MustNewConstMetric(
		mc.connectionsCountDesc,
		prometheus.GaugeValue,
		float64(numConnections),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.connectionsMaxCountDesc,
		prometheus.CounterValue,
		float64(mc.connectionsCountMaxCounter.Value()),
		mc.m.id,
	)

	numUsers := uint64(mc.m.users.Count())
	mc.usersCountMaxCounter.Set(numUsers)

	ch <- prometheus.MustNewConstMetric(
		mc.usersCountDesc,
		prometheus.GaugeValue,
		float64(numUsers),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.usersCountMaxDesc,
		prometheus.CounterValue,
		float64(mc.usersCountMaxCounter.Value()),
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
	mc.channelsCountMaxCounter.Set(numAllChannels)
	mc.groupChannelsCountMaxCounter.Set(numGroupChannels)
	mc.groupChannelsConnectionsMaxCounter.Set(numGroupChannelsConnections)

	ch <- prometheus.MustNewConstMetric(
		mc.channelsCountDesc,
		prometheus.GaugeValue,
		float64(numAllChannels),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.channelsCountMaxDesc,
		prometheus.CounterValue,
		float64(mc.channelsCountMaxCounter.Value()),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsCountDesc,
		prometheus.GaugeValue,
		float64(numGroupChannels),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsCountMaxDesc,
		prometheus.CounterValue,
		float64(mc.groupChannelsCountMaxCounter.Value()),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsConnectionsCountDesc,
		prometheus.GaugeValue,
		float64(numGroupChannelsConnections),
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.groupChannelsConnectionsMaxCountDesc,
		prometheus.CounterValue,
		float64(mc.groupChannelsConnectionsMaxCounter.Value()),
		mc.m.id,
	)
}

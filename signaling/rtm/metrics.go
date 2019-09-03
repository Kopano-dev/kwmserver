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

	channelCountDesc     *prometheus.Desc
	connectionsCountDesc *prometheus.Desc
	userCountDesc        *prometheus.Desc
}

// NewManagerCollector return as a collector that exports metrics of the
// provided Manager,
func NewManagerCollector(manager *Manager) prometheus.Collector {
	return &managerCollector{
		m: manager,

		channelCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_channels"),
			"Current number of RTM channels",
			[]string{"id"},
			nil,
		),
		connectionsCountDesc: prometheus.NewDesc(
			prometheus.BuildFQName("", metricsSubsystem, "current_connections"),
			"Current number of RTM connections",
			[]string{"id"},
			nil,
		),
		userCountDesc: prometheus.NewDesc(
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
	numChannels := float64(mc.m.channels.Count())
	numConnections := float64(mc.m.connections.Count())
	numUsers := float64(mc.m.users.Count())

	ch <- prometheus.MustNewConstMetric(
		mc.channelCountDesc,
		prometheus.GaugeValue,
		numChannels,
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.connectionsCountDesc,
		prometheus.GaugeValue,
		numConnections,
		mc.m.id,
	)
	ch <- prometheus.MustNewConstMetric(
		mc.userCountDesc,
		prometheus.GaugeValue,
		numUsers,
		mc.m.id,
	)
}

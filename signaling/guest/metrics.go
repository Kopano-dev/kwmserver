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

package guest

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsSubsystem = "guest"
)

var (
	httpRequestSucessLogon = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: metricsSubsystem,
			Name:      "http_logon_success_total",
			Help:      "Total number of successful calls to Guest logon HTTP endpoint",
		},
		[]string{"id"},
	)
)

// MustRegister registers all guest metrics with the provided registerer and
// panics upon the first registration that causes an error.
func MustRegister(reg prometheus.Registerer, cs ...prometheus.Collector) {
	reg.MustRegister(
		httpRequestSucessLogon,
	)
	reg.MustRegister(cs...)
}

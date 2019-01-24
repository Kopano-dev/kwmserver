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

package signaling

import ()

// Service is an interface for services providing information about activity.
type Service interface {
	NumActive() uint64
}

// Services is a defined collection of services which handle activity.
type Services struct {
	AdminManager Service
	MCUManager   Service
	RTMManager   Service
	GuestManager Service
}

// Services returns all active services of the accociated Services as iterable.
func (services *Services) Services() []Service {
	s := make([]Service, 0)

	if services.AdminManager != nil {
		s = append(s, services.AdminManager)
	}
	if services.MCUManager != nil {
		s = append(s, services.MCUManager)
	}
	if services.RTMManager != nil {
		s = append(s, services.RTMManager)
	}
	if services.GuestManager != nil {
		s = append(s, services.GuestManager)
	}

	return s
}

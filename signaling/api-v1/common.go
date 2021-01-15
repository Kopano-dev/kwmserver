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

package api

// Self contains a user ID conected to a name.
type Self struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	Auth string `json:"auth"`
}

// ServerStatus contains server status information to share with clients.
type ServerStatus struct {
	Kustomer *uint64 `json:"kustomer,omitempty"`
}

// Equal reports wether serverStatus and otherServers are "deeply equal".
func (serverStatus *ServerStatus) Equal(otherServerStatus *ServerStatus) bool {
	return serverStatus.Kustomer == otherServerStatus.Kustomer
}

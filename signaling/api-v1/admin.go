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

package api

// An AdminAuthToken represents an auth token with type and value and an optional
// session ID.
type AdminAuthToken struct {
	Subject   string `json:"sub,omitempty"`
	Type      string `json:"type"`
	Value     string `json:"value,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`

	Claims map[string]interface{} `json:"claims,omitempty"`
}

// Name returns the associated tokens name claim string value or empty string.
func (aat *AdminAuthToken) Name() string {
	if aat.Claims != nil {
		if ncv, ok := aat.Claims[NameClaim]; ok {
			if name, ok := ncv.(string); ok {
				return name
			}
		}
	}

	return ""
}

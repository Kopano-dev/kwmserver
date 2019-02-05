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

package clients

import (
	"crypto"
)

// Application types for clients.
const (
	ApplicationTypeWeb    = "web"
	ApplicationTypeNative = "native"
)

// Details hold detail information about clients identified by ID.
type Details struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Trusted     bool   `json:"trusted"`
}

// A Secured is a client records public key identified by ID.
type Secured struct {
	ID              string
	DisplayName     string
	ApplicationType string

	Kid        string
	PublicKey  crypto.PublicKey
	PrivateKey crypto.PrivateKey

	TrustedScopes []string
}

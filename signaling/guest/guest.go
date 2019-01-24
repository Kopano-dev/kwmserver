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
	"encoding/json"
)

// Claims as used by kwmserver guest support.
const (
	RequestGuestClaim = "guest"
	NameClaim         = "name"
)

// NOTE(longsleep): The request claims related structs in this file are copied
// over from Konnect and were stripped down to the the used functionality in
// this guest support implementation.

// RequestObjectClaims holds the incoming request object claims provided as
// JWT via request parameter to OpenID Connect 1.0 authorization endpoint
// requests as used by kwmserver guest support. Specification at
// https://openid.net/specs/openid-connect-core-1_0.html#JWTRequests
type RequestObjectClaims struct {
	ClientID        string `json:"client_id"`
	RawResponseType string `json:"response_type,omitempty"`
	RawScope        string `json:"scope,omitempty"`

	Claims *ClaimsRequest `json:"claims,omitempty"`
}

// Valid implements the claims interface.
func (roc *RequestObjectClaims) Valid() error {
	return nil
}

// ClaimsRequest define the base claims structure for OpenID Connect claims
// request parameter value as specified at
// https://openid.net/specs/openid-connect-core-1_0.html#ClaimsParameter - in
// addition a Konnect specific pass thru value can be used to pass through any
// application specific values to access and reqfresh tokens.
type ClaimsRequest struct {
	UserInfo *ClaimsRequestMap `json:"userinfo,omitempty"`
	IDToken  *ClaimsRequestMap `json:"id_token,omitempty"`
	Passthru json.RawMessage   `json:"passthru,omitempty"`
}

// ClaimsRequestMap defines a mapping of claims request values used with
// OpenID Connect claims request parameter values.
type ClaimsRequestMap map[string]*ClaimsRequestValue

// ClaimsRequestValue is the claims request detail definition of an OpenID
// Connect claims request parameter value.
type ClaimsRequestValue struct {
	Essential bool          `json:"essential,omitempty"`
	Value     interface{}   `json:"value,omitempty"`
	Values    []interface{} `json:"values,omitempty"`
}

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
	"fmt"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/rs/cors"
	"stash.kopano.io/kgol/rndm"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

var corsHandler = cors.Default()

func (m *Manager) corsAllowed(next http.Handler) http.Handler {
	return corsHandler.Handler(next)
}

// MakeHTTPLogonHandler implements the HTTP handler for guest logon requests.
func (m *Manager) MakeHTTPLogonHandler() http.Handler {
	return m.corsAllowed(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(rw, "", http.StatusMethodNotAllowed)
			return
		}

		if m.clients == nil {
			m.logger.Debugln("guest logon request but no keys are registered")
			http.Error(rw, "guest access is not set up", http.StatusNotFound)
			return
		}

		err := req.ParseForm()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		// Get client_id and response_type from request.
		clientID := req.Form.Get("client_id")
		responseType := req.Form.Get("response_type")
		scope := req.Form.Get("scope")
		if clientID == "" {
			http.Error(rw, "client_id is empty", http.StatusBadRequest)
		}

		guest := req.Form.Get("guest")
		name := req.Form.Get("name")
		m.logger.WithField("guest", guest).Debugln("guest handler logon request")

		var gc *Claims
		claims := &ClaimsRequest{}

		// Flexible guest type support.
		switch guest {
		case guestTypeSimple:
			// Guest mode 1 supports access via a path parameter to a specific
			// public group, or access via a token which was previously issued
			// to grant access to a specific private group.
			path := strings.TrimSpace(req.Form.Get("path"))
			if path == "" {
				http.Error(rw, "empty path", http.StatusBadRequest)
				return
			}
			token := req.Form.Get("token")
			if token != "" {
				// TODO(longsleep): Validate guest token.
				http.Error(rw, "token validation not implemented", http.StatusNotImplemented)
				return
			} else {
				// Validate path.
				if !m.isValidPublicPath(path, guest) {
					http.Error(rw, "guest access denied", http.StatusForbidden)
					return
				}
			}

			// Set guest claims.
			gc = &Claims{
				Type: guest,
				Path: path,
			}

		default:
			// Unknown or unsupported guest mode.
			http.Error(rw, "unknown guest type", http.StatusBadRequest)
			return
		}

		// TODO(longsleep): Optionally get id token hint from request to renew
		// previously created data?

		// TODO(longsleep): Get client secret from request.
		// TODO(longsleep): Use origin from request.
		_, client, err := m.clients.Lookup(req.Context(), clientID, "", "", true)
		if err != nil {
			m.logger.WithError(err).WithField("client_id", clientID).Debugln("client lookup failed")
			http.Error(rw, "guest access denied", http.StatusForbidden)
			return
		}
		alg := jwt.GetSigningMethod(client.RawRequestObjectSigningAlg)
		if alg == nil {
			m.logger.WithError(err).WithField("client_id", clientID).Debugln("no request object signing alg for client_id key")
			http.Error(rw, "guest access denied", http.StatusForbidden)
		}
		secured, err := client.Private(nil)
		if err != nil {
			m.logger.WithError(err).WithField("client_id", clientID).Debugln("no default key for client_id")
			http.Error(rw, "guest access denied", http.StatusForbidden)
			return
		}

		// Add pass thru claims.
		err = claims.SetPassthru(&passthruClaims{
			Guest: gc,
		})
		if err != nil {
			m.logger.WithError(err).Errorln("failed to set guest pass thru claims")
			http.Error(rw, "failed to set pass thru claims", http.StatusInternalServerError)
			return
		}

		// Generate random id, claims and request.
		id := fmt.Sprintf("%s@%s.kwmguest", rndm.GenerateRandomString(32), guest)
		claims.IDToken = &ClaimsRequestMap{
			RequestGuestClaim: &ClaimsRequestValue{
				Essential: true,
				Value:     id,
			},
			NameClaim: &ClaimsRequestValue{
				Value: guestDisplayName(name),
			},
		}
		request := &RequestObjectClaims{
			ClientID:        clientID,
			RawResponseType: responseType,
			RawScope:        scope,

			Claims: claims,
		}

		// Sign request.
		requestToken := jwt.NewWithClaims(alg, request)
		requestToken.Header["kid"] = secured.Kid

		// Create query parameters.
		eqp := make(map[string]string)
		eqp["request"], err = requestToken.SignedString(secured.PrivateKey)
		if err != nil {
			panic(err)
		}

		// API response.
		response := &api.GuestLogonResponse{
			ResponseOK: *api.ResponseOKValue,

			ExtraQueryParams: eqp,

			ID:   id,
			Name: name,
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}))
}

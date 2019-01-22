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

package rtm

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	kcoidc "stash.kopano.io/kc/libkcoidc"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/turn"
)

var corsHandler = cors.New(cors.Options{
	// TODO(longsleep): Add to configuration.
	AllowedOrigins:   []string{"*"},
	AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
	AllowCredentials: true,
})

func (m *Manager) corsAllowed(next http.Handler) http.Handler {
	return corsHandler.Handler(next)
}

func (m *Manager) isRequestWithValidAuth(req *http.Request) (*api.AdminAuthToken, bool) {
	authHeader := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(authHeader) != 2 {
		return nil, false
	}

	switch authHeader[0] {
	case api.AdminAuthTokenTypeToken:
		// Self created token.
		return m.adminm.IsValidAdminAuthTokenRequest(req)

	case "Bearer":
		if m.oidcp != nil {
			authenticatedUserID, std, claims, err := m.oidcp.ValidateTokenString(req.Context(), authHeader[1])

			if err == nil {
				if claims != nil && claims.KCTokenType() == kcoidc.TokenTypeKCAccess {
					err = claims.Valid()
				} else {
					err = errors.New("missing access token claim")
				}
			}

			if err == nil && len(m.requiredScopes) > 0 {
				// Check required scopes.
				err = kcoidc.RequireScopesInClaims(claims, m.requiredScopes)
			}

			if err != nil {
				m.logger.WithError(err).Errorln("rtm connect bearer auth failed")
				return nil, false
			}

			var userClaims map[string]interface{}

			// Check for id token support.
			if idTokenString := req.Form.Get("id_token"); idTokenString != "" {
				_, idStd, idExtra, idErr := m.oidcp.ValidateTokenString(req.Context(), idTokenString)
				if idErr == kcoidc.ErrStatusTokenExpiredOrNotValidYet {
					// Allow ID tokens to be expired.
					idErr = nil
				}
				if idErr == nil && (idStd.Subject != std.Subject || idStd.Issuer != std.Issuer) {
					idErr = errors.New("id token does not match access token")
				}
				if idErr != nil {
					m.logger.WithError(idErr).Errorln("rtm connect bearer auth with id token failed")
					return nil, false
				}

				// Set authenticated user and claims for further processing.
				req.Form.Set("user", authenticatedUserID)
				userClaims = map[string]interface{}(*idExtra)
			}

			return &api.AdminAuthToken{
				Subject:   authenticatedUserID,
				Type:      authHeader[0],
				Value:     authHeader[1],
				ExpiresAt: std.ExpiresAt,

				Claims: userClaims,
			}, true
		}
	}

	return nil, false
}

// MakeHTTPConnectHandler createss the HTTP handler for rtm.connect.
func (m *Manager) MakeHTTPConnectHandler(router *mux.Router, websocketRouteIdentifier string) http.Handler {
	return m.corsAllowed(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		req.ParseForm()

		// Check authentication
		auth, authOK := m.isRequestWithValidAuth(req)
		if !authOK {
			http.Error(rw, "", http.StatusForbidden)
			return
		}

		user := req.Form.Get("user")
		if user == "" {
			http.Error(rw, "missing user parameter", http.StatusBadRequest)
			return
		}

		if !m.insecure && user != auth.Subject {
			http.Error(rw, "user does not match auth", http.StatusForbidden)
			return
		}

		m.adminm.RefreshAdminAuthToken(auth)

		// create random URL to websocket endpoint
		key, err := m.Connect(req.Context(), user, auth)
		if err != nil {
			m.logger.WithError(err).Errorln("rtm connect failed")
			http.Error(rw, "request failed", http.StatusInternalServerError)
			return
		}

		route := router.Get(websocketRouteIdentifier)
		websocketURI, err := route.URLPath("key", key)
		if err != nil {
			m.logger.WithError(err).Errorln("rtm connect url generation failed")
			http.Error(rw, "request failed", http.StatusInternalServerError)
			return
		}

		var turnConfig *turn.ClientConfig
		// fetch TURN credentials
		if m.turnsrv != nil {
			turnConfig, err = m.turnsrv.GetConfig(req.Context(), user)
			if err != nil {
				m.logger.WithError(err).Errorln("rtm connect TURN config failed")
				http.Error(rw, "TURN config failed", http.StatusInternalServerError)
				return
			}
		}

		response := &api.RTMConnectResponse{
			ResponseOK: *api.ResponseOKValue,

			URL: websocketURI.String(),
			Self: &api.Self{
				ID:   user,
				Name: auth.Name(),
			},

			TURN: turnConfig,
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}))
}

// MakeHTTPTURNHandler creates the HTTP handler for rtm.turn.
func (m *Manager) MakeHTTPTURNHandler(router *mux.Router) http.Handler {
	if m.turnsrv == nil {
		return http.HandlerFunc(http.NotFound)
	}

	return m.corsAllowed(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Check authentication
		auth, authOK := m.isRequestWithValidAuth(req)
		if !authOK {
			http.Error(rw, "", http.StatusForbidden)
			return
		}

		req.ParseForm()
		user := req.Form.Get("user")
		if user == "" {
			http.Error(rw, "missing user parameter", http.StatusBadRequest)
			return
		}

		if !m.insecure && user != auth.Subject {
			http.Error(rw, "user does not match auth", http.StatusForbidden)
			return
		}

		m.adminm.RefreshAdminAuthToken(auth)

		// fetch TURN credentials
		turnConfig, err := m.turnsrv.GetConfig(req.Context(), user)
		if err != nil {
			m.logger.WithError(err).Errorln("rtm connect TURN config failed")
			http.Error(rw, "TURN config failed", http.StatusInternalServerError)
			return
		}

		response := &api.RTMTURNResponse{
			ResponseOK: *api.ResponseOKValue,

			TURN: turnConfig,
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(response)
	}))
}

// HTTPWebsocketHandler implements the HTTP handler for websocket requests.
func (m *Manager) HTTPWebsocketHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(rw, "", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(req)
	key, ok := vars["key"]
	if !ok {
		http.NotFound(rw, req)
		return
	}

	err := m.HandleWebsocketConnect(req.Context(), key, rw, req)
	if err != nil {
		m.logger.WithError(err).Errorln("websocket connection failed")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

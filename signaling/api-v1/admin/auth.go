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

package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"stash.kopano.io/kgol/rndm"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

const (
	adminAuthTokenTokensRecordIDPrefix = "admin"
	adminAuthTokenDuration             = time.Hour * 24 * 365 // 1 Year
)

func getAdminAuthTokenTokensRecordID(token *api.AdminAuthToken) string {
	return fmt.Sprintf("%s-%s-%s", adminAuthTokenTokensRecordIDPrefix, token.Type, token.Subject)
}

// SignAdminAuthToken signs the provided token and returns its signed string
// value.
func (m *Manager) SignAdminAuthToken(token *api.AdminAuthToken) (string, error) {
	claims := &jwt.StandardClaims{
		ExpiresAt: token.ExpiresAt,
		IssuedAt:  time.Now().Unix(),
		Subject:   token.Subject,
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t.Header["kid"] = m.tokensSigningKeyID
	key, ok := m.tokensKeys[m.tokensSigningKeyID]
	if !ok {
		return "", fmt.Errorf("unknown key id: %v", m.tokensSigningKeyID)
	}

	return t.SignedString(key)
}

// ValidateAdminAuthTokenString decodes and validates the provided token string
// value.
func (m *Manager) ValidateAdminAuthTokenString(tokenString string) (*api.AdminAuthToken, error) {
	parser := &jwt.Parser{}
	token, err := parser.ParseWithClaims(tokenString, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := (token.Header["kid"]).(string)
		if !ok {
			return nil, fmt.Errorf("invalid key id: %v", token.Header["kid"])
		}
		key, ok := m.tokensKeys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id: %v", kid)
		}

		return key, nil
	})
	if err != nil {
		validationError := err.(*jwt.ValidationError)
		if validationError.Errors&jwt.ValidationErrorSignatureInvalid != 0 {
			return nil, fmt.Errorf("token signature invalid failed: %v", err)
		}
	}

	claims := token.Claims.(*jwt.StandardClaims)
	return &api.AdminAuthToken{
		Subject:   claims.Subject,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

// IsValidAdminAuthToken checks if the provided token is known to the accociated manager.
func (m *Manager) IsValidAdminAuthToken(token *api.AdminAuthToken) bool {
	existing, exists := m.GetToken(getAdminAuthTokenTokensRecordID(token))
	if !exists || existing == nil {
		return false
	}
	return true
}

// IsValidAdminAuthTokenRequest checks if the provided request has an Authorization
// header which is a valid admin auth token at the accociated manager.
func (m *Manager) IsValidAdminAuthTokenRequest(req *http.Request) (*api.AdminAuthToken, bool) {
	auth := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
	if len(auth) != 2 {
		return nil, false
	}
	if auth[0] != api.AdminAuthTokenTypeToken {
		return nil, false
	}

	token, err := m.ValidateAdminAuthTokenString(auth[1])
	if err != nil {
		m.Logger().WithError(err).Debugln("validate admin auth token string failed")
		return nil, false
	}

	token.Type = auth[0]
	if !m.IsValidAdminAuthToken(token) {
		expiresAt := time.Unix(token.ExpiresAt, 0)
		if expiresAt.After(time.Now()) {
			// Resurrect token when not expired.
			m.SetToken(getAdminAuthTokenTokensRecordID(token), token)
		} else {
			m.Logger().WithError(err).Debugln("admin auth token is not valid")
			return nil, false
		}
	}

	return token, true
}

// RefreshAdminAuthToken updates the timestamp of the token record of the
// provided token if known to the accociated manager.
func (m *Manager) RefreshAdminAuthToken(token *api.AdminAuthToken) {
	m.RefreshToken(getAdminAuthTokenTokensRecordID(token))
}

func (m *Manager) addAuthRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	router.Handle("/tokens", wrapper(http.HandlerFunc(m.createAuthToken))).Methods(http.MethodPost)
	router.Handle("/tokens", wrapper(http.HandlerFunc(m.listValidAuthTokens))).Methods(http.MethodGet)
	router.Handle("/tokens", wrapper(http.HandlerFunc(m.removeAuthToken))).Methods(http.MethodDelete)

	return router
}

func (m *Manager) createAuthToken(rw http.ResponseWriter, req *http.Request) {
	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	// TODO(longsleep): Add maximum limit of creatable tokens.
	msg, err := ioutil.ReadAll(io.LimitReader(req.Body, maxRequestSize))
	if err != nil {
		m.Logger().WithError(err).Debugln("failed to read request body")
		http.Error(rw, fmt.Errorf("failed to read request: %v", err).Error(), http.StatusBadRequest)
		return
	}

	var token api.AdminAuthToken
	err = json.Unmarshal(msg, &token)
	if err != nil {
		m.Logger().WithError(err).Debugln("failed to parse request")
		http.Error(rw, fmt.Errorf("failed to parse: %v", err).Error(), http.StatusBadRequest)
		return
	}

	if token.Type == "" {
		http.Error(rw, fmt.Errorf("type cannot be empty").Error(), http.StatusBadRequest)
		return
	}
	if token.Type != api.AdminAuthTokenTypeToken {
		http.Error(rw, fmt.Errorf("unknown token type: %s", token.Type).Error(), http.StatusBadRequest)
		return
	}
	if token.Value != "" {
		http.Error(rw, fmt.Errorf("value must be empty").Error(), http.StatusBadRequest)
		return
	}
	if token.ExpiresAt != 0 {
		http.Error(rw, fmt.Errorf("exp must be 0").Error(), http.StatusBadRequest)
		return
	}

	if token.Subject == "" {
		// Generate random server generated value.
		token.Subject = rndm.GenerateRandomString(32)
	}

	token.ExpiresAt = time.Now().Add(adminAuthTokenDuration).Unix()
	tokenValue, err := m.SignAdminAuthToken(&token)
	if err != nil {
		m.Logger().WithError(err).Errorln("failed to sign admin auth token")
		http.Error(rw, fmt.Errorf("failed to sign token").Error(), http.StatusInternalServerError)
		return
	}
	token.Value = tokenValue
	m.SetToken(getAdminAuthTokenTokensRecordID(&token), &token)

	rw.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(rw)
	encoder.SetIndent("", "\t")
	encoder.Encode(&token)
}

func (m *Manager) listValidAuthTokens(rw http.ResponseWriter, req *http.Request) {
	filterPrefix := fmt.Sprintf("%s-", adminAuthTokenTokensRecordIDPrefix)
	result := []*api.AdminAuthToken{}
	count := 0
	for entry := range m.tokens.IterBuffered() {
		if strings.HasPrefix(entry.Key, filterPrefix) {

			result = append(result, entry.Val.(*tokenRecord).token.(*api.AdminAuthToken))
			count++
		}
		if count >= 5000 {
			// XXX(longsleep): Make this a parameter. Absolute maximum should stil be hardcoded.
			break
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(rw)
	encoder.SetIndent("", "\t")
	encoder.Encode(&result)
}

func (m *Manager) removeAuthToken(rw http.ResponseWriter, req *http.Request) {
	// TODO(longsleep): Reuse msg []byte slices / put into pool.
	msg, err := ioutil.ReadAll(io.LimitReader(req.Body, maxRequestSize))
	if err != nil {
		m.Logger().WithError(err).Debugln("failed to read request body")
		http.Error(rw, fmt.Errorf("failed to read request: %v", err).Error(), http.StatusBadRequest)
		return
	}

	var token api.AdminAuthToken
	err = json.Unmarshal(msg, &token)
	if err != nil {
		m.Logger().WithError(err).Debugln("failed to parse request")
		http.Error(rw, fmt.Errorf("failed to parse: %v", err).Error(), http.StatusBadRequest)
		return
	}

	if token.Type == "" || token.Subject == "" {
		http.Error(rw, fmt.Errorf("type or value cannot be empty").Error(), http.StatusBadRequest)
		return
	}

	_, exists := m.PopToken(getAdminAuthTokenTokensRecordID(&token))
	if !exists {
		http.NotFound(rw, req)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

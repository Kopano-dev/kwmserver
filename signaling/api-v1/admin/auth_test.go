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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

func TestAdminAuthTokensHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpServer, _, router := newTestManager(ctx, t)
	defer httpServer.Close()

	url := "/admin/auth/tokens"
	tests := []struct {
		description         string
		method              string
		body                string
		expectedContentType string
		expectedStatus      int
		expectedBody        string
		expectedBodyFunc    func(body string)
		wait                time.Duration
	}{
		{
			description:         "successful empty GET",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBody:        "[]\n",
		},
		{
			description:         "successful POST",
			method:              http.MethodPost,
			body:                `{"type": "Token"}`,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc: func(body string) {
				var token api.AdminAuthToken
				err := json.Unmarshal([]byte(body), &token)
				if err != nil {
					t.Errorf("successful POST failed to unmarshal JSON: %v", err)
					return
				}

				if token.Type != "Token" {
					t.Errorf("successful POST handler returned wrong token type: got %v, want unit-test", token.Type)
				}
				if token.Subject == "" {
					t.Errorf("successful POST handler returned token with empty subject")
				}
				if token.Value == "" {
					t.Errorf("successful POST handler returned token with empty value")
				}
				if token.ExpiresAt == 0 {
					t.Errorf("successful POST handler returned token with empty expiry")
				} else {
					expiresAt := time.Unix(token.ExpiresAt, 0)
					now := time.Now()
					if expiresAt.Before(now) {
						t.Errorf("successful POST handler returned token which is already expired: %v is before %v", expiresAt, now)
					}
				}
			},
		},
		{
			description:         "successful GET after successful POST",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc: func(body string) {
				var tokens []*api.AdminAuthToken
				err := json.Unmarshal([]byte(body), &tokens)
				if err != nil {
					t.Errorf("successful GET after successful POST failed to unmarshal JSON: %v", err)
					return
				}

				if len(tokens) != 1 {
					t.Errorf("successful GET after successful POST handler returned wrong number of tokens: got %v, want 1", len(tokens))
				}
			},
		},
		{
			description:         "successful POST with subject",
			method:              http.MethodPost,
			body:                `{"type": "Token", "sub": "wonderful"}`,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc:    func(body string) {},
		},
		{
			description:         "successful GET after successful second POST",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc: func(body string) {
				var tokens []*api.AdminAuthToken
				err := json.Unmarshal([]byte(body), &tokens)
				if err != nil {
					t.Errorf("successful GET after successful second POST failed to unmarshal JSON: %v", err)
					return
				}

				if len(tokens) != 2 {
					t.Errorf("successful GET after successful second POST handler returned wrong number of tokens: got %v, want 2", len(tokens))
				}
			},
		},
		{
			description:         "successful DELETE with none-existing subject",
			method:              http.MethodDelete,
			body:                `{"type": "Token", "sub": "does-not-exist"}`,
			expectedContentType: "text/plain; charset=utf-8",
			expectedStatus:      http.StatusNotFound,
			expectedBody:        "404 page not found\n",
		},
		{
			description:         "successful GET after DELETE of non existing",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc: func(body string) {
				var tokens []*api.AdminAuthToken
				err := json.Unmarshal([]byte(body), &tokens)
				if err != nil {
					t.Errorf("successful GET after DELETE of non existing failed to unmarshal JSON: %v", err)
					return
				}

				if len(tokens) != 2 {
					t.Errorf("successful GET after DELETE of non existing handler returned wrong number of tokens: got %v, want 2", len(tokens))
				}
			},
		},
		{
			description:         "successful DELETE with existing subject",
			method:              http.MethodDelete,
			body:                `{"type": "Token", "sub": "wonderful"}`,
			expectedContentType: "",
			expectedStatus:      http.StatusNoContent,
			expectedBody:        "",
		},
		{
			description:         "successful GET after successful DELETE",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBodyFunc: func(body string) {
				var tokens []*api.AdminAuthToken
				err := json.Unmarshal([]byte(body), &tokens)
				if err != nil {
					t.Errorf("successful GET after successful DELETE failed to unmarshal JSON: %v", err)
					return
				}

				if len(tokens) != 1 {
					t.Errorf("successful GET after successful DELETE handler returned wrong number of tokens: got %v, want 1", len(tokens))
				}
			},
		},
		{
			description:         "successful empty GET",
			method:              http.MethodGet,
			expectedContentType: "application/json",
			expectedStatus:      http.StatusOK,
			expectedBody:        "[]\n",
			wait:                2 * time.Minute,
		},
	}

	for _, tc := range tests {
		if tc.wait > 0 {
			if testing.Short() {
				continue
			}
			t.Logf("%s is waiting %v to start", tc.description, tc.wait)
			time.Sleep(tc.wait)
		}

		// Prepare the request to pass to our handler.
		var body io.Reader
		if tc.body != "" {
			body = bytes.NewBufferString(tc.body)
		}
		req, err := http.NewRequest(tc.method, url, body)
		if err != nil {
			t.Error(err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		// Create response recorder to record the response.
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != tc.expectedStatus {
			t.Errorf("%s handler returned wrong status code: got %v want %v", tc.description, status, tc.expectedStatus)
		}
		if ct := rr.Header().Get("Content-Type"); ct != tc.expectedContentType {
			t.Errorf("%s Content-Type response header was incorrect, got %s, want %s", tc.description, ct, tc.expectedContentType)
		}
		if tc.expectedBodyFunc == nil {
			if body := rr.Body.String(); body != tc.expectedBody {
				t.Errorf("%s handler returned wrong body: got %v want %v", tc.description, body, tc.expectedBody)
			}
		} else {
			tc.expectedBodyFunc(rr.Body.String())
		}
	}
}

/*
 * Copyright 2018 Kopano and its licensors
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

package turn

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// KWMUserHTTPHeaderName name to set for service requests to external TURN server.
const KWMUserHTTPHeaderName = "X-Kopano-KWM-User"

type serverResponse struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	TTL      int64    `json:"ttl"`
	URIs     []string `json:"uris"`
}

func (sr *serverResponse) ClientConfig() (*ClientConfig, error) {
	if sr.Username == "" || len(sr.URIs) == 0 {
		return nil, fmt.Errorf("TURN service data not available")
	}

	ttl := sr.TTL
	if ttl < 60 {
		// Add some protection, in case TURN service returns garbage.
		ttl = 60
	}

	return &ClientConfig{
		Username: sr.Username,
		Password: sr.Password,
		TTL:      ttl,
		URIs:     sr.URIs,
	}, nil
}

// ServerAuthServer implements means to create TURN credentials using a web
// service which is protected by HTTP basic auth.
type ServerAuthServer struct {
	url    string
	client *http.Client

	username string
	password string
}

// NewServerAuthServer creates a new ServerAuthServer with the provided options.
func NewServerAuthServer(url, username, password string, client *http.Client) (*ServerAuthServer, error) {
	s := &ServerAuthServer{
		url: url,

		username: username,
		password: password,
	}
	if client == nil {
		client = &http.Client{
			Timeout: time.Second * 60,
		}
	}
	s.client = client

	return s, nil
}

// GetConfig returns the client config record for the provided username using
// the accociated servers data.
func (s *ServerAuthServer) GetConfig(ctx context.Context, username string) (*ClientConfig, error) {
	h := sha256.New()
	h.Write([]byte(username))
	un := base64.URLEncoding.EncodeToString(h.Sum(nil))

	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(KWMUserHTTPHeaderName, un)
	req.SetBasicAuth(s.username, s.password)
	req = req.WithContext(ctx)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TURN service unexpected response status: %d", resp.StatusCode)
	}

	data := &serverResponse{}
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return nil, err
	}

	return data.ClientConfig()
}

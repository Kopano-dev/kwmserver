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

package turn

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

// SharedsecretServer implements the means to create TURN credentials using
// a configured shared secret.
type SharedsecretServer struct {
	ttl          int64
	sharedSecret []byte

	uris []string
}

// NewSharedsecretServer creates a new Server with the provided options.
func NewSharedsecretServer(uris []string, sharedSecret []byte, ttl int64) (*SharedsecretServer, error) {
	if ttl == 0 {
		ttl = DefaultServerTTL
	}

	return &SharedsecretServer{
		ttl:          ttl,
		sharedSecret: sharedSecret,

		uris: uris,
	}, nil
}

// GenerateUsername generates a new TURN username for the provided user using
// the accociated server.
func (s *SharedsecretServer) GenerateUsername(username string) (string, error) {
	timestamp := time.Now().Unix() + s.ttl
	return fmt.Sprintf("%d:%s", timestamp, username), nil
}

// GenerateTURNPassword generates a new TURN password for the provided user using
// the accociated server data.
func (s *SharedsecretServer) GenerateTURNPassword(username string) (string, error) {
	h := hmac.New(sha1.New, s.sharedSecret)
	h.Write([]byte(username))

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// GetConfig returns the client config record for the provided username using
// the accociated servers data.
func (s *SharedsecretServer) GetConfig(ctx context.Context, username string) (*ClientConfig, error) {
	turnUsername, err := s.GenerateUsername(username)
	if err != nil {
		return nil, err
	}
	turnPassword, err := s.GenerateTURNPassword(username)
	if err != nil {
		return nil, err
	}

	return &ClientConfig{
		Username: turnUsername,
		Password: turnPassword,
		TTL:      s.ttl,
		URIs:     s.uris,
	}, nil
}

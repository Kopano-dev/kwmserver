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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var adminTokenSigningKey = []byte("test-key")

func dummyWrapper(next http.Handler) http.Handler {
	return next
}

func newTestManager(ctx context.Context, t *testing.T) (*httptest.Server, *Manager, http.Handler) {
	manager := NewManager(ctx, "", logrus.New())
	manager.AddTokenKey("", adminTokenSigningKey)

	router := mux.NewRouter()

	manager.AddRoutes(ctx, router, dummyWrapper)

	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		router.ServeHTTP(rw, req)
	}))

	return s, manager, router
}

func TestNewTestManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newTestManager(ctx, t)
}

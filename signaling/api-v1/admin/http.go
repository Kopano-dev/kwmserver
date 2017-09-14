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

	"github.com/gorilla/mux"
)

// AddRoutes adds HTTP routes to the provided router, wrapped with the provided
// wrapper where appropriate.
func (m *Manager) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	r := router.PathPrefix("/admin").Subrouter()
	m.addAuthRoutes(ctx, r.PathPrefix("/auth").Subrouter(), wrapper)

	return router
}

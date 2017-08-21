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

package www

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// HTTPService binds the HTTP router with handlers for static www files.
type HTTPService struct {
	prefix string

	fs http.Handler
}

// NewHTTPService creates a new HTTP Janus API service with the provided options.
func NewHTTPService(ctx context.Context, logger logrus.FieldLogger, prefix, folder string) *HTTPService {
	return &HTTPService{
		prefix: prefix,

		fs: http.FileServer(http.Dir(folder)),
	}
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (h *HTTPService) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	router.PathPrefix(h.prefix).Handler(wrapper(http.StripPrefix(h.prefix, h.fs)))

	return router
}

// NumActive returns the number of the currently active connections at the
// accociated api..
func (h *HTTPService) NumActive() uint64 {
	return 0
}

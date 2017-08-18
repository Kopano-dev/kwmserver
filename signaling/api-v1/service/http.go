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

package handler

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling"
)

const (
	// URIPrefix defines the URL prefixed uses for API v1 requests.
	URIPrefix = "/api/v1"
)

// HTTPService binds the HTTP router with handlers for kwm API v1.
type HTTPService struct {
	logger   logrus.FieldLogger
	services []signaling.Service

	//	rtmm *rtm.Manager
	//	mcum *mcu.Manager
}

// NewHTTPService creates a new APIv1 with the provided options.
func NewHTTPService(ctx context.Context, logger logrus.FieldLogger, services []signaling.Service) *HTTPService {
	return &HTTPService{
		logger:   logger,
		services: services,
	}
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (h *HTTPService) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	v1 := router.PathPrefix(URIPrefix).Subrouter()

	for _, service := range h.services {
		service.AddRoutes(ctx, v1, wrapper)
	}

	return router
}

// NumActive returns the number of the currently active connections at the
// accociated api..
func (h *HTTPService) NumActive() (active uint64) {
	for _, service := range h.services {
		active = active + service.NumActive()
	}

	return active
}

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
	"stash.kopano.io/kwm/kwmserver/signaling/admin"
	"stash.kopano.io/kwm/kwmserver/signaling/guest"
	"stash.kopano.io/kwm/kwmserver/signaling/mcu"
	"stash.kopano.io/kwm/kwmserver/signaling/rtm"
)

const (
	// URIPrefix defines the URL prefixed uses for API v1 requests.
	URIPrefix = "/api/kwm/v2"
	// WebsocketRouteIdentifier is the name of websocket route.
	WebsocketRouteIdentifier = "v2-rtm-websocket-by-key"
)

// HTTPService binds the HTTP router with handlers for kwm API v1.
type HTTPService struct {
	logger   logrus.FieldLogger
	services *signaling.Services
}

// NewHTTPService creates a new APIv2  with the provided options.
func NewHTTPService(ctx context.Context, logger logrus.FieldLogger, services *signaling.Services) *HTTPService {
	return &HTTPService{
		logger:   logger,
		services: services,
	}
}

// AddRoutes configures the services HTTP end point routing on the provided
// context and router.
func (h *HTTPService) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	v2 := router.PathPrefix(URIPrefix).Subrouter()

	if adminm, ok := h.services.AdminManager.(*admin.Manager); ok {
		r := v2.PathPrefix("/admin").Subrouter()
		adminm.AddRoutes(ctx, r, wrapper)
	}

	if mcum, ok := h.services.MCUManager.(*mcu.Manager); ok {
		r := v2.PathPrefix("/mcu").Subrouter()
		r.Handle("/websocket/{transaction}", wrapper(http.HandlerFunc(mcum.HTTPWebsocketHandler)))
		r.Handle("/websocket", wrapper(http.HandlerFunc(mcum.HTTPWebsocketHandler)))
	}

	if rtmm, ok := h.services.RTMManager.(*rtm.Manager); ok {
		r := v2.PathPrefix("/rtm").Subrouter()
		r.Handle("/connect", wrapper(rtmm.MakeHTTPConnectHandler(router, WebsocketRouteIdentifier)))
		r.Handle("/turn", wrapper(rtmm.MakeHTTPTURNHandler(router)))
		r.Handle("/websocket/{key}", wrapper(http.HandlerFunc(rtmm.HTTPWebsocketHandler))).Name(WebsocketRouteIdentifier)
	}

	if guestm, ok := h.services.GuestManager.(*guest.Manager); ok {
		r := v2.PathPrefix("/guest").Subrouter()
		r.Handle("/logon", wrapper(guestm.MakeHTTPLogonHandler()))
	}

	return router
}

// NumActive returns the number of the currently active connections at the
// accociated api..
func (h *HTTPService) NumActive() (active uint64) {
	for _, service := range h.services.Services() {
		active = active + service.NumActive()
	}

	return active
}

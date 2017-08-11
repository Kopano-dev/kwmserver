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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling"
	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	rtm "stash.kopano.io/kwm/kwmserver/signaling/api-v1/rtm"
)

const (
	// URIPrefix defines the URL prefixed uses for API v1 requests.
	URIPrefix = "/api/v1"
)

// HTTPService binds the HTTP router with handlers for kwm API v1.
type HTTPService struct {
	logger   logrus.FieldLogger
	services []signaling.Service

	rtmm *rtm.Manager
}

// NewHTTPService creates a new APIv1 with the provided options.
func NewHTTPService(ctx context.Context, logger logrus.FieldLogger, services []signaling.Service) *HTTPService {
	return &HTTPService{
		logger:   logger,
		services: services,

		rtmm: rtm.NewManager(ctx, "", logger),
	}
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (h *HTTPService) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(context.Context, http.Handler) http.Handler) http.Handler {
	v1 := router.PathPrefix(URIPrefix).Subrouter()
	for _, service := range h.services {
		service.AddRoutes(ctx, v1, wrapper)
	}
	v1.Handle("/rtm.connect", wrapper(ctx, http.HandlerFunc(h.RTMConnectHandler)))
	v1.Handle("/websocket/{key}", http.HandlerFunc(h.WebsocketHandler))

	return router
}

// RTMConnectHandler implements the HTTP handler for rtm.connect.
func (h *HTTPService) RTMConnectHandler(rw http.ResponseWriter, req *http.Request) {
	// TODO(longsleep): check authentication
	req.ParseForm()
	user := req.Form.Get("user")
	if user == "" {
		http.Error(rw, "missing user parameter", http.StatusBadRequest)
	}

	// create random URL to websocket endpoint
	key, err := h.rtmm.Connect(req.Context(), user)
	if err != nil {
		h.logger.Errorln("connect failed", err)
		http.Error(rw, "request failed", http.StatusInternalServerError)
		return
	}

	response := &api.RTMConnectResponse{
		ResponseOK: *api.ResponseOKValue,

		URL: fmt.Sprintf("%s/websocket/%s", URIPrefix, key),
		Self: &api.Self{
			ID:   user,
			Name: fmt.Sprintf("User %s", strings.ToUpper(user)),
		},
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// WebsocketHandler implements the HTTP handler for websocket requests.
func (h *HTTPService) WebsocketHandler(rw http.ResponseWriter, req *http.Request) {
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

	err := h.rtmm.HandleWebsocketConnect(req.Context(), key, rw, req)
	if err != nil {
		h.logger.WithError(err).Errorln("websocket connection failed")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

// NumActive returns the number of the currently active connections at the
// accociated api..
func (h *HTTPService) NumActive() int {
	return h.rtmm.NumActive()
}

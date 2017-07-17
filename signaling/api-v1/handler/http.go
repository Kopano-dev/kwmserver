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

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	apiManagers "stash.kopano.io/kwm/kwmserver/signaling/api-v1/managers"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	// APIv1URIPrefix defines the URL prefixed uses for API v1 requests.
	APIv1URIPrefix = "/api/v1"
)

// APIv1 binds the HTTP router with handlers for API version 1.
type APIv1 struct {
	logger logrus.FieldLogger

	rtm *apiManagers.RTMManager
}

// NewAPIv1 creates a new APIv1 with the provided options.
func NewAPIv1(logger logrus.FieldLogger) *APIv1 {
	return &APIv1{
		logger: logger,

		rtm: apiManagers.NewRTMManager("", logger),
	}
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (h *APIv1) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(context.Context, http.Handler) http.Handler) http.Handler {
	v1 := router.PathPrefix(APIv1URIPrefix).Subrouter()
	v1.Handle("/rtm.connect", wrapper(ctx, http.HandlerFunc(h.RTMConnectHandler)))
	v1.Handle("/websocket/{key}", http.HandlerFunc(h.WebsocketHandler))

	return router
}

// RTMConnectHandler implements the HTTP handler for rtm.connect.
func (h *APIv1) RTMConnectHandler(rw http.ResponseWriter, req *http.Request) {
	// TODO(longsleep): check authentication
	// create random URL to websocket endpoint
	key, err := h.rtm.Connect(req.Context())
	if err != nil {
		h.logger.Errorln("connect failed", err)
		http.Error(rw, "request failed", http.StatusInternalServerError)
		return
	}

	response := &api.RTMConnectResponse{
		ResponseOK: *api.ResponseOKValue,

		URL:  fmt.Sprintf("%s/websocket/%s", APIv1URIPrefix, key),
		Self: &api.Self{},
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(response)
}

// WebsocketHandler implements the HTTP handler for websocket requests.
func (h *APIv1) WebsocketHandler(rw http.ResponseWriter, req *http.Request) {
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

	err := h.rtm.HandleWebsocketConnect(req.Context(), key, rw, req)
	if err != nil {
		h.logger.Errorf("websocket connection failed", err)
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

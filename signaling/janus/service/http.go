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
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/admin"
	"stash.kopano.io/kwm/kwmserver/signaling/api-v1/mcu"
	"stash.kopano.io/kwm/kwmserver/signaling/janus"
	"stash.kopano.io/kwm/kwmserver/signaling/janus/plugins"
)

const (
	// URIPrefix defines the URL prefixed uses for Janus requests.
	URIPrefix = "/janus"
)

// HTTPService binds the HTTP router with handlers for Janus API.
type HTTPService struct {
	logger logrus.FieldLogger

	janus *janus.Manager
}

// NewHTTPService creates a new HTTP Janus API service with the provided options.
func NewHTTPService(ctx context.Context, logger logrus.FieldLogger, mcum *mcu.Manager, adminm *admin.Manager) *HTTPService {
	// create plugin factories
	factories := make(map[string]func(string, *janus.Manager) (janus.Plugin, error))

	if mcum != nil {
		// Forward all plugin calls to chromiumcu.
		_, factory := plugins.ChromiuMCUFactory(mcum)
		factories[""] = factory
	} else {
		id, factory := plugins.VideoCallFactory()
		factories[id] = factory
	}

	return &HTTPService{
		logger: logger,

		janus: janus.NewManager(ctx, "", logger, mcum, adminm, factories),
	}
}

func dumpRequest(req *http.Request) {
	requestDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println("janus dump err", err)
	}
	fmt.Println("janus request", string(requestDump))
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (h *HTTPService) AddRoutes(ctx context.Context, router *mux.Router, wrapper func(http.Handler) http.Handler) http.Handler {
	r := router.PathPrefix(URIPrefix).Subrouter()
	r.Handle("/admin", wrapper(http.HandlerFunc(h.adminHandler)))
	r.Handle("/websocket", wrapper(http.HandlerFunc(h.websocketHandler)))
	r.NotFoundHandler = http.HandlerFunc(h.DebugHandler)

	return router
}

func (h *HTTPService) adminHandler(rw http.ResponseWriter, req *http.Request) {
	//dumpRequest(req)

	err := h.janus.HandleAdmin(req.Context(), rw, req)
	if err != nil {
		h.logger.WithError(err).Errorln("admin request failed")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

func (h *HTTPService) websocketHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(rw, "", http.StatusMethodNotAllowed)
		return
	}

	err := h.janus.HandleWebsocketConnect(req.Context(), rw, req)
	if err != nil {
		h.logger.WithError(err).Errorln("websocket connection failed")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

// DebugHandler dumps the incoming requests data.
func (h *HTTPService) DebugHandler(rw http.ResponseWriter, req *http.Request) {
	dumpRequest(req)

	http.Error(rw, "not_implemented", http.StatusNotImplemented)
}

// NumActive returns the number of the currently active connections at the
// accociated api..
func (h *HTTPService) NumActive() uint64 {
	return h.janus.NumActive()
}

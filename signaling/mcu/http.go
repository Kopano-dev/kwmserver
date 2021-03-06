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

package mcu

import (
	"net/http"

	"github.com/gorilla/mux"
)

// HTTPWebsocketHandler implements the HTTP handler for websocket requests.
func (m *Manager) HTTPWebsocketHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(rw, "", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(req)
	transaction, _ := vars["transaction"]

	err := m.HandleWebsocketConnect(req.Context(), transaction, rw, req)
	if err != nil {
		m.logger.WithError(err).Errorln("websocket connection failed")
		http.Error(rw, "", http.StatusInternalServerError)
		return
	}
}

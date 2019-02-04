/*
 * Copyright 2019 Kopano and its licensors
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

package guest

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	kcoidc "stash.kopano.io/kc/libkcoidc"

	api "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
)

// Manager handles guests.
type Manager struct {
	id     string
	logger logrus.FieldLogger
	ctx    context.Context
}

// NewManager creates a new Manager with an id.
func NewManager(ctx context.Context, id string, logger logrus.FieldLogger) *Manager {
	m := &Manager{
		id:     id,
		logger: logger.WithField("manager", "guest"),
		ctx:    ctx,
	}

	return m
}

// Context Returns the Context of the associated manager.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// NumActive returns the number of the currently active connections at the
// accociated manager.
func (m *Manager) NumActive() uint64 {
	return 0
}

// ApplyRestrictions returns the guest claims from the provided claims.
func (m *Manager) ApplyRestrictions(auth *api.AdminAuthToken, claims *kcoidc.ExtraClaimsWithType) error {
	auth.GroupRestriction = make(map[string]bool)

	authorizedClaims := kcoidc.AuthorizedClaimsFromClaims(claims)
	if authorizedClaims == nil {
		return nil
	}

	if passthru, _ := authorizedClaims["passthru"].(map[string]interface{}); passthru != nil {
		if guestclaim, _ := passthru[guestClaim].(map[string]interface{}); guestclaim != nil {
			gc, err := newClaimsFromMap(guestclaim)
			if err != nil {
				return err
			}

			switch gc.Type {
			case guestTypeSimple:
				// FIXME(longsleep): This currently sets all incoming paths as group
				// restriction. In the future we might want to have other restricts
				// based on path patterns.
				auth.GroupRestriction[gc.Path] = true

			default:
				return errors.New("unknown guest type in guest claims")
			}

		}
	}

	return nil
}

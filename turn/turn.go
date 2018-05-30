/*
 * Copyright 2018 Kopano and its licensors
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

package turn

import (
	"context"
)

// DefaultServerTTL specifies the default time how long TURN credentials
// created by this module will be valid.
const DefaultServerTTL = 3600

// A Server is an interface to retrieve TURN server connectivity configuration.
type Server interface {
	GetConfig(ctx context.Context, username string) (*ClientConfig, error)
}

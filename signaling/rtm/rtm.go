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

package rtm

import (
	"time"
)

const (
	connectExpiration      = time.Duration(30) * time.Second
	connectCleanupInterval = time.Duration(1) * time.Minute
	connectKeySize         = 24
	connectionIDSize       = 24
	channelIDSize          = 24
	channelExpiration      = time.Duration(1) * time.Minute
	activeUserExpiration   = time.Duration(30) * time.Second

	// Buffer sizes.
	websocketReadBufferSize  = 1024
	websocketWriteBufferSize = 1024
)

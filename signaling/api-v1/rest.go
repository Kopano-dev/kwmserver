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

package api

// ResponseOK is the most basic response type with boolean OK flag.
type ResponseOK struct {
	OK bool `json:"ok"`
}

// ResponseOKValue is a response value with true OK status.
var ResponseOKValue = &ResponseOK{true}

// ResponseError is the most basic error response with error string.
type ResponseError struct {
	ResponseOK

	Error string `json:"error"`
}

// NewResponseError creates a new error response with the provided error.
func NewResponseError(s string) *ResponseError {
	return &ResponseError{
		Error: s,
	}
}

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

'use strict';

/**
 * @private
 */
export function makeAbsoluteURL(url: string): string {
	const a = document.createElement('a');
	a.href = url;
	return a. href;
}

/**
 * @private
 */
export function toHexString(byteArray: number[]) {
	return byteArray.map(byte => {
		return ('0' + (/* tslint:disable */byte & 0xFF/* tslint:enable */).toString(16)).slice(-2);
	}).join('');
}

/**
 * @private
 */
export function getRandomString(length?: number) {
	const bytes = new Uint8Array((length || 32) / 2);
	window.crypto.getRandomValues(bytes);
	return toHexString(Array.from(bytes));
}

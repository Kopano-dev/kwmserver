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

export interface IRTMConnectResponse {
	url?: string;
	ok: boolean;
	error: IRTMDataError;
}

export interface IRTMDataError {
	code: string;
	msg: string;
}

export interface IRTMTypeEnvelope {
	id: number;
	type: string;
}

export interface IRTMTypeEnvelopeReply extends IRTMTypeEnvelope {
	reply_to: number;
}

export interface IRTMTypeError extends IRTMTypeEnvelope {
	error: IRTMDataError;
}

export interface IRTMTypeSubTypeEnvelope extends IRTMTypeEnvelope {
	subtype: string;
}

export interface IRTMTypePingPong extends IRTMTypeEnvelope {
	ts: number;
	auth?: string;
}

export interface IRTMTypeWebRTC extends IRTMTypeSubTypeEnvelope {
	target: string;
	source: string;
	initiator: boolean;
	state: string;
	channel: string;
	hash: string;
	data: any;
}

export class RTMDataError {
	public code: string;
	public msg: string = '';

	constructor(data: IRTMDataError) {
		this.code = data.code;
		this.msg = data.msg;
	}
}

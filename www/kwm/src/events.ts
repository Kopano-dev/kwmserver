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

import { KWM } from './kwm';
import { PeerRecord } from './webrtc';

/**
 * @private
 */
class BaseEvent {
	public static eventName = 'BaseEvent';
	public static getName(): string {
		return this.eventName;
	}

	public target: any;

	constructor(target: any) {
		this.target = target;
	}
}

export class KWMStateChangedEvent extends BaseEvent {
	public static eventName = 'KWMStateChangedEvent';

	public connecting: boolean;
	public connected: boolean;

	constructor(target: any) {
		super(target);

		this.connecting = target.connecting;
		this.connected = target.connected;
	}
}

export class KWMErrorEvent extends BaseEvent {
	public static eventName = 'KWMErrorEvent';

	public code: string;
	public msg: string;

	constructor(target: any, details: any) {
		super(target);

		this.code = details.code;
		this.msg = details.msg;
	}
}

export class WebRTCPeerEvent extends BaseEvent {
	public static eventName = 'WebRTCPeerEvent';

	public event: string;
	public channel: string;
	public record: PeerRecord;
	public details: any;

	constructor(target: any, event: string, record: PeerRecord, details?: any) {
		super(target);
		this.event = event;
		this.record = record;
		this.details = details;
	}
}

export class WebRTCStreamEvent extends WebRTCPeerEvent {
	public static eventName = 'WebRTCStreamEvent';

	public stream: MediaStream;

	constructor(target: any, event: string, record: PeerRecord, stream: MediaStream) {
		super(target, event, record);
		this.stream = stream;
	}
}

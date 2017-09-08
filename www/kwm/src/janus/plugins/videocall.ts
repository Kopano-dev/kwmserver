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

import { KWM } from '../../kwm';

/**
 * Implementaton of a compatibility layer to emulate Janus VideoCall.
 *
 * @private
 */
export class VideoCall {
	public static getName(): string {
		return 'janus.plugin.videocall';
	}

	private kwm: KWM;

	private onerror?: () => void;
	private onmessage?: (msg: any, jsep: any) => void;
	private onremotestream?: (stream: MediaStream) => void;

	/**
	 * Creates VideoCall instance bound to the provided [[KWM]].
	 *
	 * @param kwm Reference to KWM instance.
	 * @param callbacks Janus callback function object.
	 */
	constructor(kwm: KWM, callbacks: any) {
		this.kwm = kwm;

		if (callbacks.error) {
			this.onerror = callbacks.error;
		}
		if (callbacks.onmessage) {
			this.onmessage = callbacks.onmessage;
		}
		if (callbacks.onremotestream) {
			this.onremotestream = callbacks.onremotestream;
		}

		kwm.webrtc.onpeer = event => {
			console.debug('onpeer', event);
			switch (event.event) {
				case 'incomingcall':
					this.dispatchMessage({
						channel: event.channel,
						event: 'incomingcall',
						record: event.record,
					});
					break;
				case 'outgoingcall':
					this.dispatchMessage({
						channel: event.channel,
						event: 'accepted',
						record: event.record,
					});
					break;
				case 'destroycall':
					this.dispatchMessage({
						channel: event.channel,
						event: 'hangup',
						record: event.record,
					});
					break;
			}
		};
		kwm.webrtc.onstream = event => {
			console.debug('onstream', event);
			if (this.onremotestream) {
				this.onremotestream(event.stream);
			}
		};
	}

	public send(data: any): void {
		console.debug('send', data);

		switch (data.message.request) {
			case 'register':
				this.kwm.connect(data.message.username).then(() => {
					console.log('connected');
					this.dispatchMessage({
						event: 'registered',
					});
				}).catch(() => {
					console.log('failed to connect');
				});
				break;
			case 'call':
				this.kwm.webrtc.doCall(data.message.username);
				break;
			case 'hangup':
				this.kwm.webrtc.doHangup();
				break;
			case 'accept':
				// NOTE(longsleep): We abuse jsep to pass the record which we want to accept.
				this.kwm.webrtc.doAnswer(data.jsep.user);
				break;

			default:
				throw new Error('not implemented message request: ' + data.message.request);
		}
	}

	public hangup() {
		console.debug('hangup');
		// NOTE(longsleep): This does nothing, send hangup request instead. I
		// know its stupid.
	}

	public createOffer(options: any) {
		console.log('createOffer');

		this.kwm.webrtc.setLocalStream(options.stream);
		options.success();
	}

	public createAnswer(options: any) {
		console.log('createAnswer');

		this.kwm.webrtc.setLocalStream(options.stream);
		options.success(options.jsep);
	}

	public handleRemoteJsep(jsep: any) {
		console.debug('handleRemoteJsep', jsep);
	}

	public detach() {
		console.debug('detach');

		this.kwm.webrtc.setLocalStream(undefined);
	}

	public unmuteVideo() {
		console.debug('unmuteVideo');

		this.kwm.webrtc.mute(true, true);
	}

	public muteVideo() {
		console.debug('muteVideo');

		this.kwm.webrtc.mute(true, false);
	}

	public unmuteAudio() {
		console.debug('unmuteAudio');

		this.kwm.webrtc.mute(false, true);
	}

	public muteAudio() {
		console.debug('muteAudio');

		this.kwm.webrtc.mute(false, false);
	}

	private dispatchMessage(message: any, async: boolean = true): void {
		if (async) {
			setTimeout(() => {
				this.dispatchMessage(message, false);
			}, 0);
			return;
		}

		if (this.onmessage) {
			// NOTE(longsleep): We always pass the record as second parameter.
			this.onmessage({result: message}, message.record);
		}
	}
}

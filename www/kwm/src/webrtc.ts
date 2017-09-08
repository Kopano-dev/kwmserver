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

import * as SimplePeer from 'simple-peer';
import { WebRTCPeerEvent, WebRTCStreamEvent } from './events';
import { KWM } from './kwm';
import { IRTMTypeEnvelope, IRTMTypeWebRTC } from './rtm';
import { getRandomString } from './utils';

/**
 * A PeerRecord represents a current or future peer connection with additional
 * meta data.
 */
export class PeerRecord {
	public hash: string;
	public initiator: boolean;
	public pc?: SimplePeer;
	public ref: string;
	public state: string;
	public user: string;
}

/**
 * A WebRTCManager bundles all WebRTC related client functionality and keeps
 * track of individual peer states via [[WebRTCManager]].
 */
export class WebRTCManager {
	/**
	 * WebRTC PeerConnection config for all connections created by
	 * [[WebRTCManager.getPeerConnection]]. Overwrite as needed.
	 */
	public config: any = {
		iceServers: [
			{url: 'stun:stun.l.google.com:19302'},
		],
	};
	/**
	 * Event handler for [[WebRTCPeerEvent]]. Set to a function to get called
	 * whenever [[WebRTCPeerEvent]]s are triggered.
	 */
	public onpeer?: (event: WebRTCPeerEvent) => void;
	/**
	 * Event handler for [[WebRTCStreamEvent]]. Set to a function to get called
	 * whenever [[WebRTCStreamEvent]]s are triggered.
	 */
	public onstream?: (event: WebRTCStreamEvent) => void;

	private kwm: KWM;

	private localStream?: MediaStream;
	private channel: string;
	private peers: Map<string, PeerRecord>;

	/**
	 * Creates WebRTCManager instance bound to the provided [[KWM]].
	 *
	 * @param kwm Reference to KWM instance.
	 */
	constructor(kwm: KWM) {
		this.kwm = kwm;
		this.peers = new Map<string, PeerRecord>();
	}

	/**
	 * Triggers a WebRTC call request via RTM to the provided user.
	 *
	 * @param user The User ID to call. Must not exist in the accociated
	 *        [[[[WebRTCManager]].
	 * @returns Promise providing the channel ID assigned to the new call.
	 */
	public async doCall(user: string): Promise<string> {
		console.log('webrtc doCall', user);

		if (this.channel) {
			throw new Error('already have a channel');
		}
		if (this.peers.has(user)) {
			throw new Error('peer already exists');
		}

		const record = new PeerRecord();
		record.initiator = true;
		record.user = user;
		record.state = getRandomString(12);
		this.peers.set(user, record);

		const event = new WebRTCPeerEvent(this, 'newcall', record);
		this.dispatchEvent(event);

		const reply = await this.sendWebrtc('webrtc_call', '', record, undefined, 5000) as IRTMTypeWebRTC;
		if (record !== this.peers.get(user)) {
			throw new Error('unknown or invalid peer');
		}
		if (record.hash) {
			throw new Error('record already has a hash');
		}
		record.hash = reply.hash;

		this.handleWebRTCMessage(reply);

		return this.channel;
	}

	/**
	 * Triggers a WebRTC call request via RTM to the provided user to answer
	 * and accept a previously received peer.
	 *
	 * @param user The User ID of the peer to answer. Must exist in the
	 *        accociated [[WebRTCManager.peers]].
	 * @returns Promise providing the channel ID assigned to the call.
	 */
	public async doAnswer(user: string): Promise<string> {
		console.log('webrtc doAnswer', user);

		if (!this.channel) {
			throw new Error('no channel');
		}

		const record = this.peers.get(user);
		if (!record) {
			throw new Error('no matching peer');
		}

		const event = new WebRTCPeerEvent(this, 'newcall', record);
		event.channel = this.channel;
		this.dispatchEvent(event);

		await this.sendWebrtc('webrtc_call', this.channel, record, {
			accept: true,
			state: record.ref,
		});

		return this.channel;
	}

	/**
	 * Triggers a WebRTC hangup request via RTM to the provided user ID. If no
	 * user ID is given all calls will be hung up and the accociated channel
	 * will be cleared.
	 *
	 * @param user The User ID of the peer to hangup. Must exist in the
	 *        accociated [[WebRTCManager.peers]]. If empty, all known peers will
	 *        be sent hangup requests.
	 * @returns Promise providing the accociated channel ID.
	 */
	public async doHangup(user: string = '', reason: string = 'hangup'): Promise<string> {
		console.log('webrtc doHangup', user);

		const channel = this.channel;
		if (!user) {
			// Hangup all.
			this.channel = '';
			this.peers.forEach((record: PeerRecord, key: string, peers: Map<string, PeerRecord>) => {
				this.sendHangup(channel, record, reason);
			});
		} else {
			const record = this.peers.get(user);
			if (!record) {
				throw new Error('unknown peer');
			}
			this.sendHangup(this.channel, record, reason);
		}

		return channel;
	}

	/**
	 * Set the local media stream. That stream will be attached to all new
	 * Peers which are created afterwards.
	 *
	 * @param stream MediaStream object. Do not provie this parameter to no
	 *        longer use a local stream.
	 */
	public setLocalStream(stream?: MediaStream): void {
		this.localStream = stream;
	}

	/**
	 * Process incoming KWM RTM API WebRTC related payload data.
	 *
	 * @private
	 * @param message Payload message.
	 */
	public handleWebRTCMessage(message: IRTMTypeWebRTC): void {
		console.debug('<<< webrtc', message);
		let record: PeerRecord;

		switch (message.subtype) {
			case 'webrtc_call':
				if (message.initiator) {
					if (!message.source) {
						console.log('webrtc incoming call without source');
						return;
					}

					record = this.peers.get(message.source) as PeerRecord;
					if (record) {
						if (!message.target) {
							// Silent clear incoming call, call was taken by other connection.
							setTimeout(() => {
								this.sendHangup(message.channel, record, '');
							}, 0);
							return;
						}

						throw new Error('already have that peer: ' + message.source);
					}

					// Incoming call.
					record = new PeerRecord();
					record.user = message.source;
					record.state = getRandomString(12);
					record.ref = message.state;
					record.hash = message.hash;

					if (this.channel) {
						// busy
						console.log('webrtc incoming call while already have a call');
						this.sendWebrtc('webrtc_call', message.channel, record, {
							accept: false,
							reason: 'reject_busy',
							state: record.ref,
						});
						return;
					}
					if (this.channel && this.channel !== message.channel) {
						console.log('webrtc incoming call with wrong channel', this.channel);
						return;
					}

					this.channel = message.channel;
					this.peers.set(message.source, record);

					const event = new WebRTCPeerEvent(this, 'incomingcall', record);
					event.channel = message.channel;
					this.dispatchEvent(event, true);

				} else {
					// check and start webrtc.
					record = this.peers.get(message.source) as PeerRecord;
					if (!record) {
						console.log('webrtc unknown peer', message.source);
						return;
					}
					if (record.state !== message.data.state) {
						console.log('webbrtc peer data with wrong state', record.state);
						return;
					}
					if (record.hash !== message.hash) {
						console.log('webrtc peer data with wrong hash', record.hash);
						return;
					}
					if (!message.data.accept) {
						console.log('webrtc peer did not accept call', message);
						const abortEvent = new WebRTCPeerEvent(this, 'abortcall', record, message.data.reason || 'no reason given');
						abortEvent.channel = this.channel;
						this.dispatchEvent(abortEvent, true);
						return;
					}

					record.ref = message.state;
					console.log('start webrtc, accept call reply');

					const pc = this.getPeerConnection(true, record);
					console.debug('created pc', pc);

					const event = new WebRTCPeerEvent(this, 'outgoingcall', record);
					event.channel = this.channel;
					this.dispatchEvent(event, true);
				}
				break;

			case 'webrtc_channel':
				if (this.channel) {
					console.log('webrtc channel when already have one', this.channel, message.channel);
					return;
				}

				this.channel = message.channel;
				break;

			case 'webrtc_hangup':
				if (!message.channel || this.channel !== message.channel) {
					console.log('webrtc hangup with wrong channel', this.channel, message.channel);
					return;
				}
				if (!message.data) {
					console.log('webrtc hangup data empty');
					return;
				}

				record = this.peers.get(message.source) as PeerRecord;
				if (!record) {
					console.log('webrtc hangup for unknown peer');
					return;
				}
				if (record.ref !== message.state && record.ref) {
					console.log('webrtc hangup with wrong state', record.ref);
					return;
				}
				this.sendHangup(this.channel, record, '');

				break;

			case 'webrtc_signal':
				if (!message.channel || this.channel !== message.channel) {
					console.log('webrtc signal with wrong channel', this.channel, message.channel);
					return;
				}
				if (!message.data) {
					console.log('webrtc signal data empty');
					return;
				}

				record = this.peers.get(message.source) as PeerRecord;
				if (!record) {
					console.log('webrtc signal for unknown peer');
					return;
				}
				if (record.ref !== message.state && record.ref) {
					console.log('webrtc signal with wrong state', record.ref);
					return;
				}

				if (!record.pc) {
					console.log('start webrtc, received signal');
					const pc = this.getPeerConnection(false, record);
					console.debug('created pc', pc);
					record.pc = pc;
				}
				record.pc.signal(message.data);

				break;
		}
	}

	private async sendHangup(channel: string, record: PeerRecord, reason: string = 'hangup'): Promise<boolean> {
		this.peers.delete(record.user);
		if (record.pc) {
			record.pc.destroy();
			record.pc = undefined;
		}

		const event = new WebRTCPeerEvent(this, 'destroycall', record);
		event.channel = channel;
		this.dispatchEvent(event);

		if (reason) {
			return this.sendWebrtc('webrtc_hangup', channel, record, {
				accept: false,
				reason,
				state: record.ref,
			}).then(() => {
				return Promise.resolve(true);
			});
		} else {
			return Promise.resolve(true);
		}
	}

	private async sendWebrtc(
		subtype: string, channel: string, record: PeerRecord,
		data?: any, replyTimeout: number = 0): Promise<IRTMTypeEnvelope> {
		const payload = {
			channel,
			data,
			hash: record.hash,
			id: 0,
			initiator: !!record.initiator,
			state: record.state,
			subtype,
			target: record.user,
			type: 'webrtc',
		};

		return this.kwm.sendWebSocketPayload(payload, replyTimeout = replyTimeout);
	}

	private getPeerConnection(initiator: boolean, record: PeerRecord): SimplePeer {
		const pc = new SimplePeer({
			config: this.config,
			initiator,
			stream: this.localStream,
			trickle: true,
		});
		pc.on('error', err => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('peerconnection error', err);
			this.dispatchEvent(new WebRTCPeerEvent(this, 'pc.error', record, err));
		});
		pc.on('signal', data => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('peerconnection signal', data);
			const payload = {
				channel: this.channel,
				data,
				hash: record.hash,
				id: 0,
				state: record.state,
				subtype: 'webrtc_signal',
				target: record.user,
				type: 'webrtc',
			};
			console.debug('>>> send signal', payload);
			this.kwm.sendWebSocketPayload(payload);
		});
		pc.on('connect', () => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('peerconnection connect');
			this.dispatchEvent(new WebRTCPeerEvent(this, 'pc.connect', record, pc));
		});
		pc.on('close', () => {
			if (pc !== record.pc) {
				return;
			}

			console.log('peerconnection close');
			this.dispatchEvent(new WebRTCPeerEvent(this, 'pc.closed', record, pc));
			record.pc = undefined;
		});
		pc.on('stream', mediaStream => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('peerconnection stream', mediaStream);
			this.dispatchEvent(new WebRTCStreamEvent(this, 'pc.stream', record, mediaStream));
		});
		pc.on('iceStateChange', state => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('iceStateChange', state);
			this.dispatchEvent(new WebRTCPeerEvent(this, 'pc.iceStateChange', record, state));
		});
		pc.on('signalingStateChange', state => {
			if (pc !== record.pc) {
				return;
			}

			console.debug('signalingStateChange', state);
			this.dispatchEvent(new WebRTCPeerEvent(this, 'pc.signalingStateChange', record, state));
		});

		record.pc = pc;
		return pc;
	}

	/**
	 * Generic event dispatcher. Dispatches callback functions based on event
	 * types. Throws error for unknown event types. If a known event type has no
	 * event handler registered, dispatchEvent does nothing.
	 *
	 * @param event Event to be dispatched.
	 * @param async Boolean value if the event should trigger asynchronously.
	 */
	private dispatchEvent(event: any, async?: boolean): void {
		if (async) {
			setTimeout(() => {
				this.dispatchEvent(event, false);
			}, 0);
			return;
		}

		switch (event.constructor.getName()) {
			case WebRTCPeerEvent.getName():
				if (this.onpeer) {
					this.onpeer(event);
				}
				break;
			case WebRTCStreamEvent.getName():
				if (this.onstream) {
					this.onstream(event);
				}
				break;
			default:
				throw new Error('unknown event: ' + event.constructor.getName());
		}
	}
}

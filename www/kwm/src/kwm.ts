/*!
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

import { KWMErrorEvent, KWMStateChangedEvent } from './events';
import { Plugins } from './plugins';
import { IRTMConnectResponse, IRTMDataError, IRTMTypeEnvelope, IRTMTypeEnvelopeReply, IRTMTypeError,
	IRTMTypeWebRTC, RTMDataError } from './rtm';
import { getRandomString, makeAbsoluteURL } from './utils';
import { WebRTCManager } from './webrtc';

/**
 * The sequence counter for sent websocket message payloads. It is automatically
 * incremented whenever a payload message is sent via [[KWM.sendWebSocketPayload]].
 * @private
 */
let websocketSequence = 0;

/**
 * IReplyTimeoutRecord is an interface to hold registered reply timeouts with a
 * resolve function.
 */
interface IReplyTimeoutRecord {
	resolve: (message: IRTMTypeEnvelope) => void;
	timeout: number;
}

/**
 * KWMInit is a helper constructor to create KWM interface with settings and
 * callbacks.
 */
class KWMInit {
	public static options: any = {};

	public static init(options: any) {
		this.options = options;
	}

	constructor(callbacks: any = {}) {
		let url = callbacks.server || '';
		if (url) {
			const urlParser = document.createElement('a');
			urlParser.href = url;
			if (urlParser.protocol === 'wss:' || urlParser.protocol === 'ws:') {
				// Convert Websocket URLs to HTTP/HTTPS.
				urlParser.protocol = 'http' + urlParser.protocol.substr(2);
				url = urlParser.href;
			}
		}

		const kwm = new KWM(url);
		kwm.webrtc.config = {
			iceServers: callbacks.iceServers || [],
		};

		if (callbacks.success) {
			setTimeout(() => {
				callbacks.success();
			}, 0);
		}
		return kwm;
	}
}

/**
 * KWM is the main Kopano Web Meetings Javascript library entry point. It holds
 * the status and connections to KWM.
 */
export class KWM {
	/**
	 * Alternative constructor which provides asynchrous callbacks.
	 */
	public static KWMInit: KWMInit = KWMInit;

	/**
	 * Boolean flag wether KWM is currently trying to establish a connection.
	 */
	public connecting: boolean = false;

	/**
	 * Boolean flag wether KWM is currently connected or not.
	 */
	public connected: boolean = false;

	/**
	 * Event handler for [[KWMStateChangedEvent]]. Set to a function to get called
	 * whenever [[KWMStateChangedEvent]]s are triggered.
	 */
	public onstatechanged?: (event: KWMStateChangedEvent) => void;
	/**
	 * Event handler for [[KWMErrorEvent]]. Set to a function to get called
	 * whenever [[KWMErrorEvent]]s are triggered.
	 */
	public onerror?: (event: KWMErrorEvent ) => void;

	/**
	 * Reference to WebRTC related functionality in KWM.
	 */
	public webrtc: WebRTCManager;

	private baseURI: string;
	private socket?: WebSocket;
	private replyHandlers: Map<number, IReplyTimeoutRecord>;

	/**
	 * Creates KWM instance with the provided parameters.
	 *
	 * @param baseURI The base URI to the KWM server API.
	 */
	constructor(baseURI: string = '') {
		console.log('new KWM', this, baseURI);

		this.webrtc = new WebRTCManager(this);

		this.baseURI = baseURI.replace(/\/$/, '');
		this.replyHandlers = new Map<number, IReplyTimeoutRecord>();
	}

	/**
	 * Allows attaching plugins with callbacks to the accociated [[KWM]] instance.
	 *
	 * @param callbacks Object with callbacks.
	 */
	public attach(callbacks: any = {}): void {
		let plugin: any;
		let err: any;

		const pluginFactory = Plugins.get(callbacks.plugin);
		if (pluginFactory) {
			plugin = new pluginFactory(this, callbacks);
		} else {
			err = new Error('unknown plugin: ' + callbacks.plugin);
		}

		if (err) {
			if (callbacks.error) {
				setTimeout(() => {
					callbacks.error(err);
				}, 0);
				return;
			}

			throw err;
		}

		if (callbacks.success) {
			setTimeout(() => {
				callbacks.success(plugin);
			}, 0);
		}
	}

	/**
	 * Global destruction of all accociated resources.
	 *
	 * @param callbacks Object with callbacks.
	 */
	public destroy(callbacks: any = {}): void {
		this.webrtc.doHangup().then(() => {
			if (this.socket) {
				this.socket.close();
			}

			if (callbacks.success) {
				setTimeout(() => {
					callbacks.success();
				}, 0);
			}
		}).catch((reason: any) => {
			const err = new Error('failed to destroy: ' + reason);
			if (callbacks.error) {
				setTimeout(() => {
					callbacks.error(err);
				}, 0);
				return;
			}

			throw err;
		});
	}

	/**
	 * Establish Websocket connection to KWM server as the provided user.
	 *
	 * @param user The user ID.
	 * @returns Promise which resolves when the connection was established.
	 */
	public async connect(user: string): Promise<void> {
		console.log('KWM connect', user);

		const connectResult = await this.rtmConnect(user);
		console.debug('connect result', connectResult);
		if (!connectResult.ok) {
			if (connectResult.error) {
				throw new RTMDataError(connectResult.error);
			}
			throw new RTMDataError({code: 'unknown_error', msg: ''});
		}

		let url = connectResult.url;
		if (!url.includes('://')) {
			// Prefix with base when not absolute already.
			url = this.baseURI + url;
		}
		this.createWebSocket(url);
	}

	/**
	 * Encode and send JSON payload data via [[KWM.socket]] connection.
	 *
	 * @private
	 * @param payload The payload data.
	 * @param replyTimeout Timeout in milliseconds for reply callback. If 0,
	 *        then no callback is expected and none is registered.
	 * @returns Promise which resolves when the reply was received or immediately
	 *          when no timeout was given.
	 */
	public async sendWebSocketPayload(payload: IRTMTypeEnvelope, replyTimeout: number = 0): Promise<IRTMTypeEnvelope> {
		return new Promise<IRTMTypeEnvelope>((resolve, reject) => {
			if (!this.connected || !this.socket) {
				reject(new Error('no_connection'));
				return;
			}

			payload.id = ++websocketSequence;
			try {
				this.socket.send(JSON.stringify(payload));
			} catch (err) {
				reject(err);
				return;
			}
			if (replyTimeout > 0) {
				const timeout = window.setTimeout(() => {
					reject(new Error('timeout'));
				}, replyTimeout);
				this.replyHandlers.set(payload.id, {resolve, timeout});
			} else {
				setTimeout(resolve, 0);
			}
		});
	}

	/**
	 * Dispatch a new [[KWMStateChangedEvent]].
	 * @private
	 */
	public dispatchStateChangedEvent(): void {
		this.dispatchEvent(new KWMStateChangedEvent(this));
	}

	/**
	 * Dispatch a new [[KWMErrorEvent]] with the provided error details.
	 * @private
	 */
	public dispatchErrorEvent(err: IRTMDataError): void {
		this.dispatchEvent(new KWMErrorEvent(this, err));
	}

	/**
	 * Call KWM RTM rtm.connect via REST to retrieve Websocket endpoint details.
	 *
	 * @param user The user ID.
	 * @returns Promise with the unmarshalled response data once received.
	 */
	private async rtmConnect(user: string): Promise<IRTMConnectResponse> {
		console.log('KWM rtmConnect');

		const url = this.baseURI + '/api/v1/rtm.connect';
		const params = new URLSearchParams();
		params.set('user', user);

		return fetch(url, {
			body: params,
			method: 'POST',
			mode: 'cors',
		}).then(response => {
			if (!response.ok) {
				return {
					error: {
						code: 'http_error_' + response.status,
						msg: response.statusText,
					},
					ok: false,
				};
			}

			return response.json();
		});
	}

	/**
	 * Create a new KWM RTM Websocket connection using the provided uri. If
	 * the accociated KWM instance already has a connection, the old connection
	 * will be closed before the new connection is established.
	 *
	 * @param uri URI or URL to use. The value will be made absolute if not
	 *        already absolute. The scheme will be transformed to `ws:` or `wss:`
	 *        if `http:` or `https:`.
	 */
	private createWebSocket(uri: string): void {
		console.debug('create websocket', uri);

		if (this.socket) {
			console.log('closing existing socket');
			this.socket.close();
			this.socket = undefined;
			this.connected = false;
		}
		this.connecting = true;
		this.dispatchStateChangedEvent();

		const url = makeAbsoluteURL(uri).replace(/^https:\/\//i, 'wss://').replace(/^http:\/\//i, 'ws://');
		console.log('connecting socket URL', url);
		const socket = new WebSocket(url);
		socket.onopen = (event: Event) => {
			if (event.target !== this.socket) {
				return;
			}
			console.log('socket connected', event);
			this.connected = true;
			this.connecting = false;
			this.dispatchStateChangedEvent();
			this.socket.onmessage = this.handleWebSocketMessage.bind(this);
		};
		socket.onclose = (event: CloseEvent) => {
			if (event.target !== this.socket) {
				return;
			}
			console.log('socket closed', event);
			this.socket = undefined;
			this.connected = false;
			this.connecting = false;
			this.dispatchStateChangedEvent();
		};
		socket.onerror = (event: Event) => {
			if (event.target !== this.socket) {
				return;
			}
			console.log('socket error', event);
			this.socket = undefined;
			this.connected = false;
			this.connecting = false;
			this.dispatchErrorEvent({
				code: 'websocket_error',
				msg: '' + event,
			});
			this.dispatchStateChangedEvent();
		};

		this.socket = socket;
	}

	/**
	 * Process incoming KWM RTM API Websocket payload data.
	 *
	 * @param event Websocket event holding payload data.
	 */
	private handleWebSocketMessage(event: MessageEvent): void {
		if (event.target !== this.socket) {
			(event.target as WebSocket).close();
			return;
		}

		console.debug('socket message', event);
		const message: IRTMTypeEnvelope = JSON.parse(event.data);
		const reply = message as IRTMTypeEnvelopeReply;
		if (reply.reply_to) {
			const replyTimeout = this.replyHandlers.get(reply.reply_to);
			if (replyTimeout) {
				this.replyHandlers.delete(reply.reply_to);
				clearTimeout(replyTimeout.timeout);
				replyTimeout.resolve(message);
			} else {
				console.log('received reply without handler', reply);
			}
			return;
		}

		switch (message.type) {
			case 'hello':
				console.log('server said hello', message);
				break;
			case 'goodbye':
				console.log('server said goodbye, close connection', message);
				this.socket.close();
				this.socket = undefined;
				this.connected = false;
				break;
			case 'webrtc':
				this.webrtc.handleWebRTCMessage(message as IRTMTypeWebRTC);
				break;
			case 'error':
				console.log('server said error', message);
				this.dispatchErrorEvent((message as IRTMTypeError).error);
				break;
			default:
				console.debug('unknown type', message.type, message);
				break;
		}
	}

	/**
	 * Generic event dispatcher. Dispatches callback functions based on event
	 * types. Throws error for unknown event types. If a known event type has no
	 * event handler registered, dispatchEvent does nothing.
	 *
	 * @param event Event to be dispatched.
	 */
	private dispatchEvent(event: any): void {
		switch (event.constructor.getName()) {
			case KWMStateChangedEvent.getName():
				if (this.onstatechanged) {
					this.onstatechanged(event);
				}
				break;
			case KWMErrorEvent.getName():
				if (this.onerror) {
					this.onerror(event);
				}
				break;
			default:
				throw new Error('unknown event: ' + event.constructor.getName());
		}
	}
}

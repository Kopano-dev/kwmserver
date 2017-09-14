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
	IRTMTypePingPong, IRTMTypeWebRTC, RTMDataError } from './rtm';
import { getRandomString, makeAbsoluteURL } from './utils';
import { WebRTCManager } from './webrtc';

/**
 * The sequence counter for sent websocket message payloads. It is automatically
 * incremented whenever a payload message is sent via [[KWM.sendWebSocketPayload]].
 * @private
 */
let websocketSequence = 0;

const authorizationTypeToken = 'Token';
const authorizationTypeBearer = 'Bearer';

interface IKWMOptions {
	authorizationType?: string;
	authorizationValue?: string;
}

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
	public static options: any = {
		connectTimeout: 5000,
		heartbeatInterval: 5000,
		maxReconnectInterval: 30000,
		reconnectEnabled: true,
		reconnectFactor: 1.5,
		reconnectInterval: 1000,
		reconnectSpreader: 500,
	};

	public static init(options: any) {
		Object.assign(this.options, options);
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

		const options: IKWMOptions = {};
		if (callbacks.token) {
			options.authorizationType = authorizationTypeToken;
			options.authorizationValue = callbacks.token;
		}

		const kwm = new KWM(url, options);
		kwm.webrtc.config = {
			iceServers: callbacks.iceServers || [],
		};

		if (callbacks.success) {
			setTimeout(() => {
				callbacks.success();
			}, 0);
		}
		if (callbacks.error) {
			kwm.onerror = event => {
				callbacks.error(event);
			};
		}

		return kwm;
	}
}

/**
 * KWM is the main Kopano Web Meetings Javascript library entry point. It holds
 * the status and connections to KWM.
 */
export class KWM {
	public static version: string = __VERSION__;

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
	 * Boolean flag wether KWM is automatically reconnecting or not.
	 */
	public reconnecting: boolean = false;

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
	private options: IKWMOptions;
	private socket?: WebSocket;
	private closing: boolean = false;
	private reconnector: number;
	private heartbeater: number;
	private latency: number = 0;
	private reconnectAttempts: number = 0;
	private replyHandlers: Map<number, IReplyTimeoutRecord>;

	/**
	 * Creates KWM instance with the provided parameters.
	 *
	 * @param baseURI The base URI to the KWM server API.
	 * @param options Additional options.
	 */
	constructor(baseURI: string = '', options?: IKWMOptions) {
		this.webrtc = new WebRTCManager(this);

		this.baseURI = baseURI.replace(/\/$/, '');
		this.options = options || {};
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
		this.reconnecting = false;
		this.webrtc.doHangup().then(() => {
			if (this.socket) {
				this.closeWebsocket(this.socket);
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
		console.debug('KWM connect', user);

		clearTimeout(this.reconnector);
		clearTimeout(this.heartbeater);
		const reconnector = (fast: boolean = false): void => {
			clearTimeout(this.reconnector);
			if (!this.reconnecting) {
				return;
			}
			let reconnectTimeout = KWMInit.options.reconnectInterval;
			if (!fast) {
				reconnectTimeout *= Math.trunc(Math.pow(KWMInit.options.reconnectFactor, this.reconnectAttempts));
				if (reconnectTimeout > KWMInit.options.maxReconnectInterval) {
					reconnectTimeout = KWMInit.options.maxReconnectInterval;
				}
				reconnectTimeout += Math.floor(Math.random() * KWMInit.options.reconnectSpreader);
			}
			this.reconnector = window.setTimeout(() => {
				this.connect(user);
			}, reconnectTimeout);
			this.reconnectAttempts++;
		};
		const latencyMeter: number[] = [];
		const heartbeater = (init: boolean = false): void => {
			clearTimeout(this.heartbeater);
			if (!this.connected || this.closing) {
				return;
			}
			this.heartbeater = window.setTimeout(() => {
				heartbeater();
			}, KWMInit.options.heartbeatInterval);
			if (init) {
				return;
			}

			const payload: IRTMTypePingPong = {
				id: 0,
				ts: new Date().getTime(),
				type: 'ping',
			};
			const replyTimeout = KWMInit.options.heartbeatInterval / 100 * 90 ;
			const socket = this.socket;
			this.sendWebSocketPayload(payload, replyTimeout).then((message: IRTMTypePingPong) => {
				if (message.type !== 'pong') {
					// Ignore unknow stuff.
					return;
				}
				let latency = (new Date().getTime()) - message.ts;
				latencyMeter.push(latency);
				if (latencyMeter.length > 10) {
					latencyMeter.shift();
				}
				latency = latencyMeter.reduce((a, b) => {
					return a + b;
				});
				latency = Math.floor(latency / latencyMeter.length);
				if (socket === this.socket && latency !== this.latency) {
					this.latency = latency;
				}
				if (message.auth && this.options.authorizationType) {
					this.options.authorizationValue = message.auth;
				}
			}).catch(err => {
				if (socket && this.socket === socket) {
					console.warn('heartbeat failed', err);
					// NOTE(longsleep): Close the socket asynchronously and directly trigger a
					// close event. This avoids issues where the socket is in a state which
					// cannot be closed yet.
					setTimeout(() => {
						this.closeWebsocket(socket);
					}, 0);
					const event = new CloseEvent('close', {
						reason: 'client heartbeat timeout',
					});
					socket.dispatchEvent(event);
				}
			});
		};

		this.reconnecting = KWMInit.options.reconnectEnabled;
		this.connecting = true;
		this.dispatchStateChangedEvent();

		return new Promise<void>(async (resolve, reject) => {
			let connectResult: IRTMConnectResponse;
			let authorizationHeader: string = '';
			if (this.options.authorizationType && this.options.authorizationValue) {
				authorizationHeader = this.options.authorizationType + ' ' + this.options.authorizationValue;
			}
			try {
				connectResult = await this.rtmConnect(user, authorizationHeader);
			} catch (err) {
				console.warn('failed to fetch connection details', err);
				connectResult = {
					error: {
						code: 'request_failed',
						msg: '' + err,
					},
					ok: false,
				};
			}
			// console.debug('connect result', connectResult);
			if (!connectResult.ok || !connectResult.url) {
				this.connecting = false;
				this.dispatchStateChangedEvent();
				if (this.reconnecting) {
					if (connectResult.error && connectResult.error.code === 'http_error_403') {
						console.warn('giving up reconnect, as connect returned forbidden', connectResult.error.msg);
						this.reconnecting = false;
						this.dispatchStateChangedEvent();
						this.dispatchErrorEvent(connectResult.error);
					}
					reconnector();
				} else if (connectResult.error) {
					reject(new RTMDataError(connectResult.error));
				} else {
					reject(new RTMDataError({code: 'unknown_error', msg: ''}));
				}
				return;
			}

			let url = connectResult.url;
			if (!url.includes('://')) {
				// Prefix with base when not absolute already.
				url = this.baseURI + url;
			}
			const start = new Date().getTime();
			this.createWebSocket(url, this.reconnecting ? reconnector : undefined).then(() => {
				this.reconnectAttempts = 0;
				this.latency = (new Date().getTime()) - start;
				latencyMeter.push(this.latency);
				console.debug('connection established', this.reconnectAttempts, this.latency);
				heartbeater(true);
				resolve();
			}, err => {
				console.warn('connection failed', err, !!this.reconnecting);
				if (this.reconnecting) {
					reconnector();
				} else {
					reject(err);
				}
			});
		});
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
			if (!this.connected || !this.socket || this.closing) {
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
	 * @param authorizataionHeader Authorization HTTP request header value.
	 * @returns Promise with the unmarshalled response data once received.
	 */
	private async rtmConnect(user: string, authorizationHeader?: string): Promise<IRTMConnectResponse> {
		const url = this.baseURI + '/api/v1/rtm.connect';
		const headers = new Headers();
		if (authorizationHeader) {
			headers.set('Authorization', authorizationHeader);
		}
		const params = new URLSearchParams();
		params.set('user', user);

		return fetch(url, {
			body: params,
			headers,
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
	private async createWebSocket(uri: string, reconnector?: (fast?: boolean) => void): Promise<WebSocket> {
		console.debug('create websocket', uri);

		return new Promise<WebSocket>((resolve, reject) => {
			if (this.socket) {
				console.warn('closing existing socket connection');
				const oldSocket = this.socket;
				this.socket = undefined;
				this.connected = false;
				this.closeWebsocket(oldSocket);
			}

			const url = makeAbsoluteURL(uri).replace(/^https:\/\//i, 'wss://').replace(/^http:\/\//i, 'ws://');
			console.debug('connecting socket URL', url);
			const socket = new WebSocket(url);

			let isTimeout = false;
			const timeout = setTimeout(() => {
				isTimeout = true;
				if (socket === this.socket) {
					this.socket = undefined;
					this.connected = false;
					this.connecting = false;
					this.dispatchStateChangedEvent();
				}
				setTimeout(() => {
					this.closeWebsocket(socket);
				}, 0);
				reject(new Error('connect_timeout'));
			}, KWMInit.options.connectTimeout);

			socket.onopen = (event: Event) => {
				clearTimeout(timeout);
				if (isTimeout) {
					return;
				}
				setTimeout(() => {
					resolve(event.target as WebSocket);
				}, 0);
				if (event.target !== this.socket) {
					return;
				}
				console.debug('socket connected', event);
				this.connected = true;
				this.connecting = false;
				this.dispatchStateChangedEvent();
				this.socket.onmessage = this.handleWebSocketMessage.bind(this);
			};
			socket.onclose = (event: CloseEvent) => {
				clearTimeout(timeout);
				if (isTimeout) {
					return;
				}
				if (event.target !== this.socket) {
					if (!this.socket && !this.connecting && reconnector) {
						console.debug('socket closed, retry immediate reconnect now', event);
						// Directly try to reconnect. This makes reconnects fast
						// in the case where the connection was lost on the client
						// and has come back.
						reconnector(true);
					}
					return;
				}
				console.debug('socket closed', event);
				this.socket = undefined;
				this.closing = false;
				this.connected = false;
				this.connecting = false;
				this.dispatchStateChangedEvent();
				if (reconnector) {
					reconnector();
				}
			};
			socket.onerror = (event: Event) => {
				clearTimeout(timeout);
				if (isTimeout) {
					return;
				}
				setTimeout(() => {
					reject(event);
				}, 0);
				if (event.target !== this.socket) {
					return;
				}
				console.debug('socket error', event);
				this.socket = undefined;
				this.connected = false;
				this.connecting = false;
				this.dispatchErrorEvent({
					code: 'websocket_error',
					msg: '' + event,
				});
				this.dispatchStateChangedEvent();
			};

			this.closing = false;
			this.socket = socket;
		});
	}

	/**
	 * Closes the provided websocket connection.
	 *
	 * @param socket Websocket to close.
	 */
	private closeWebsocket(socket: WebSocket): void {
		if (socket === this.socket) {
			this.closing = true;
		}
		socket.close();
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

		// console.debug('socket message', event);
		const message: IRTMTypeEnvelope = JSON.parse(event.data);
		const reply = message as IRTMTypeEnvelopeReply;
		if (reply.type === 'pong') {
			// Special case for pongs, which just reply back everything.
			reply.reply_to = reply.id;
		}
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
				console.debug('server hello', message);
				break;
			case 'goodbye':
				console.debug('server goodbye, close connection', message);
				this.reconnectAttempts = 1; // NOTE(longsleep): avoid instant reconnect.
				this.closeWebsocket(this.socket);
				this.connected = false;
				break;
			case 'webrtc':
				this.webrtc.handleWebRTCMessage(message as IRTMTypeWebRTC);
				break;
			case 'error':
				console.warn('server error', message);
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

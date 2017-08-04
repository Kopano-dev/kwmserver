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

import { JanusVideoCall } from './plugins/janus.js';

export class ChromiuMCU {
	constructor(uri='ws://localhost:8778/api/v1/mcu/websocket', handler=null, options=null) {
		console.debug('new chromiumcu', this);

		this.uri = uri;
		this.handler = handler;

		this.ws = null;
		this.reconnectEnabled = handler === null ? true : false;
		this.reconnectInterval = 1000;
		this.maxReconnectInterval = 30000;
		this.reconnectFactor = 1.5;
		this.connectTimeout = 5000;
		this.reconnectAttempts = 0;

		this.plugins = {};

		if (!options) {
			options = {};
		}

		[JanusVideoCall].forEach(plugin => {
			this.plugins[plugin.plugin] = plugin;
			console.info('registered plugin', plugin.plugin);
		});

		Object.entries(options).forEach(([key, value]) => {
			this[key] = value;
		});
	}

	autostart(query='', hash='', reconnect=true) {
		return new Promise((resolve, reject) => {
			const queryParams = decodeParams(query);
			const hashParams = decodeParams(hash);
			console.info('chromiumcu autostart', queryParams, hashParams);

			if (queryParams.uri) {
				this.uri = queryParams.uri;
			}
			if (hashParams.plugin) {
				console.log('chromiumcu launching plugin', hashParams.plugin);
				let plugin = this.plugins[hashParams.plugin];
				if (!plugin) {
					reject(new Error('mcu unknown plugin: ' + hashParams.plugin));
					return;
				}
				this.handler = new plugin();
				if (hashParams.id) {
					this.uri = this.uri + '/' + hashParams.id;
				}
			}

			this.connect(reconnect).then(socket => {
				resolve(socket);
			}).catch(err => {
				reject(err);
			});
		});
	}

	connect(reconnect=false) {
		return new Promise((resolve, reject) => {
			console.info('connecting to', this.uri, reconnect);
			let connected = false;
			let timedout = false;
			if (reconnect && !this.reconnectEnabled) {
				reconnect = false;
			}

			let ws = new WebSocket(this.uri, 'kwmmcu-protocol');

			let reconnector = () => {
				if (!connected && !reconnect) {
					return;
				}
				if (this.handler !== null) {
					// For now never reconnect handler connections.
					return;
				}
				let reconnectDelay = this.reconnectInterval * Math.pow(this.reconnectFactor, this.reconnectAttempts);
				setTimeout(() => {
					this.reconnectAttempts++;
					this.connect(true);
				}, reconnectDelay > this.maxReconnectInterval ? this.maxReconnectInterval : reconnectDelay);
				console.info('websocket reconnecting in', reconnectDelay);
			};

			let timeout = setTimeout(() => {
				ws.close();
				if (reconnect) {
					reconnector();
					return;
				}

				reject(new Error('connection timeout'));
				if (this.ws !== ws) {
					return;
				}
				this.ws = null;
			}, this.connectTimeout);

			ws.onclose = event => {
				clearTimeout(timeout);
				if (this.ws !== ws) {
					return;
				}
				console.warn('websocket close', event);
				if (this.handler !== null && this.handler.onclose) {
					this.handler.onclose(event);
				}
				this.ws = null;
				reconnector();
			};
			ws.onerror = err => {
				if (timedout) {
					return;
				}
				console.error('websocket error', err);
				if (this.handler !== null && this.handler.onerror) {
					this.handler.onerror(err);
				}
				if (reconnect && !connected) {
					return;
				}
				reject(err);
			};
			ws.onmessage = event => {
				if (this.handler !== null) {
					this.handler.process(event.target, JSON.parse(event.data));
				} else {
					this.process(JSON.parse(event.data));
				}
			};
			ws.onopen = event => {
				clearTimeout(timeout);
				connected = true;
				if (timedout) {
					return;
				}
				console.info('websocket open', event);
				if (this.handler !== null && this.handler.onopen) {
					this.handler.onopen(event);
				}
				resolve(ws);
				this.reconnectAttempts = 0;
			};

			this.ws = ws;
		});
	}

	process(msg) {
		console.debug('processing mcu msg', msg);
		switch (msg.type) {
			case 'attach':
				let plugin = this.plugins[msg.plugin];
				if (!plugin) {
					console.warn('mcu unknown plugin', msg.plugin);
					return;
				}

				// Create iframe and load handler in there.
				let iframe = document.createElement('iframe');
				iframe.onload = event => {
					console.debug('iframe loaded', event);
				};
				iframe.src = location.href + '#plugin=' + msg.plugin + '&id=' + msg.transaction;
				document.body.appendChild(iframe);
				break;

			default:
				console.warn('mcu unknown type', msg.type);
				break;
		}
	}
}


function decodeParams(s) {
	var regex = /([^&=]+)=([^&]*)/g;
	var result = {};
	var m;
	while (m = regex.exec(s)) {
		result[decodeURIComponent(m[1])] = decodeURIComponent(m[2]);
	}

	return result;
}

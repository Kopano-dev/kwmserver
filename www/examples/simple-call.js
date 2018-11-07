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

window.app = new Vue({
	el: '#app',
	data: {
		source: '',
		target: '',
		error: null,
		connecting: false,
		connected: false,
		reconnecting: false,
		latency: undefined,
		peercallPending: undefined,
		peercall: undefined,
		localStream: undefined,
		remoteStream: undefined,
		settings: {
			connect: false,
			accept: false
		},
		gUMconstraints: commonGumHelper.defaultConstraints,
		webrtcConfig: commonWebRTCDefaultConfig,
		kwmOptions: {}
	},
	components: commonComponents({
		// local components.
	}),
	created: function() {
		console.info('welcome to simple-call');
		this.kwm = new KWM.KWM('', this.kwmOptions);
		this.kwm.onstatechanged = event => {
			this.connecting = event.connecting;
			this.connected = event.connected;
			this.reconnecting = event.reconnecting;
		};
		this.kwm.onerror = event => {
			this.error = event;
		};
		this.kwm.webrtc.config = this.$data.webrtcConfig;
		this.kwm.webrtc.onpeer = event => {
			console.debug('onpeer', event);
			switch (event.event) {
				case 'incomingcall':
					this.peercallPending = event.record;
					if (this.settings.accept) {
						console.info('auto accepting incoming call');
						this.accept();
					}
					break;
				case 'newcall':
					if (!this.peercall || this.peercall.user !== event.record.user) {
						throw new Error('invalid peer call');
					}
					this.peercall = event.record;
					break;
				case 'destroycall':
					if (event.record === this.peercall || event.record === this.peercallPending) {
						this.remoteStream = undefined;
						this.hangup();
					}
					break;
				case 'abortcall':
					if (event.details) {
						this.error = {
							code: 'aborted',
							msg: event.details
						};
					}
					this.hangup();
					break;
				case 'pc.error':
					console.error('peerconnection error', event);
					this.error = {
						code: 'peerconnection_error',
						msg: event.details
					};
					this.hangup();
					break;
				default:
					console.debug('unknown peer event', event.event, event);
					break;
			}
		};
		this.kwm.webrtc.onstream = event => {
			console.debug('onstream', event);
			if (event.record !== this.peercall) {
				console.warn('received stream for wrong peer', event.record);
				return;
			}
			if (this.remoteStream) {
				console.warn('received stream but have one already', event.stream);
				return;
			}
			this.remoteStream = event.stream;
		};

		const queryValues = parseParams(location.search.substr(1));
		console.log('URL query values on load', queryValues);
		if (queryValues.source) {
			this.source = queryValues.source;
		}
		if (queryValues.target) {
			this.target = queryValues.target;
		}
		if (queryValues.accept) {
			this.settings.accept = true;
		}
		if (queryValues.connect) {
			this.settings.connect = true;
			this.$nextTick(this.connect);
		}

		setInterval(() => {
			if (this.kwm) {
				const latency = this.kwm.latency;
				this.$nextTick(() => {
					this.latency = latency;
				});
			}
		}, 500);
	},
	watch: {
	},
	methods: {
		connect: async function() {
			console.log('connect clicked');
			try {
				// Get admin token.
				const token = await fetchAdminToken(null, this.source);
				console.log('admin token received', token);
				this.kwmOptions.authorizationType = token.type;
				this.kwmOptions.authorizationValue = token.value;
			} catch (err) {
				console.error('connect auth failed', err);
				return;
			}

			this.kwm.connect(this.source).then(() => {
				console.log('connected');
			}).catch(err => {
				console.error('connect failed', err);
				this.error = err;
			});
		},
		reload: function() {
			window.location.reload();
		},
		closeErrorModal: function() {
			this.error = undefined;
		},
		call: function() {
			console.log('call clicked');

			if (this.peercall || this.peercallPending) {
				return;
			}

			const peercall = {
				user: this.target
			};
			this.peercall = peercall;
			this.getUserMedia().then(ok => {
				if (!ok || !this.peercall) {
					this.hangup();
					return;
				}

				this.kwm.webrtc.setLocalStream(this.localStream);
				this.kwm.webrtc.doCall(peercall.user).then(channel => {
					console.log('doCall sent', channel);
				});
			});
		},
		hangup: function() {
			console.log('hangup clicked');

			if (!this.peercall && !this.peercallPending) {
				return;
			}

			this.kwm.webrtc.doHangup().then(channel => {
				console.log('doHangup sent', channel);
			});
			this.peercall = undefined;
			this.peercallPending = undefined;

			if (this.localStream) {
				this.stopUserMedia(this.localStream);
				this.localStream = undefined;
			}
		},
		accept: function() {
			console.log('accept clicked');

			if (!this.peercallPending || this.peercall) {
				return;
			}

			const peercall = this.peercallPending;
			this.peercall = peercall;
			this.peercallPending = undefined;
			this.getUserMedia().then(ok => {
				if (!ok) {
					this.hangup();
					return;
				}

				this.kwm.webrtc.setLocalStream(this.localStream);
				this.kwm.webrtc.doAnswer(peercall.user).then(channel => {
					console.log('doAnwser sent', channel);
				});
			});
		},
		reject: function() {
			console.log('reject clicked');

			if (!this.$data.peercallPending) {
				return;
			}
			const peercall = this.peercallPending;
			this.peercallPending = undefined;

			this.kwm.webrtc.doHangup(peercall.user, 'reject').then(channel => {
				console.log('doHangup reject sent', channel);
			});
		},
		getUserMedia: function() {
			var constraints = this.gUMconstraints;
			console.log('starting getUserMedia', constraints);

			return commonGumHelper.getUserMedia(constraints)
				.then(mediaStream => {
					console.log('getUserMedia done', mediaStream);
					this.localStream = mediaStream;
					return true;
				})
				.catch(err => {
					console.error('getUserMedia error', err.name + ': ' + err.message, err);
					this.$data.error = {
						code: 'get_usermedia_failed',
						msg: err.name + ': ' + err.message
					};
					this.localStream = undefined;
					return false;
				});
		},
		stopUserMedia: function(localStream) {
			console.log('stopping getUserMedia');

			return commonGumHelper.stopUserMedia(localStream);
		}
	}
});

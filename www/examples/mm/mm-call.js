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
		mm: undefined,
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
		webrtcConfig: commonWebRTCDefaultConfig
	},
	components: commonComponents({
		// local components.
	}),
	created: function() {
		console.info('welcome to mm-call');

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
			if (this.mm && this.mm.session) {
				const latency = this.mm.session.latency;
				this.$nextTick(() => {
					this.latency = latency;
				});
			}
		}, 500);
	},
	watch: {
	},
	methods: {
		connect: function() {
			console.log('connect clicked');
			this.mm = new FakeMM(this);
			this.getUserMedia();

			this.mm.onConnectCall(this.source);
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

			this.mm.doCall();
		},
		hangup: function() {
			console.log('hangup clicked');

			if (!this.peercall && !this.peercallPending) {
				return;
			}

			this.peercall = undefined;
			this.peercallPending = undefined;

			this.mm.doHangup();
		},
		accept: function() {
			console.log('accept clicked');

			if (!this.peercallPending || this.peercall) {
				return;
			}

			const peercall = this.peercallPending;
			this.peercall = peercall;
			this.peercallPending = undefined;

			this.mm.doAnswer(peercall);
		},
		destroy: function() {
			console.log('destroy clicked');

			if (this.mm) {
				this.hangup();

				this.mm.close();
				this.mm = undefined;
			}
			if (this.localStream) {
				this.stopUserMedia(this.localStream);
				this.localStream = undefined;
			}
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
		},
		onSessionCreated: function() {
			// UI integration.
			this.mm.session.onstatechanged = event => {
				this.connecting = event.connecting;
				this.connected = event.connected;
				this.reconnecting = event.reconnecting;
			};
			this.mm.session.onerror = event => {
				this.error = event;
			};
		},
		onIncomingCall: function(event, jsep) {
			this.peercallPending = jsep;
			if (this.settings.accept) {
				console.info('auto accepting incoming call');
				this.accept(jsep);
			}
		},
		handleRemoteStream: function(stream) {
			if (this.remoteStream) {
				console.warn('received stream but have one already', event.stream);
				return;
			}
			this.remoteStream = stream;
		},
		onHangup: function() {
			this.remoteStream = undefined;

			if (this.peercall || this.peercallPending) {
				this.hangup();
			}
		}
	}
});

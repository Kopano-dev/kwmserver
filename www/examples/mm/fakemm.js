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

class FakeMM {
	constructor(app) {
		this.app = app;

		// Huh, we are as crazy as MM.
		this.onConnectCall = this.onConnectCall.bind(this);
		this.onSessionCreated = this.onSessionCreated.bind(this);
		this.onSessionError = this.onSessionError.bind(this);
		this.handleVideoCallEvent = this.handleVideoCallEvent.bind(this);
		this.handleRemoteStream = this.handleRemoteStream.bind(this);
		this.doCall = this.doCall.bind(this);
		this.doAnswer = this.doAnswer.bind(this);
		this.doHangup = this.doHangup.bind(this);
		this.stopRinging = this.stopRinging.bind(this);
		this.doCleanup = this.doCleanup.bind(this);
		this.close = this.close.bind(this);
	}

	onConnectCall(sessionID) {
		console.log('onConnectCall');

		setTimeout(async () => {
			// NOTE(longsleep): Async in MM as info is received from server.
			if (this.session) {
				this.onSessionCreated();
			} else {
				const info = {
					'gateway_url': ''
				};

				const token = await fetchAdminToken('', sessionID);
				console.log('admin token registered', token);

				KWM.KWMInit.init({debug: true});

				this.session = new KWM.KWMInit({
					server: info.gateway_url,
					token: token.value,
					iceServers: this.app.webrtcConfig.iceServers,
					success: this.onSessionCreated,
					error: this.onSessionError
				});
			}
		}, 0);
	}

	onSessionCreated() {
		console.log('onSessionCreated');
		this.app.onSessionCreated();

		if (this.videocall) {
			this.app.call();
		} else {
			this.session.attach({
			   plugin: 'janus.plugin.videocall',
			   success: (plugin) => {
				   this.videocall = plugin;
				   this.videocall.send({message: {request: 'register', username: this.app.source}});
			   },
			   error: this.onSessionError,
			   onmessage: this.handleVideoCallEvent,
			   onremotestream: this.handleRemoteStream
		   });
		}
	}

	onSessionError() {
		console.log('onSessionError');

	}

	handleVideoCallEvent(msg, jsep) {
		console.log('handleVideoCallEvent', msg, jsep, this);

		const result = msg.result;
		if (result) {
			const event = result.event;
			switch (event) {
				case 'registered':
					// NOTE(longsleep): Do nothing here, MM auto calls.
					break;
				case 'incomingcall':
					// NOTE(longsleep): MM auto accepts, we let our UI decide.
					this.app.onIncomingCall(result, jsep);
					break;
				case 'accepted':
					this.stopRinging();
					break;
				case 'hangup':
					this.doHangup(false);
					break;
				default:
					throw new Error('unsupported videocall event: ' + event);
			}
		} else {
			throw new Error('not implemented reached');
		}
	}

	handleRemoteStream(stream) {
		console.log('handleRemoteStream', stream);

		this.app.handleRemoteStream(stream);
	}

	doCall() {
		console.log('doCall');

		setTimeout(() => {
			this.videocall.createOffer({
				stream: this.app.localStream,
				success: (jsepSuccess) => {
					const body = {request: 'call', username: this.app.target};
					this.videocall.send({message: body, jsep: jsepSuccess});
				},
				error: () => {
					this.doHangup(true);
				}
			});
		}, 200);
	}

	doAnswer(jsep) {
		this.videocall.createAnswer({
			jsep,
			stream: this.app.localStream,
			success: (jsepSuccess) => {
				const body = {request: 'accept'};
				this.videocall.send({message: body, jsep: jsepSuccess});
			},
			error: () => {
				this.doHangup(true);
			}
		});
	}

	doHangup(error, manual) {
		if (this.videocall) {
			this.videocall.send({message: {request: 'hangup'}});
			this.videocall.hangup();

			this.app.onHangup();
		}

		if (error) {
			this.onSessionError();
			return this.doCleanup();
		}

		if (manual) {
			return this.close();
		}
	}

	onFailed() {
		this.stopRinging();

		this.doCleanup();
	}

	stopRinging() {
		console.log('stop ringing');
	}

	doCleanup() {
		if (this.videocall) {
			this.videocall.detach();
			this.videocall = null;
		}
	}

	close() {
		this.doCleanup();

		if (this.session) {
			this.session.destroy();
			this.session = null;
		}
	}
};

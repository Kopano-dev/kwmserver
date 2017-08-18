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

export class JanusVideoCall {
	constructor() {
		this.users = {};
		this.ids = {};
		this.pending = {};
		this.active = {};

		console.info('creating janus plugin', this.constructor.plugin);
	}

	static get plugin() {
		return 'janus.plugin.videocall';
	}

	process(socket, msg) {
		//console.debug('janus videocall process janus data', msg);
		switch (msg.janus) {
			case 'message':
				return this.processMessage(socket, msg);

			case 'trickle':
				return this.processTrickle(msg);

			default:
				console.warn('janus videocall unknown type', msg.janus);
				break;
		}
	}

	processTrickle(msg) {
		let active = this.active[msg.session_id];
		if (!active) {
			active = this.active[msg.session_id] = {
				'trickles': [],
				'ready': false
			};
		}
		if (!active.ready) {
			if (active.trickles === undefined) {
				console.warn('janus videocall trickle received unexpected');
				return;
			}
			active.trickles.push(msg);
			return;
		}

		if (msg.body.completed) {
			// do nothing, this message type is janus specific.
			return;
		}

		active.pc.addIceCandidate(new RTCIceCandidate(msg.body)).then(() => {
			//console.debug('added ice candidate');
		}).catch(err => {
			console.error('janus videocall failed to add ice candidate', err);
		});
	}

	processMessage(socket, msg) {
		let body = msg.body;
		let active;

		switch (body.request) {
			case 'register':
				this.users[body.username] = msg.session_id;
				this.ids[msg.session_id] = body.username;

				let response = {
					'janus': 'event',
					'transaction': msg.transaction,
					'sender': msg.handle_id,
					'plugindata': {
						'plugin': this.constructor.plugin,
						'data': {
							'result': {
								'event': 'registered',
								'username': body.username
							}
						}
					}
				};
				//console.debug('sending response', response);
				this.websocketSend(socket, response);

				let pending = this.pending[body.username];
				if (pending) {
					console.debug('janus videocall flushing pending after register', body.username);
					delete this.pending[body.username];
					pending.forEach(pendingMsg => {
						this.processMessage(socket, pendingMsg);
					});
				}

				break;

			case 'hangup':
				active = this.active[msg.session_id];
				if (!active) {
					console.warn('janus videocall hangup for unknown active session, ignored', msg.session_id);
					return;
				}
				Object.keys(active.peers).forEach(sessionID => {
					let response = {
						'janus': 'event',
						'sender': msg.handle_id,
						'plugindata': {
							'plugin': this.constructor.plugin,
							'data': {
								'result': {
									'event': 'hangup',
									'username': this.ids[msg.session_id],
									'reason': 'hangup request received'
								}
							}
						},
						'target_session_id': parseInt(sessionID, 10)
					};
					//console.debug('sending hangup', response, active);
					this.websocketSend(socket, response);
				});

				this.cleanupActive(msg.session_id, active);

				break;

			case 'accept':
				active = this.active[msg.session_id];
				if (!active) {
					console.warn('janus videocall accept for unknown active session, ignored', msg.session_id);
					return;
				}

				active.transaction = msg.transaction;
				active.pc.setRemoteDescription(new RTCSessionDescription(msg.jsep)).then(() => {
					//console.debug('pc2 setRemoteDescription done');
				});

				break;

			case 'call':
				let target = this.users[body.username];
				if (!target) {
					console.debug('janus videocall unknown target username, adding to pending', body.username);
					let pending = this.pending[body.username];
					if (!pending) {
						pending = [];
						this.pending[body.username] = pending;
					}
					if (pending.length > 100) {
						console.warn('janus video call pending quue full, ignored message', body.username);
						return;
					}
					pending.push(msg);
					return;
				}
				let source = msg.session_id;
				let username = this.ids[msg.session_id];

				let pcConfig = {
					'iceCandidatePoolSize': 10
				};
				let pc1 = new RTCPeerConnection(pcConfig);
				let pc2 = new RTCPeerConnection(pcConfig);
				window.pc1 = pc1;
				window.pc2 = pc2;

				let active1 = this.active[source];
				let trickles = [];
				if (active1) {
					if (active1.pc) {
						console.warn('janus videocall for already active source session', source, active1);
						return;
					}
					trickles = active1.trickles;
				}
				let active2 = this.active[target];
				if (active2 && active2.pc) {
					console.warn('janus videocall for already active target session', target, active2);
					return;
				}

				active1 = {
					'pc': pc1,
					'peers': {},
					'session_id': source,
					'transaction': msg.transaction,
					'ready': false,
					'trickles': trickles
				};
				active1.peers[target] = pc2;
				this.active[source] = active1;
				active2 = {
					'pc': pc2,
					'peers': {},
					'session_id': target,
					'transaction': '',
					'ready': true
				};
				active2.peers[source] = pc1;
				this.active[target] = active2;

				pc1.onnegotiationneeded = event => {
					console.debug('janus videocall pc1 negotiation needed', event);
					this.peerConnectionNegotiate(event.target).then(() => {
						//console.debug('set pc1 local description', pc1.localDescription);
					}).catch(err => {
						console.error('janus videocall pc1 negotation error', err);
						this.cleanupActive(source);
					});
				};
				pc2.onnegotiationneeded = event => {
					console.debug('janus videocall pc2 negotiation needed', event);
					this.peerConnectionNegotiate(event.target).then(() => {
						//console.debug('set pc2 local description', pc2.localDescription);
					}).catch(err => {
						console.error('janus videocall pc2 negotiation error', err);
						this.cleanupActive(target);
					});
				};
				// NOTE(longsleep): addStream and onAddStream are deprecated, but
				// chrome still uses them.
				pc1.onaddstream = event => {
					//console.debug('pc1 addstream', event, event.stream);
					let elem = document.createElement('audio');
					elem.muted = true;
					elem.srcObject = event.stream; // XXX(longsleep): Hack to make audio work.
					pc2.addStream(event.stream);
				};
				pc2.onaddstream = event => {
					//console.debug('pc2 addstream', event, event.stream);
					pc1.addStream(event.stream);
				};
				pc1.onsignalingstatechange = event => {
					console.debug('janus videocall pc1 signalingstatechange', event.target.signalingState);
				};
				pc2.onsignalingstatechange = event => {
					console.debug('janus videocall pc2 signalingstatechange', event.target.signalingState);
				};
				pc1.oniceconnectionstatechange = event => {
					console.debug('janus videocall pc1 iceconnectionstatechange', event.target.iceConnectionState);
					if (event.target.iceConnectionState === 'closed' ||
						event.target.iceConnectionState === 'failed') {
						console.info('janus videocall pc1 ended', event.target.iceConnectionState);
						this.cleanupActive(source);
					}
				};
				pc2.oniceconnectionstatechange = event => {
					console.debug('janus videocall pc2 iceconnectionstatechange', event.target.iceConnectionState);
					if (event.target.iceConnectionState === 'closed' ||
						event.target.iceConnectionState === 'failed') {
						console.info('janus videocall pc2 ended', event.target.iceConnectionState);
						this.cleanupActive(target);
					}
				};
				pc1.onicecandidate = event => {
					console.debug('pc1 icecandidate', event);

					if (event.candidate !== null) {
						return;
					}
					let active = this.active[target];
					let response = {
						'janus': 'event',
						'transaction': active.transaction,
						'sender': msg.handle_id,
						'plugindata': {
							'plugin': this.constructor.plugin,
							'data': {
								'result': {
									'event': 'accepted',
									'username': this.ids[active.session_id]
								}
							}
						},
						'jsep': pc1.localDescription,
						'target_session_id': msg.session_id
					};
					//console.debug('sending response', response, pc1.signalingState);
					this.websocketSend(socket, response);
				};
				pc2.onicecandidate = event => {
					console.debug('janus videocall pc2 icecandidate', event);

					if (event.candidate !== null) {
						return;
					}
					let response = {
						'janus': 'event',
						'transaction': msg.transaction,
						'sender': msg.handle_id,
						'plugindata': {
							'plugin': this.constructor.plugin,
							'data': {
								'result': {
									'event': 'incomingcall',
									'username': username
								}
							}
						},
						'jsep': pc2.localDescription,
						'target_session_id': target
					};
					//console.debug('sending response', response, pc2.signalingState);
					this.websocketSend(socket, response);
				};

				pc1.setRemoteDescription(new RTCSessionDescription(msg.jsep)).then(() => {
					console.debug('janus videocall pc1 setRemoteDescription done', msg.jsep);
					active1.ready = true;
					if (active1.trickles) {
						active1.trickles.forEach(msg => {
							this.processTrickle(msg);
						});
					}
					active1.trickles = [];
				});
				break;

			default:
				console.warn('janus videocall unknown request type', body.request);
				break;
		}
	}

	websocketSend(socket, msg) {
		let raw = JSON.stringify(msg);
		socket.send(raw);
	}

	peerConnectionNegotiate(pc) {
		let offerOptions = {
			'offerToReceiveAudio': true,
			'offerToReceiveVideo': true
		};

		console.debug('janus videocall negotiate', pc.signalingState);
		switch (pc.signalingState) {
			case 'have-remote-offer':
				return pc.createAnswer().then(answer => {
					console.debug('janus videocall created answer', answer);
					return pc.setLocalDescription(answer);
				});
			case 'stable':
				return pc.createOffer(offerOptions).then(offer => {
					console.debug('janus videocall created offer', offer);
					return pc.setLocalDescription(offer);
				});
			default:
				console.warn('janus videocall negotiation needed when in unknown signaling state', pc.signalingState);
				break;
		}
	}

	cleanupActive(sessionID=null, active=null) {
		if (active === null) {
			active = this.active[sessionID];
		}
		if (!active) {
			return;
		}
		console.debug('janus videocall cleaning up active session', sessionID);

		active.ready = false;
		delete active.trickles;
		if (active.pc && active.pc.signalingState !== 'closed') {
			active.pc.close();
		}
		active.pc = null;
		Object.entries(active.peers).forEach(([id, pc]) => {
			if (pc.signalingState !== 'closed') {
				pc.close();
			}
		});
		active.peers = {};
		delete this.active[sessionID];
	}

	onclose() {
		console.info('janus videocall onclose');
		this.users = {};
		this.ids = {};
		this.pending = {};
		Object.entries(this.active).forEach(([id, active]) => {
			if (active.pc) {
				active.pc.close();
				active.pc = null;
			}
			active.peers = {};
		});
		this.active = {};
	}
}

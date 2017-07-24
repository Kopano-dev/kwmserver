/* jshint esversion:6 */
/* jshint browser:true */
/* jshint camelcase:false */
/* jshint globalstrict:true */
/* jshint devel:true */
'use strict';

function parseParams(s) {
	if (!s) {
		return {};
	}
	let pieces = s.split('&');
	let data = {};
	let parts;
	for (let i = 0; i < pieces.length; i++) {
		parts = pieces[i].split('=');
		if (parts.length < 2) {
			parts.push('');
		}
		data[decodeURIComponent(parts[0])] = decodeURIComponent(parts[1]);
	}

	return data;
}

function encodeParams(data) {
	let ret = [];
	for (let d in data) {
		ret.push(encodeURIComponent(d) + '=' + encodeURIComponent(data[d]));
	}
	return ret.join('&');
}

function toHexString(byteArray) {
	return byteArray.map(function(byte) {
		/* jshint bitwise: false */
		return ('0' + (byte & 0xFF).toString(16)).slice(-2);
	}).join('');
}

function getRandomString(length) {
	let bytes = new Uint8Array((length || 32) / 2);
	window.crypto.getRandomValues(bytes);
	return toHexString(Array.from(bytes));
}

function makeURLFromPath(path) {
	let a = document.createElement('a');
	a.href = path;
	return a.href;
}

window.app = new Vue({
	el: '#app',
	data: {
		source: '',
		target: '',
		error: null,
		connectResult: null,
		connecting: false,
		connected: false,
		socket: null,
		peercall: null,

		// TODO(longsleep): Add additional constraints and settings.
		gUMconstraints: {
			audio: true,
			video: {
				width: 640,
				height: 480,
				frameRate: {
					ideal: 10,
					max: 15
				}
			}
		},
		webrtcConfig: {
			iceServers: [
				{url: 'stun:stun.l.google.com:19302'}
			]
		}
	},
	components: {
		'remote-video': {
			props: ['stream'],
			template: `
				<div>
					<video ref="video"></video>
				</div>`,
			watch: {
				stream: function(mediaStream) {
					let video = this.$refs.video;
					if (!mediaStream) {
						video.src = '';
						return;
					}
					video.srcObject = mediaStream;
					video.onloadedmetadata = function(event) {
						video.play();
					};
				}
			}
		},
		'local-video': {
			props: ['stream'],
			template: `
				<div>
					<video ref="video" muted></video>
				</div>`,
			watch: {
				stream: function(mediaStream) {
					let video = this.$refs.video;
					if (!mediaStream) {
						video.src = '';
						return;
					}
					video.srcObject = mediaStream;
					video.onloadedmetadata = function(event) {
						video.play();
					};
				}
			}
		},
	},
	created: function() {
		console.log('welcome to simple-call');

		let queryValues = parseParams(location.search.substr(1));
		console.log('URL query values on load', queryValues);
		if (queryValues.source) {
			this.source = queryValues.source;
		}
		if (queryValues.target) {
			this.target = queryValues.target;
		}
		if (queryValues.connect) {
			this.connect();
		}
	},
	watch: {
		connectResult: (function() {
			let socket = null;

			return function(connectResult) {
				if (!connectResult.ok) {
					return;
				}
				this.$data.connecting = true;

				if (socket) {
					console.log('closing existing socket');
					socket.close();
				}

				this.$data.connected = false;

				let url = makeURLFromPath(connectResult.url).replace(/^https:\/\//i, 'wss://').replace(/^http:\/\//i, 'ws://');
				console.log('connecting socket URL', url);
				socket = new WebSocket(url);
				socket.onopen = event => {
					if (event.target !== socket) {
						return;
					}
					console.log('socket connected', event);
					this.$data.socket = socket;
					this.$data.connected = true;
					this.$data.connecting = false;
				};
				socket.onclose = event => {
					if (event.target !== socket) {
						return;
					}
					console.log('socket closed', event);
					this.$data.socket = null;
					this.$data.connected = false;
					this.$data.connecting = false;
				};
				socket.onerror = err => {
					if (event.target !== socket) {
						return;
					}
					console.log('socket error', err);
					this.$data.socket = null;
					this.$data.connected = false;
					this.$data.connecting = false;
					this.$data.error = {
						code: 'socket error',
						msg: err
					};
				};
				socket.onmessage = event => {
					if (event.target !== socket) {
						socket.close();
						return;
					}
					//console.log('socket message', event);
					let message = JSON.parse(event.data);
					switch (message.type) {
						case 'hello':
							console.log('server said hello', message);
							break;
						case 'goodbye':
							console.log('server said goodbye, close connection', message);
							socket.close();
							this.$data.connected = false;
							this.$data.socket = null;
							break;
						case 'webrtc':
							this.handleWebRTC(message);
							break;
						default:
							console.log('unknown type', message.type, message);
							break;
					}
				};
			};
		})()
	},
	methods: {
		connect: function() {
			console.log('connect clicked');

			let target = '/api/v1/rtm.connect';
			let params = {
				user: this.$data.source
			};
			this.$http.post(target, encodeParams(params), {
				headers: {
					'Content-Type': 'application/x-www-form-urlencoded'
				}
			}).then(response => {
				return response.json();
			}, errorResponse => {
				let error = {
					ok: false,
					code: 'http_error_' + errorResponse.status,
					msg: errorResponse.statusText
				};

				return error;
			}).then(connectResult => {
				console.log('connectResult', connectResult);
				if (!connectResult.ok) {
					this.$data.error = connectResult;
					return;
				}

				this.$data.connectResult = connectResult;
			});
		},
		reload: function() {
			window.location.reload();
		},
		closeErrorModal: function() {
			this.$data.error = null;
		},
		call: function() {
			console.log('call clicked');
			let data = {
				type: 'webrtc',
				subtype: 'webrtc_call',
				target: this.$data.target,
				initiator: true,
				state: getRandomString(12)
			};
			let peercall = {
				initiator: true,
				peer: data.target,
				pc: null,
				localStream: null,
				remoteStream: null,
				channel: null,
				state: data.state,
				hash: null
			};
			this.$data.peercall = peercall;
			this.getUserMedia(peercall).then(() => {
				if (this.$data.peercall !== peercall) {
					return;
				}
				this.websocketSend(data);
			});
		},
		hangup: function() {
			console.log('hangup clicked');
			if (!this.$data.peercall) {
				return;
			}
			let peercall = this.$data.peercall;
			if (peercall.pc) {
				// close
				peercall.pc.destroy();
				peercall.pc = null;
			}
			if (peercall.localStream) {
				// kill gUM.
				this.stopUserMedia(peercall.localStream);
				peercall.localStream = null;
			}
			this.$data.peercall = null;
		},

		//  webbsocket functions.
		websocketSend: function(data) {
			let socket = this.$data.socket;
			if (socket === null) {
				throw 'no socket';
			}

			let raw = JSON.stringify(data);
			socket.send(raw);
		},

		handleWebRTC: function(message) {
			console.log('received webrtc message', message);
			switch (message.subtype) {
				case 'webrtc_call':
					if (message.initiator) {
						let response = {
							type: 'webrtc',
							subtype: 'webrtc_call',
							target: message.source,
							state: getRandomString(12),
							channel: message.channel,
							hash: message.hash
						};

						if (this.$data.peercall) {
							console.log('rejecting incoming call while already have a call');
							response.data = {
								accept: false,
								reason: 'reject_busy'
							};
							this.websocketSend(response);
							return;
						}

						// incoming call request, auto accept.
						response.data = {
							accept: true
						};
						let peercall = {
							initiator: false,
							peer: message.source,
							pc: null,
							localStream: null,
							remoteStream: null,
							channel: message.channel,
							state: response.state,
							hash: response.hash
						};
						this.$data.target = message.source;
						this.$data.peercall = peercall;
						this.getUserMedia(peercall).then(() => {
							if (this.$data.peercall !== peercall) {
								return;
							}
							this.websocketSend(response);
						});
					} else {
						// call reply, check and start webrtc.
						if (!message.data.accept) {
							console.log('peer did not accept call', message);
							this.hangup();
							this.$data.error = {
								code: 'not_accepted',
								msg: message.data.reason
							};
							return;
						}
						if (!this.$data.peercall) {
							return;
						}
						let peercall = this.$data.peercall;
						if (peercall.peer !== message.source) {
							console.log('peer is the wrong source');
							this.hangup();
							return;
						}

						peercall.channel = message.channel;
						peercall.hash = message.hash;
						console.log('start webrtc, accept call reply');
						this.getPeerConnection(peercall);
					}
					break;

				case 'webrtc_signal':
					if (!this.$data.peercall) {
						return;
					}
					let peercall = this.$data.peercall;
					// check channel
					if (peercall.channel !== message.channel) {
						console.log('wrong channel');
						return;
					}
					if (peercall.peer !== message.source) {
						console.log('wrong source');
						return;
					}
					if (!message.data) {
						console.log('webrtc signal data empty');
						return;
					}

					if (!peercall.pc) {
						console.log('start webrtc, received signal');
						this.getPeerConnection(peercall);
					}
					peercall.pc.signal(message.data);

					break;

				default:
					console.log('unknown webrtc subtype', message.subtype, message);
					break;
			}
		},

		getPeerConnection: function(peercall) {
			console.log('peerconnection create', peercall.initiator, peercall.localStream);
			let pc = new SimplePeer({
				initiator: peercall.initiator,
				stream: peercall.localStream,
				trickle: true,
				config: this.$data.webrtcConfig
			});
			pc.on('error', err => {
				console.log('peerconnection error', err);
				if (this.$data.peercall !== peercall) {
					return;
				}
				this.$data.error = {
					code: 'perrconnection error',
					msg: err
				};
				this.hangup();
			});
			pc.on('signal', data => {
				console.log('peerconnection signal', data);
				let message = {
					type: 'webrtc',
					subtype: 'webrtc_signal',
					target: peercall.peer,
					state: peercall.state,
					channel: peercall.channel,
					hash: peercall.hash,
					data: data
				};
				this.websocketSend(message);
			});
			pc.on('connect', () => {
				console.log('peerconnection connect');
			});
			pc.on('close', () => {
				console.log('peerconnection close');
				if (this.$data.peercall !== peercall) {
					return;
				}
				this.hangup();
			});
			pc.on('stream', mediaStream => {
				console.log('peerconnection stream', mediaStream);
				peercall.remoteStream = mediaStream;
			});
			pc.on('iceStateChange', state => {
				console.log('iceStateChange', state);
			});
			pc.on('signalingStateChange', state => {
				console.log('signalingStateChange', state);
			});

			peercall.pc = pc;
			return pc;
		},
		getUserMedia: function(peercall) {
			// Prefer camera resolution nearest to 1280x720.
			var constraints = this.$data.gUMconstraints;
			console.log('starting getUserMedia', constraints);
			return navigator.mediaDevices.getUserMedia(constraints)
				.then(mediaStream => {
					console.log('getUserMedia done', mediaStream);
					if (this.$data.peercall !== peercall) {
						this.stopUserMedia(mediaStream);
						return;
					}
					peercall.localStream = mediaStream;
					/*let video = document.getElementById('video-local');
					video.srcObject = mediaStream;
					video.onloadedmetadata = function(event) {
						video.play();
					};*/
				})
				.catch(err => {
					console.log('getUserMedia error', err.name + ': ' + err.message);
					peercall.localStream = null;
					/*let video = document.getElementById('video-local');
					video.src = '';*/
				});
		},
		stopUserMedia: function(localStream) {
			console.log('stopping getUserMedia');
			for (let track of localStream.getTracks()) {
				track.stop();
			}
		}
	}
});

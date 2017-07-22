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
		peercall: null
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
				};
				socket.onmessage = event => {
					if (event.target !== socket) {
						return;
					}
					console.log('socket message', event);
					let message = JSON.parse(event.data);
					switch (message.type) {
						case 'hello':
							break;
						case 'webrtc':
							this.webrtc_handler(message);
							break;
						default:
							console.log('unknown type', message.type);
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
		send: function(data) {
			let socket = this.$data.socket;
			if (socket === null) {
				throw 'no socket';
			}

			let raw = JSON.stringify(data);
			socket.send(raw);
		},
		webrtc_call: function() {
			console.log('webrtc_call clicked');
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
				this.send(data);
			});
		},
		webrtc_hangup: function() {
			console.log('webrtc_hangup clicked');
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
		webrtc_handler: function(message) {
			console.log('received webrtc message', message);
			switch (message.subtype) {
				case 'webrtc_call':
					if (message.initiator) {
						// incoming call request, auto accept.
						let response = {
							type: 'webrtc',
							subtype: 'webrtc_call',
							target: message.source,
							state: getRandomString(12),
							channel: message.channel,
							hash: message.hash,
							data: {
								accept: true
							}
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
							this.send(response);
						});
					} else {
						// call reply, check and start webrtc.
						if (!message.data.accept) {
							console.log('peer did not accept call');
							this.webrtc_hangup();
							return;
						}
						if (!this.$data.peercall) {
							return;
						}
						let peercall = this.$data.peercall;
						if (peercall.peer !== message.source) {
							console.log('peer is the wrong source');
							this.webrtc_hangup();
							return;
						}

						peercall.channel = message.channel;
						peercall.hash = message.hash;
						console.log('start webrtc, accept call reply');
						this.getPeerConnection(peercall);
					}
					break;

				case 'webrtc_offer':
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
					console.log('unknown webrtc subtype', message.subtype);
					break;
			}
		},
		getPeerConnection: function(peercall) {
			console.log('peerconnection create', peercall.initiator, peercall.localStream);
			let pc = new SimplePeer({
				initiator: peercall.initiator,
				stream: peercall.localStream,
				trickle: true
			});
			pc.on('error', err => {
				console.log('peerconnection error', err);
				if (this.$data.peercall !== peercall) {
					return;
				}
				this.webrtc_hangup();
			});
			pc.on('signal', data => {
				console.log('peerconnection signal', data);
				let message = {
					type: 'webrtc',
					subtype: 'webrtc_offer',
					target: peercall.peer,
					state: peercall.state,
					channel: peercall.channel,
					hash: peercall.hash,
					data: data
				};
				this.send(message);
			});
			pc.on('connect', () => {
				console.log('peerconnection connect');
			});
			pc.on('close', () => {
				console.log('peerconnection close');
				if (this.$data.peercall !== peercall) {
					return;
				}
				this.webrtc_hangup();
			});
			pc.on('stream', mediaStream => {
				console.log('peerconnection stream', mediaStream);
				peercall.remoteStream = mediaStream;
				let video = document.getElementById('video-remote');
				video.srcObject = mediaStream;
				video.onloadedmetadata = function(event) {
					video.play();
				};
			});

			peercall.pc = pc;
			return pc;
		},
		stopUserMedia: function(localStream) {
			for (let track of localStream.getTracks()) {
				track.stop();
			}
		},
		getUserMedia: function(peercall) {
			// Prefer camera resolution nearest to 1280x720.
			var constraints = {
				audio: true,
				video: {
					width: 1280,
					height: 720
				}
			};
			return navigator.mediaDevices.getUserMedia(constraints)
				.then(mediaStream => {
					console.log('getUserMedia done', mediaStream);
					if (this.$data.peercall !== peercall) {
						this.stopUserMedia(mediaStream);
						return;
					}
					peercall.localStream = mediaStream;
					let video = document.getElementById('video-local');
					video.srcObject = mediaStream;
					video.onloadedmetadata = function(event) {
						video.play();
					};
				})
				.catch(err => {
					console.log(err.name + ': ' + err.message);
				});
		},
		reload: function() {
			window.location.reload();
		},
		closeErrorModal: function() {
			this.$data.error = null;
		}
	}
});

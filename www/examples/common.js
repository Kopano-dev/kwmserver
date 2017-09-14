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

function commonComponents(components) {
	components = components || {};

	components['streamed-video'] = {
		props: ['stream', 'muted'],
		template: `
			<div>
				<video ref="video" v-bind:muted="muted"></video>
			</div>`,
		watch: {
			stream: function(mediaStream) {
				const video = this.$refs.video;
				if (!mediaStream) {
					video.srcObject = undefined;
					return;
				}
				video.srcObject = mediaStream;
				video.onloadedmetadata = function(event) {
					video.play();
				};
			}
		}
	};

	return components;
}

const commonWebRTCDefaultConfig = {
	iceServers: [
		{url: 'stun:stun.l.google.com:19302'}
	]
};

const commonGumHelper = {
	// TODO(longsleep): Add additional constraints and settings.
	// NOTE(longsleep): Firefox does not support frameRate and thus fails.
	defaultConstraints: {
		audio: true,
		video: {
			width: 640,
			height: 360,
			frameRate: {
				ideal: 10
			}
		}
	},
	getUserMedia: function(constraints) {
		return navigator.mediaDevices.getUserMedia(constraints);
	},
	stopUserMedia: function(localStream) {
		for (let track of localStream.getTracks()) {
			track.stop();
		}
	}
};

async function fetchAdminToken(baseURI, sessionID) {
	const url = (baseURI ? baseURI : '') + '/api/v1/admin/auth/tokens';
	const payload = {
		'type': 'Token'
	};
	if (sessionID) {
		payload.id = sessionID;
	}

	return fetch(url, {
		body: JSON.stringify(payload),
		method: 'POST'
	}).then(response => {
		return response.json();
	});
}

function getRandomHexString(n) {
	const b = new Uint8Array(n);
	window.crypto.getRandomValues(b);
	const pad = s => '00'.slice(s.length) + s;
	const mapped = Array.prototype.map.call(b, c => pad(c.toString(16)));
	return mapped.join('');
}

# WebRTC

## initiator (simon) creates json and sends it to server

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "sean",
	"initiator": true,
	"state": "some-random-state"
}

## server check and adds stuff and sends it to sean

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "sean",
	"source": "simon",
	"initiator": true,
	"state": "some-random-state",
	"channel": "server-made-random-string",
	"hash": hmac(type,sorted(source,target),channel)
}

### checks

- initiator must be true if without channel and hash.
- state and target must not be empty.

## receiver receives json from server and prepares response

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "simon",
	"state": "some-random-state",
	"channel": "some-random-string",
	"hash": hmac(type,sorted(source,target),channel),
	"data": {
		"accept": true,
	}
}

## server checks response and sends it to simon --

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "simon",
	"source": "sean",
	"state": "some-random-state",
	"channel": "some-random-string",
	"hash": hmac(type,sorted(source,target),channel),
	"data": {
		"accept": true,
	}
}

### checks

- hmac needs to be valid.
- there must be channel and hash and data.

## simon receives response

Create WebRTC peer connection and exchange signals by sending the WebRTC sdp
data through the server to the target.

{
	"type": "webrtc",
	"subtype": "webrtc_signal",
	"target": "simon",
	"state": "some-random-state",
	"channel": "some-random-string",
	"data": {
		/*sdp*/
	}
}

The `webrtc_signal` type can be sent by both parties. The peer connection is
created on the fly when none is yet there for the source/target.

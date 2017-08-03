# WebRTC

## initiator (simon) creates json and sends it to server

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"id": client-created-sequence-number,
	"target": "sean",
	"initiator": true,
	"state": "simons-random-client-state"
}

## server check and adds stuff, forwards message

### checks

- initiator must be true if without channel and hash.
- state and target must not be empty.

### send channel and hash initiator if message has id

{
	"type": "webrtc",
	"subtype": "webrtc_channel",
	"reply_to": client-created-sequence-number,
	"channel":"server-made-random-string",
	"hash": hmac(type,sorted(source,target),channel)
}

### send initial message without id plus stuff to target (sean)

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "sean",
	"source": "simon",
	"initiator": true,
	"state": "simons-random-client-state",
	"channel": "server-made-random-string",
	"hash": hmac(type,sorted(source,target),channel)
}

## receiver receives json from server and prepares response

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "simon",
	"state": "simons-random-client-state",
	"channel": "server-made-random-string",
	"hash": hmac(type,sorted(source,target),channel),
	"data": {
		"accept": true,
	}
}

## server checks response and sends it to simon

### checks

- hmac needs to be valid.
- there must be state, channel, hash and data.
- data.state must be the previously generated state from the peer.

### send

{
	"type": "webrtc",
	"subtype": "webrtc_call",
	"target": "simon",
	"source": "sean",
	"state": "seans-random-client-state",
	"channel": "server-made-random-string",
	"hash": hmac(type,sorted(source,target),channel),
	"data": {
		"accept": true,
		"state": "simons-random-client-state"
	}
}

## simon receives response

Create WebRTC peer connection and exchange signals by sending the WebRTC sdp
data through the server to the target.

### checks

- channel must match.
- state revceived must match the accepted state from webrtc_call response.
- source must match the accepted source from webrtc_call response.

### additional data can be sent in both ways, with channel and remote state

{
	"type": "webrtc",
	"subtype": "webrtc_signal",
	"target": "sean",
	"state": "simons-random-client-state",
	"channel": "server-made-random-string",
	"data": {
		/*sdp*/
	}
}

{
	"type": "webrtc",
	"subtype": "webrtc_signal",
	"target": "simon",
	"state": "seans-random-client-state",
	"channel": "server-made-random-string",
	"data": {
		/*sdp*/
	}
}

The `webrtc_signal` type can be sent by both parties. The peer connection is
created on the fly when none is yet there for the source/target.

# Example usage

## Create a new call

```javascript
const kwm = new kwmjs.KWM();
kwm.connect('userA').then(() => {
	// Connected, ready to call.
	return kwm.webrtc.doCall('userB');
}).then(channel => {
	// Ringing, waiting for userB to accept call.
});
```

## Accept an incoming call

```javascript
const kwm = new kwmjs.KWM();
kwm.webrtc.onpeer = event => {
	switch (event.event) {
		case 'incomingcall':
			kwm.webrtc.doAnswer(event.record.user).then(channel => {
				// Waiting for connection to establish.
			});
			break;
	}
};
kwm.connect('userB').then(() => {
	// Connected, ready to accept calls.
});
```

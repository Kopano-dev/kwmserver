# Kopano Web Meetings Server (kwmserver)

This project implements the signaling/channelling server for Kopano
Web Meetings.

## Technologies:
  - Go
  - WebSockets
  - WebRTC
  - JSON

## TL;DW

Make sure you have Go 1.8 installed. This assumes your GOPATH is `~/go` and
you have `~/go/bin` in your $PATH and you have [Glide](https://github.com/Masterminds/glide)
installed as well.

```
mkdir -p ~/go/src/stash.kopano.io/kwm
cd ~/go/src/stash.kopano.io/kwm
git clone <THIS-PROJECT> kwmserver
cd kwmserver
make
bin/kwmserverd serve --listen=127.0.0.1:8778
```

## Run unit tests

```
cd ~/go/src/stash.kopano.io/kwm/kwmserver
make test
```

## Documentation

See the `docs` folder. KWM server uses  the [OpenAPI standard](https://openapis.org/) and
the [ReDoc document generator](https://github.com/Rebilly/ReDoc) for rich HTML
API documentation.

Start `kwmserverd` with the `--enable-docs` parameter to make the `./docs` folder
available at `/docs` URL for easy access to documentation.

## Integration

### Mattermost

Kopano Webmeetings can be used with Mattermost to provide WebRTC video calls
within Mattermost.

Mattermost uses the `admin` API to create shared tokens and thus `kwmserverd`
has to be started with `--admin-tokens-key` parameter.

In Mattermost `config.json` use the `webrtc` section to configure Kopano
Webmeetings:

```json
"WebrtcSettings": {
	"Enable": true,
	"GatewayType": "kopano-webmeetings",
	"GatewayWebsocketUrl": "wss://url-to-kwmserverd",
	"GatewayAdminUrl": "https://url-to-kwmserverd",
	"GatewayAdminSecret": "this-is-not-used",
}
```
Make sure to set `GatewayWebsocketUrl` to a public routable URL which gets
routed to the base URL of `kwmserverd`. Mattermost only supports websocket
URLs here, so prefix the URL with `wss://`. KWM handles this automagically
and uses the correct protocol as needed.

The `GatewayAdminUrl` is used internally by the Mattermost server and thus can
be a local/non-public URL, eg `http://127.0.0.1:8778`. If you choose to expose
the Admin API public, make sure to limit access to requests from Mattermost as
KMW currently does not use the `GatewayAdminSecret` option to protect the admin
API by itself.


## License

See `LICENSE.txt` for licensing information of this project.

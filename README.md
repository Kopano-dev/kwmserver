# Kopano Web Meetings Server (kwmserver)

This project implements the signaling/channelling server for Kopano
Web Meetings.

## Technologies:
  - Go
  - WebSockets
  - WebRTC
  - JSON

## Quickstart

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

Kopano Webmeetings server should be exposed with TLS. Usually a frontend HTTP
proxy like Nginx is suitable. For inspiration take a look at the `Caddyfile` in
the root of this project.

## Run unit tests

```
cd ~/go/src/stash.kopano.io/kwm/kwmserver
make test-short
```

## API documentation

See the `docs` folder. KWM server uses  the [OpenAPI standard](https://openapis.org/) and
the [ReDoc document generator](https://github.com/Rebilly/ReDoc) for rich HTML
API documentation.

Start `kwmserverd` with the `--enable-docs` parameter to make the `./docs` folder
available at `/docs` URL for easy access to documentation.

## Integration

Kopano Webmeetings can be integrated into other services to provide audio and
video calls via WebRTC.

### Mattermost

Kopano Webmeetings can be used with Mattermost to provide WebRTC video calls
within Mattermost via the KWM Javascript library included with Mattermost.

Mattermost uses the `admin` API to create shared tokens and thus `kwmserverd`
should be started with `--admin-tokens-key` parameter to use a persistent
secret key for security tokens used by the `admin` API. The recommended key
size is 32 byte and a suitable file can be generated with:

	openssl rand -out admin-tokens.key 32

In Mattermost configuration file `config.json`, the `Webrtc` section has to be
used to configure/enable the Kopano Webmeetings integration:

```json
"WebrtcSettings": {
	"Enable": true,
	"GatewayType": "kopano-webmeetings",
	"GatewayWebsocketUrl": "wss://url-to-kwmserverd",
	"GatewayAdminUrl": "https://url-to-kwmserverd/api/v1/admin",
	"GatewayAdminSecret": "this-is-not-used",
}
```
Make sure to set `GatewayWebsocketUrl` to a public routable URL which gets
routed to the base URL of `kwmserverd`. Mattermost only supports Websocket
URLs here, so prefix the URL with `wss://`. KWM handles this automagically
and uses the correct protocol as needed.

The `GatewayAdminUrl` is used internally by the Mattermost server and thus can
be a local/non-public URL, eg `http://127.0.0.1:8778/api/v1/admin`. If you
choose to expose the Admin API public, make sure to limit access to requests
from Mattermost as KMW currently does not use the `GatewayAdminSecret` option
to protect the admin API by itself.


## License

See `LICENSE.txt` for licensing information of this project.

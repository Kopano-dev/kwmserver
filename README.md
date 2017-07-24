# Kopano Web Meetings Server (kwmserver)

This project implements the backend signaling/channelling server for Kopano
Web Meetings.

## Technologies:
  - Go
  - WebSockets
  - JSON

## TL;DW

Make sure you have Go 1.8 installed. This assumes your GOPATH is `~/go` and
you have `~/go/bin` in your $PATH and you have [Glide](https://github.com/Masterminds/glide)
installed as well.

NOTE: currently some dependencies are non-public in the Kopano Bitbucket. Thus
glide will ask for your credentials on install.

```
mkdir -p ~/go/src/stash.kopano.io/kwm
cd ~/go/src/stash.kopano.io/kwm
git clone <THIS-PROJECT> kwmserver
cd kwmserver
glide install
go install -v ./cmd/kwmserverd && kwmserverd serve --listen=127.0.0.1:8778
```

## Run unit tests

```
cd ~/go/src/stash.kopano.io/kwm/kwmserver
go test -v $(glide novendor)
```

## RTM API

RTM stands for real time messaging. This API uses a websocket connection with
JSON payload data.

### Try RTM API

One can use the tools [curl](https://curl.haxx.se/), [jq](https://stedolan.github.io/jq/) and [wsc](https://github.com/raphael/wsc) for testing.

```
curl -v http://localhost:8778/api/v1/rtm.connect
> GET /api/v1/rtm.connect HTTP/1.1
> Host: localhost:8778
> User-Agent: curl/7.47.0
> Accept: */*
>
< HTTP/1.1 200 OK
< Content-Type: application/json
< Date: Mon, 17 Jul 2017 16:00:12 GMT
< Content-Length: 98
<
{
  "ok": true,
  "url": "/api/v1/websocket/MTU-g8XPVzEVYLzC7mlEbFgeJ0Q8L6Cz",
  "self": {
    "id": "",
    "name": ""
  }
}
```

```
wsc ws://localhost:8778/api/v1/websocket/MTU-g8XPVzEVYLzC7mlEbFgeJ0Q8L6Cz
<< {
        "type": "hello"
}
>>
```

URLs expire after 30 seconds and can only be used once. For this reason, together
with jq and wsc:

```
wsc ws://localhost:8778$(curl -s http://localhost:8778/api/v1/rtm.connect |jq -r '.url')
```

### RTM Websocket API

The RTM API is a simple JSON based Websocket API that allows to send and received
events in real time.

See `docs/RTM-API.md` for the events are JSON format definition.

# Kopano Web Meetings Server (kwmserver)

This project implements the signaling/channelling server for Kopano
Web Meetings.

## Technologies:
  - Go
  - WebSockets
  - WebRTC
  - JSON
  - Typescript

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

## Documentation

See the `docs` folder. KWM server uses  the [OpenAPI standard](https://openapis.org/) and
the [ReDoc document generator](https://github.com/Rebilly/ReDoc) for rich HTML
API documentation.

Start `kwmserverd` with the `--enable-docs` parameter to make the `./docs` folder
available at `/docs` URL for easy access to documentation.

## License

See `LICENSE.txt` for licensing information of this project.

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

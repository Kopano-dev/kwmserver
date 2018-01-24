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
proxy like Nginx is suitable. For an example take a look at the `Caddyfile.example`
in the root of this project.

### Run with Docker

This project includes a Dockerfile which can be used to build a Docker container
to run Kopano Webmeetings inside a container. The Dockerfile supports all features
of Kopano Webmeetings and can make use of Docker Secrets to manage sensitive
data like keys.

#### Docker Swarm

Make sure to have built this project (see above), then build and setup the Docker
container in swarm mode like this:

```
docker build -t kopano/kwmserverd .
openssl -rand 32 | docker secret create kwmserverd_admin_tokens_key -
docker service create \
	--secret kwmserverd_admin_tokens_key \
	--publish 8778:8778 \
	--name=kwmserverd \
	kopano/kwmserverd
```

#### Without Docker Swarm - running the Docker image

```
docker build -t kopano/kwmserverd .
openssl -rand 32 -out /etc/kopano/kwm-admin-tokens.key
docker run --rm=true --name=kwmserverd \
	--volume /etc/kopano/kwm-admin-tokens.key:/run/secrets/kwmserverd_admin_tokens_key
	--publish 127.0.0.1:18778:8778 \
	 kopano/kwmserverd
```

Of course modify the paths and ports according to your requirements.

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

To integrate Kopano Webmeetings with Mattermost, a Mattermost plugin for
Kopano Webmeetings can be used to enable Audio/Video calling within Mattermost.

## License

See `LICENSE.txt` for licensing information of this project.

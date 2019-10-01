# Kopano Web Meetings Server (kwmserver)

This project implements the signaling/channelling server for Kopano
Web Meetings.

## Technologies

  - Go
  - WebSockets
  - WebRTC
  - JSON

## Quickstart

Either download a KWM Server binary release from https://download.kopano.io/community/kwmserver:/
or use the Docker image from https://hub.docker.com/r/kopano/kwmserverd/ to run
Kopano Webmeetings Server. For details see below.

## Build dependencies

Make sure you have Go 1.13 or later installed. This assumes your GOPATH is `~/go` and
you have `~/go/bin` in your $PATH and you have [Glide](https://github.com/Masterminds/glide)
installed as well.

When building, third party dependencies will tried to be fetched from the Internet
if not there already.

## Building from source

```
mkdir -p ~/go/src/stash.kopano.io/kwm
cd ~/go/src/stash.kopano.io/kwm
git clone <THIS-PROJECT> kwmserver
cd kwmserver
make
```

## Running KWM Server

```
bin/kwmserverd serve --listen=127.0.0.1:8778
```

Kopano Webmeetings server should be exposed with TLS. Usually a frontend HTTP
proxy like Nginx is suitable. For an example take a look at the `Caddyfile.example`
in the root of this project.

### Run with Docker

Kopano Web Meetings Server supports Docker to easily be run inside a container.
Running with Docker supports all features and can make use of Docker Secrets to
manage sensitive data like keys.

Kopano provides [official Docker images for KWM Server](https://hub.docker.com/r/kopano/kwmserverd/).

```
docker pull kopano/kwmserverd
```

#### Run KWM Server with Docker Swarm

Setup the Docker container in swarm mode like this:

```
openssl rand 32 | docker secret create kwmserverd_admin_tokens_key -
docker service create \
	--read-only \
	--secret kwmserverd_admin_tokens_key \
	--publish published=8778,target=8778,mode=host \
	--name=kwmserverd \
	kopano/kwmserverd
```

#### Running from Docker image

```
openssl rand -out /etc/kopano/kwm-admin-tokens.key 32
docker run --rm=true --name=kwmserverd \
	--read-only \
	--volume /etc/kopano/kwm-admin-tokens.key:/run/secrets/kwmserverd_admin_tokens_key:ro \
	--publish 127.0.0.1:8778:8778 \
	 kopano/kwmserverd
```

Of course modify the paths and ports according to your requirements.

#### Build Docker image

This project includes a `Dockerfile` which can be used to build a Docker
container from the locally build version. Similarly the `Dockerfile.release`
builds the Docker image locally from the latest release download.

```
docker build -t kopano/kwmserverd .
```

```
docker build -f Dockerfile.release -t kopano/kwmserverd .
```

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

## Usage survey

By default, any running kwmserverd regularly transmits usage data to a Kopano
user survey service at https://stats.kopano.io . To disable participation, set
the environment variable `KOPANO_SURVEYCLIENT_AUTOSURVEY` to `no`.

The survey data includes system and platform information and the following usage
metrics:

 - rtm_distinct_users_connected_max
 - rtm_group_channels_created_max
 - rtm_channels_created_max
 - rtm_connections_connected_max
 - guest_http_logon_success_total

See [here](https://stash.kopano.io/projects/KGOL/repos/ksurveyclient-go) for further
documentation and customization possibilities.

## License

See `LICENSE.txt` for licensing information of this project.

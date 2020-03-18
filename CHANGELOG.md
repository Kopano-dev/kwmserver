# CHANGELOG

## Unreleased



## v1.1.1 (2020-03-18)

- Add channel pipeline after reset handler support
- Improve connection stability in MCU pipeline connections
- Update 3rd party dependencies
- Update license ranger and generate 3rd party licenses from vendor folder
- Build with Go 1.14


## v1.1.0 (2020-01-22)

- Update external dependencies
- Use Go modules
- Fix spelling mistakes


## v1.0.1 (2019-12-09)

- Build with Go 1.13.4
- Fix panic on startup when --iss parameter is not given
- Update Dockerfile for release
- Build with Go 1.13.3


## v1.0.0 (2019-10-17)

- Improve README


## v0.17.3 (2019-09-30)

- Build with Go 1.13.1
- Add support for Basic auth in RTM signaling


## v0.17.2 (2019-09-10)

- Update Docker entrypoint for metrics listener


## v0.17.1 (2019-09-10)

- Update 3rd party dependencies
- Expose metrics port for Docker containers


## v0.17.0 (2019-09-10)

- Build with Go 1.13 and update minimal required Go version
- Add usage survey block to README
- Use survey metrics alias syntax
- Add guest connect metrics to prometheus and survey client
- Update survey client for proper prometheus conversion
- Mark counter metrics which are counters as counter
- Derive service survey GUID from iss configuration
- Include usage metrics in survey data
- Add metrics for maximum concurrenct RTM manager data
- Improve sorting and add attional RTM metrics
- Add metrics for group channels
- Add automatic survey reporting
- Add kwm rtm metrics for channels, connections and users
- Add basic metrics
- Return better error when TURN service is not available


## v0.16.1 (2019-07-25)

- Add missing return in guest mode logon request error handling


## v0.16.0 (2019-07-09)

- Update Dockerfiles for best practices
- Add healthcheck sub command
- Bump js-yaml from 3.10.0 to 3.13.1 in /docs
- Bump forwarded from 0.1.1 to 0.1.2 in /docs
- Ensure oidc provider is correctly set and log if not
- Use proper import annotation
- Update libkcoidc to 0.6.0 and use its new flexible logger


## v0.15.3 (2019-03-24)

- remove surplus $ from variable


## v0.15.2 (2019-03-18)

- Fix missing registration_conf parameter support
- Typo fix in kwmserverd.cfg
- Add support for auth parameter to select auth mode


## v0.15.1 (2019-02-06)

- Fixup group restriction check for non-guest users
- Detect in guest logon when client_id is not registered


## v0.15.0 (2019-02-06)

- Add guest settings to bin script and its config
- Include registration.yaml.in in dist tarball
- Migrage from glide to dep for dependencies
- Add configuration for public guest access
- Implement guest API configuration
- Add restriction for create channels
- Update libkcoidc to 0.5.0 for guest claim support
- Add RTM api restrictions via access token extra claims
- Implement guest API prototype


## v0.14.0 (2019-01-23)

- Use name claim from ID token auth
- Bump base copyright years to 2019
- Add rudimentary support for kcoidc debugging
- Avoid crash when a connection is cleaning up and possibly nil


## v0.13.1 (2018-12-06)

- Improve busy handling when connected multiple times


## v0.13.0 (2018-12-05)

- Include self information in hello
- Gitignore .vscode


## v0.12.0 (2018-11-12)

- Add support for id_token based identification
- Ensure Bearer token matches subject to request id
- No longer return api.RTMErrorIDNoSessionForUser errors to clients
- Support replace of connections in groups
- Trim bytes from TURN shared secret file
- Add TURN service configuration
- Implement external TURN service support
- Add support for pcid RTM webrtc payload field


## v0.11.0 (2018-11-09)

- Update examples to use v2 API
- Fixup kwmjs compatibility
- Add v2 API
- Move CORS settings from API to manager
- Fixup unit tests after refactoring
- Move API v1 definition to v1 service
- Move manager modules directly into signaling
- Reorganize how API routes are initialized with managers
- Remove obsolete/unmaintained Janus API support
- Move connection module directly into signaling module
- Build with Go 1.11


## v0.10.5 (2018-11-05)

- Include 3rd party license information in dist


## v0.10.4 (2018-10-01)

- scripts: Add missing --iss parameter if set
- Update build checks


## v0.10.3 (2018-09-24)

- Add binscript, cfg and systemd service


## v0.10.2 (2018-09-21)

- Update libkcoidc to 0.4.3


## v0.10.1 (2018-09-06)

- Update libkcoidc to 0.4.1


## v0.10.0 (2018-09-06)

- Unify logging messages
- Add support to require a scope in token
- Update libkcoidc to 0.4.0
- Cleanup token validation to prepare for required scopes


## v0.9.0 (2018-08-29)

- Run Jenkins with Go 1.10
- Add commandline args for log output control
- mcu: Add detach support
- rtm: Pimp transaction support for better call control
- rtm: Implement pipeline reconnect logic
- rtm: Add Pipeline information to channel extra data
- rtm: Remove obsolete inner data from RTM channel extra
- rtm: Add MCU support to RTM channels
- add systemd unit file for running kwmserver directly
- Fix README copy/paste issue


## v0.8.0 (2018-07-03)

- Add support for WebRTC payload version


## v0.7.0 (2018-06-20)

- Group call security support
- Allow muliple channel adds with same connection
- Add server side group support


## v0.6.1 (2018-06-05)

- Generate TURN credentials with TURN username


## v0.6.0 (2018-05-30)

- Add native support to create TURN credentials
- Update examples to use new kwm folder layout (umd)


## v0.5.0 (2018-04-17)

- Improve docker swarm example
- Use corret token type in tests
- Validate that auth matches users by default
- Add support to sign-in with OIDC access token
- Force self generated admin Tokens to type Token
- Docker: Support additional ARGS via environment


## v0.4.2 (2018-03-19)

- server: Disable HTTP request log by default
- Fix openssl rand usage


## v0.4.1 (2018-03-15)

- Update build parameters for Go 1.10 compatibility
- Update README to include official Docker details
- Update to Go 1.9 and Glide 0.13.1
- Never fail on junit in post state
- Update release download link
- Update Docker example to be read only


## v0.4.0 (2018-01-26)

- Add Dockerfile.release
- Fixup: typos
- Add Dockerfile
- Only expose admin API endpoint when key set
- Update README with Caddyfile example
- Update Mattermost integration hints
- Update examples to use webrtc-adapter.js to 6.0.3
- Move example Caddyfile to Caddyfile.dev
- Add test coverage reporting
- Update Caddyfile


## v0.3.3 (2017-10-16)

- Update README
- Update readme
- Add Jenkinsfile
- Remove kwmjs leftovers


## v0.3.2 (2017-10-04)

- Remove note about internal dependencies


## v0.3.1 (2017-10-04)

- Increase default test timeout to 240 and allow override


## v0.3.0 (2017-10-04)

- Move kwmjs to its own repository
- Make variables overrideable
- Build kwmjs as ES6
- Fix script name
- Update commandline flag usage
- Add admin API and authentication
- Fix syntax error
- Add OpenAPI documentation
- Avoid sending messages to closing websocket connection
- Remove referrer from requests by example apps
- Move most debug logging to debug level
- Add build version and date to kwm.js build
- Show reconnecting and latency in example apps
- Implement automatic rtm reconnect
- Prefix kwm.js with license and build 3rdparty license file
- Add webrtc-adapter to example webapps
- Remove chromiumcu, moved to its own project
- Use rndm from external module
- Fix trigger of close event on server side close
- Add support to mute/unmute local stream
- Merge pull request [#6](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/6/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-kwmjs-mmjanus to master
- Export KWM class as root of javascript module
- Merge pull request [#5](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/5/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-kwmjs to master
- Prevent wakelock by properly clearing stream
- Implement KWM javascript client library
- Merge pull request [#4](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/4/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-aci-building to master
- Add aci build script and systemd service
- Add support to serve www directly with signaling server
- Merge pull request [#3](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/3/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-chromiumcu to master
- Move pprof support to cmd package
- Avoid mix cased command line parameters
- Simplify close callbacks and add more logging
- Fix cleanup of active calls
- Validate Janus API tokens
- Improve reconnect logging
- Add locks for janus API and pprof support
- Add commandline parameters for Janus and MCU API
- Add script to enable WebRTC in Mattermost
- Add chromiumcu launcher app
- Add mcu basics and chromiumcu
- Add janus compatible API for MM
- Build static without cgo by default
- Merge pull request [#2](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/2/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-handler-to-service to master
- Refactor handler into service
- Merge pull request [#1](https://stash.kopano.io/projects/KWM/repos/kwmserver/issues/1/) in KWM/kwmserver from ~SEISENMANN/kwmserver:longsleep-signaling-server to master
- Move to use eslint, remove jshint and jscs
- Prepare for multi point mode
- Add Makefile
- Put local imports last
- Use build date in version command
- Allow multiple connections
- Add auto accept to simple-call example
- Implement channel message for pre checks and proper call control
- Implement hash, channels and hangup
- Implement per message source/target/channel hash
- Use Firefox compatible constraints
- Add note about Firefox missing frameRate support
- Reduce example video resolution and fps
- Prepare layout for multistream
- Add ice and signaling change event logging
- Use own default constraints
- Cleanup example client
- Avoid throwing http.Serve error when not required
- Add muted to own video to avoid echo
- Simplify RTM payload
- Implement WebRTC calling with example
- Implement WebRTC messages
- Implement goodbye message
- Rename to a better name
- Move RTM implementation into sub module
- Cleanup
- Add per connection logging and use numeric connection ids
- Add websocket ping with payload
- Wait on connections to exit before server exit
- Replace go-cache with concurrent-map
- Add ping/pong rtm support
- Add RTM api
- KWM-19 #start-progress Implement signaling server
- Initial commit KWM-19 #start-progress Add README


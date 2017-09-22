#!/usr/bin/env bash
set -ex

BINARY=bin/kwmserverd
HOSTOS=$(go env GOHOSTOS)
ARCH=$(go env GOHOSTARCH)

make
VERSION=$($BINARY version|grep Version|awk -F': ' '{ print $2 }')

acbuild --debug begin
trap "{ export EXT=$?; acbuild --debug end && rm -rf $TMPDIR && exit $EXT; }" EXIT

acbuild --debug set-name kopano.com/kwmserverd
acbuild --debug set-user 500
acbuild --debug set-group 500
acbuild --debug copy $BINARY /bin/kwmserverd
acbuild --debug copy www/ /srv/www
acbuild --debug set-exec -- /bin/kwmserverd serve \
	--listen 0.0.0.0:8778 \
	--www-root /srv/www
acbuild --debug port add www tcp 8778
acbuild --debug mount add admin-tokens-key /admin-tokens.key --read-only
acbuild --debug label add version $VERSION
acbuild --debug label add arch $ARCH
acbuild --debug label add os $HOSTOS
acbuild --debug write --overwrite kopano-kwmserverd-$VERSION-$HOSTOS-$ARCH.aci

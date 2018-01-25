#!/bin/sh
#
# Copyright 2018 Kopano and its licensors
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License, version 3 or
# later, as published by the Free Software Foundation.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

FROM alpine:3.7
MAINTAINER Kopano Development <development@kopano.io>

# Expose ports.
EXPOSE 8778

# Define basic environment variables.
ENV EXE=kwmserverd
ENV KWMSERVERD_LISTEN=0.0.0.0:8778
ENV KWMSERVERD_ADMIN_TOKENS_KEY_FILE=kwmserverd_admin_tokens_key

# Add a volume to be able to mount in a well defined way.
VOLUME /var/lib/kwmserverd-docker
WORKDIR /var/lib/kwmserverd-docker

# Copy Docker specific scripts and ensure they are executable.
COPY \
	scripts/docker-entrypoint.sh \
	scripts/healthcheck.sh \
	/usr/local/bin/
RUN chmod 755 /usr/local/bin/*.sh

# Add Docker specific runtime setup functions.
RUN mkdir /etc/defaults && echo $'\
setup_secrets() { \n\
	local adminTokensFile="/run/secrets/${KWMSERVERD_ADMIN_TOKENS_KEY_FILE}" \n\
	if [ -f "${adminTokensFile}" ]; then \n\
		export KWMSERVERD_ADMIN_TOKENS_KEY="${adminTokensFile}" \n\
	fi \n\
}\n\
setup_secrets\n\
' > /etc/defaults/docker-env

# Add project resources.
COPY docs/* /var/lib/kwmserverd-docker/docs/
COPY www/* /var/lib/kwmserverd-docker/www/

# Add project main binary.
COPY bin/kwmserverd /usr/local/bin/${EXE}

# Run as nobody by default is always a good idea.
USER nobody

ENTRYPOINT ["docker-entrypoint.sh"]
CMD [ \
	"kwmserverd", \
	"serve" \
	]

# Health check support is cool too.
HEALTHCHECK --interval=30s --timeout=5s \
	CMD healthcheck.sh

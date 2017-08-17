# Dockerfile to run ChromiuMCU inside a Docker container.
#
# Build with `docker build -t my-chromiumcu .` and then run the image with
# the following command.
#
# ```
# docker run -it --rm --name my-running-chromiumcu \
#    --net=host \
#    --cap-add=SYS_ADMIN \
#    my-chromiumcu:latest -r
# ```
# Note: To run Chromium in Docker, the SYS_ADMIN capability is required. Also to
# make WebRTC work properly, ChromiuMCU must use host networking.

FROM node:7

RUN apt-get update && apt-get install -y --no-install-recommends \
	libx11-xcb1 \
	libxcomposite1 \
	libxcursor1 \
	libxdamage1 \
	libxi6 \
	libxtst6 \
	libnss3 \
	libcups2 \
	libxss1 \
	libxrandr2 \
	libgconf-2-4 \
	libasound2 \
	libatk1.0-0 \
	libgtk-3-0

RUN apt-get clean

WORKDIR /app
COPY package.json yarn.lock /app/
RUN yarn install
COPY . /app

ENTRYPOINT ["node", "index.js"]

RUN chown -R node.node /app/node_modules/puppeteer/.local-chromium

USER node

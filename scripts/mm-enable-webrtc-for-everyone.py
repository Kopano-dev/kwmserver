#!/usr/bin/env python
#
# Copyright 2017 Kopano and its licensors
#
# Simple script to enable WebRTC experiment for all users of a Mattermost.
#
# Usage:
#
#   MM_URL=https://mattermost:8465 MM_TOKEN=mm-token \
#     ./scripts/mm-enable-webrtc-for-everyone.py
#
# Get the value for MM_TOKEN from your browsers cookie store. The user for the
# token needs to be an Mattermost administrator to be able to modify all users.
#
# To disable TLS verification, set VERIFY_TLS=0 in environment. Similarly to
# debug HTTP data flow, set DEBUG=1 in environment.

from __future__ import print_function

import json
import os
import requests
import sys

try:
    import http.client as http_client
except ImportError:
    # Python 2
    import httplib as http_client

if os.environ.get("DEBUG", False):
    http_client.HTTPConnection.debuglevel = 1


class Mattermost:
    def __init__(self, url=None, token="", verify_tls=False):
        self.url = url
        self.token = token
        self.verify_tls = verify_tls

    def enableWebRTCForAll(self):
        # Get list of all users.
        r = requests.get(self.url + "/api/v4/users",
                         headers={"Authorization": "Bearer %s" % self.token},
                         verify=self.verify_tls)
        response = r.json()
        if r.status_code != 200:
            print("Error: %(status_code)s - %(id)s: %(detailed_error)s" %
                  response, file=sys.stderr)
            sys.exit(1)

        # Enable WebRTC for each user.
        print("OK found %d users - processing ..." % len(response))
        for user in response:
            print("-> %s" % user["id"])

            payload = [{
                "user_id": user["id"],
                "category": "advanced_settings",
                "name": "feature_enabled_webrtc_preview",
                "value": "true"
            }]
            r = requests.put(self.url
                             + "/api/v4/users/%s/preferences" % user["id"],
                             json=payload,
                             headers={
                                 "Authorization": "Bearer %s" % self.token
                             },
                             verify=self.verify_tls)
            print("   %s - %s" % (r.status_code, r.json()))


def main():
    url = os.environ.get('MM_URL', '').strip()
    token = os.environ.get('MM_TOKEN', '').strip()
    verify_tls = os.environ.get('VERIFY_TLS', True) != "0"
    if not url or not token:
        print("Error: Please set MM_URL and MM_TOKEN environment variables.",
              file=sys.stderr)
        sys.exit(1)

    mm = Mattermost(url=url, token=token, verify_tls=verify_tls)
    mm.enableWebRTCForAll()


if __name__ == "__main__":
    main()

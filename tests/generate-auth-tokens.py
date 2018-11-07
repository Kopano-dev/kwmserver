#!/usr/bin/env python
#
# Copyright 2017 Kopano
#
# Simple script to generate auth tokens with the /admin/auth/tokens API.
#
# Usage:
#
#   KWM_URL=http://localhost:8778 \
#       python3 tests/auth-tokens.py [number-of-tokens]
#
# To disable TLS verification, set VERIFY_TLS=0 in environment. Similarly to
# debug HTTP data flow, set DEBUG=1 in environment.

from __future__ import print_function
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


class KWM:
    def __init__(self, url=None, verify_tls=False):
        self.url = url
        self.verify_tls = verify_tls
        self.session = requests.Session()

    def adminAuthCreateToken(self, tokenType):
        payload = {
            "type": tokenType
        }
        r = self.session.post(self.url + "/api/kwm/v2/admin/auth/tokens",
                              json=payload,
                              verify=self.verify_tls)
        print("> %s - %s" % (r.status_code, r.text))


def main():
    url = os.environ.get("KWM_URL", "").strip()
    verify_tls = os.environ.get('VERIFY_TLS', True) != "0"
    if not url:
        print("Error: Please set KWM_URL environment variable.\n\
               Eg. KWM_URL=http://localhost:8778",
              file=sys.stderr)
        sys.exit(1)

    args = sys.argv[1:]
    if len(args) > 0:
        try:
            count = int(args[0])
        except ValueError:
            print("Error: number-of-tokens parameter must be a number")
            sys.exit(2)
    else:
        count = 1

    kwm = KWM(url=url, verify_tls=verify_tls)
    for i in range(count):
        kwm.adminAuthCreateToken("Token")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        pass

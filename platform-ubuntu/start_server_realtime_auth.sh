#!/bin/bash

# Start server_realtime_auth with a JWT signing secret unique to this installation.
#
# The secret signs the auth tokens of the web server. The value shipped in
# src/server_realtime_auth/app/config/auth.config.js is public (it is in the
# repository), so anybody knowing it could forge an admin token and issue
# commands to field devices. This script therefore always provides one:
#
#   1. the JS_JWT_SECRET environment variable, when set (e.g. docker run -e ...);
#   2. otherwise the secret stored in <install>/conf/jwt.secret;
#   3. otherwise a new random secret, written to that file on first start.
#
# Delete conf/jwt.secret to roll the secret over (all users must log in again).

JS_BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)
SECRET_FILE="$JS_BASE_DIR/conf/jwt.secret"

if [ -z "$JS_JWT_SECRET" ]; then
    if [ ! -s "$SECRET_FILE" ]; then
        mkdir -p "$JS_BASE_DIR/conf" || exit 1
        ( umask 077 && head -c 48 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$SECRET_FILE" ) || {
            echo "FATAL: cannot write the JWT secret to $SECRET_FILE" >&2
            exit 1
        }
        chmod 400 "$SECRET_FILE"
        echo "Generated a new random JWT secret in $SECRET_FILE"
    fi
    JS_JWT_SECRET=$(cat "$SECRET_FILE")
    export JS_JWT_SECRET
fi

if [ ${#JS_JWT_SECRET} -lt 32 ]; then
    echo "FATAL: JS_JWT_SECRET is missing or too short (need at least 32 characters)." >&2
    exit 1
fi

cd "$JS_BASE_DIR/src/server_realtime_auth" || exit 1
NODE_BIN=node
if [ -x /usr/bin/node ]; then NODE_BIN=/usr/bin/node; fi
exec "$NODE_BIN" index.js

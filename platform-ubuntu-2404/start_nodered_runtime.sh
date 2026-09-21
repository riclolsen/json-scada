#!/bin/bash

# Start the local Node-RED runtime with a credential secret unique to this
# installation.
#
# Node-RED encrypts the credentials stored in flows (passwords, tokens) with this
# secret. The value shipped in conf-templates/node-red-settings.js is public (it
# is in the repository), so anybody knowing it could decrypt the credentials of a
# stolen flows file. This script therefore always provides one:
#
#   1. the JS_NODERED_CRED_SECRET environment variable, when set (e.g. docker run -e ...);
#   2. otherwise the secret stored in <install>/conf/nodered.secret;
#   3. otherwise a new random secret, written to that file on first start.
#
# Changing the secret makes the credentials already stored in flows unreadable:
# Node-RED will report them as invalid and they must be typed in again.

JS_BASE_DIR=$(cd "$(dirname "$0")/.." && pwd)
SECRET_FILE="$JS_BASE_DIR/conf/nodered.secret"

if [ -z "$JS_NODERED_CRED_SECRET" ]; then
    if [ ! -s "$SECRET_FILE" ]; then
        mkdir -p "$JS_BASE_DIR/conf" || exit 1
        ( umask 077 && head -c 48 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$SECRET_FILE" ) || {
            echo "FATAL: cannot write the Node-RED credential secret to $SECRET_FILE" >&2
            exit 1
        }
        chmod 400 "$SECRET_FILE"
        echo "Generated a new random Node-RED credential secret in $SECRET_FILE"
    fi
    JS_NODERED_CRED_SECRET=$(cat "$SECRET_FILE")
    export JS_NODERED_CRED_SECRET
fi

if [ ${#JS_NODERED_CRED_SECRET} -lt 32 ]; then
    echo "FATAL: JS_NODERED_CRED_SECRET is missing or too short (need at least 32 characters)." >&2
    exit 1
fi

cd "$JS_BASE_DIR/nodered-runtime" || exit 1
NODE_BIN=node
if [ -x /usr/bin/node ]; then NODE_BIN=/usr/bin/node; fi
exec "$NODE_BIN" node_modules/node-red/red.js -s "$JS_BASE_DIR/conf/node-red-settings.js"

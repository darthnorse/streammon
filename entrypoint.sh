#!/bin/sh
set -e

KEY_FILE="${DB_PATH%/*}/.encryption_key"

# Auto-generate encryption key if not explicitly provided
if [ -z "$TOKEN_ENCRYPTION_KEY" ]; then
    if [ ! -f "$KEY_FILE" ]; then
        head -c 32 /dev/urandom | base64 > "$KEY_FILE"
        chmod 600 "$KEY_FILE"
        echo "Generated new encryption key at $KEY_FILE"
    fi
    export TOKEN_ENCRYPTION_KEY
    TOKEN_ENCRYPTION_KEY=$(cat "$KEY_FILE")
fi

exec ./streammon "$@"

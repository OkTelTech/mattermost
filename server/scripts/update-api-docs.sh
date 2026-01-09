#!/bin/bash
# Script to update embedded OpenAPI documentation
# Run this before building the server if API docs have changed

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(dirname "$SCRIPT_DIR")"
API_DIR="$(dirname "$SERVER_DIR")/api"

echo "Building OpenAPI spec..."
cd "$API_DIR"
npm install --silent
make build

echo "Copying OpenAPI spec to server..."
cp "$API_DIR/v4/html/static/mattermost-openapi-v4.yaml" \
   "$SERVER_DIR/channels/api4/apidocs/openapi.yaml"

echo "API docs updated successfully!"
echo "OpenAPI spec: $SERVER_DIR/channels/api4/apidocs/openapi.yaml"

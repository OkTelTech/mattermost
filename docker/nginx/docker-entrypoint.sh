#!/bin/sh
set -e

# Process nginx config template with only BACKEND_HOST substitution
# This preserves all other nginx variables like $host, $uri, etc.
envsubst '${BACKEND_HOST}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# Start nginx
exec nginx -g 'daemon off;'

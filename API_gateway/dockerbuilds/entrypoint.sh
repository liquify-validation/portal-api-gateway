#!/bin/bash

# Perform any initialization tasks here
echo "Performing initialization tasks..."

# Substitute environment variables in nginx configuration template if needed
echo "Converting nginx.con.template, substituting env vars"
export DOLLAR="$"
envsubst < /usr/local/openresty/nginx/conf/nginx.conf.template > /usr/local/openresty/nginx/conf/nginx.conf

# Start OpenResty with the specified command
echo "Starting OpenResty..."
exec /usr/local/openresty/bin/openresty -g "daemon off;"
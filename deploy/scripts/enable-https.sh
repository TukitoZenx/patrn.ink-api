#!/usr/bin/env bash
# Install HTTPS Nginx config only after Let's Encrypt certificates exist.
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

if ! certs_exist; then
  echo "Certificates are not present yet. Leaving HTTP-only Nginx config in place."
  echo "Point DNS at this host, then run scripts/bootstrap-certs.sh"
  exit 0
fi

install -m 0644 \
  "${DEPLOY_ROOT}/nginx/templates/http-redirect.conf" \
  "${DEPLOY_ROOT}/nginx/conf.d/http.conf"

install -m 0644 \
  "${DEPLOY_ROOT}/nginx/templates/https.conf" \
  "${DEPLOY_ROOT}/nginx/conf.d/https.conf"

if docker inspect patrn-nginx >/dev/null 2>&1; then
  docker exec patrn-nginx nginx -t
  docker exec patrn-nginx nginx -s reload
  echo "HTTPS Nginx configuration enabled and reloaded."
else
  echo "HTTPS files installed. Nginx is not running yet; it will load them on start."
fi

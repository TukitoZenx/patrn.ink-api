#!/usr/bin/env bash
# Renew Let's Encrypt certificates and reload Nginx when needed.
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

docker run --rm \
  -v patrn_certbot-www:/var/www/certbot \
  -v patrn_certbot-etc:/etc/letsencrypt \
  certbot/certbot \
  renew --webroot -w /var/www/certbot --quiet

if docker inspect patrn-nginx >/dev/null 2>&1; then
  docker exec patrn-nginx nginx -s reload
fi

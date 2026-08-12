#!/usr/bin/env bash
# Obtain Let's Encrypt certificates using the HTTP Nginx config (webroot).
# Safe to run on a fresh instance after DNS A records point at the Elastic IP.
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
load_env
require_vars CERTBOT_EMAIL

if [[ ! -f "${DEPLOY_ROOT}/nginx/conf.d/http.conf" ]]; then
  echo "nginx/conf.d/http.conf is missing" >&2
  exit 1
fi

if docker inspect patrn-nginx >/dev/null 2>&1; then
  echo "Nginx is already running."
else
  echo "Starting the stack so Nginx can serve the ACME challenge..."
  compose up -d
fi

echo "Requesting certificates for patrn.ink and api.patrn.ink..."
docker run --rm \
  -v patrn_certbot-www:/var/www/certbot \
  -v patrn_certbot-etc:/etc/letsencrypt \
  certbot/certbot \
  certonly \
  --webroot \
  -w /var/www/certbot \
  -d patrn.ink \
  -d api.patrn.ink \
  --email "${CERTBOT_EMAIL}" \
  --agree-tos \
  --non-interactive \
  --keep-until-expiring

"${DEPLOY_ROOT}/scripts/enable-https.sh"
echo "Certificate bootstrap complete."

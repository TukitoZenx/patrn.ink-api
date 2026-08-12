#!/usr/bin/env bash
# Verify containers and the API /health endpoint. Used after deploy and by operators.
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

fail=0

for name in patrn-api patrn-ui patrn-nginx; do
  if ! docker inspect --format '{{.State.Running}}' "${name}" 2>/dev/null | grep -q true; then
    echo "FAIL  ${name} is not running"
    fail=1
  else
    echo "OK    ${name} is running"
  fi
done

if ! docker exec patrn-api wget -qO- http://127.0.0.1:8080/health >/tmp/patrn-health.json; then
  echo "FAIL  API /health did not return HTTP 200"
  docker exec patrn-api wget -qO- http://127.0.0.1:8080/health || true
  fail=1
else
  echo "OK    API /health"
  cat /tmp/patrn-health.json
  echo
fi

if docker exec patrn-ui node -e "fetch('http://127.0.0.1:3000/').then((r)=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"; then
  echo "OK    UI responds on :3000"
else
  echo "FAIL  UI did not respond on :3000"
  fail=1
fi

if command -v curl >/dev/null 2>&1; then
  if curl -fsS -H "Host: api.patrn.ink" http://127.0.0.1/health >/dev/null; then
    echo "OK    Nginx HTTP proxy to API /health"
  elif curl -fsk -H "Host: api.patrn.ink" https://127.0.0.1/health >/dev/null; then
    echo "OK    Nginx HTTPS proxy to API /health"
  else
    echo "FAIL  Nginx is not proxying /health for api.patrn.ink"
    fail=1
  fi
else
  echo "SKIP  curl not installed; skipped Nginx host-header check"
fi

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "All health checks passed."

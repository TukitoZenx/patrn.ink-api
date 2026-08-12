# Shared helpers. Sourced by the other scripts. Not executed directly.

DEPLOY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${DEPLOY_ROOT}/docker-compose.prod.yml"
ENV_FILE="${DEPLOY_ROOT}/.env"

require_env_file() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    echo "Missing ${ENV_FILE}" >&2
    echo "Copy env.prod.example to .env and fill in real values." >&2
    exit 1
  fi
}

load_env() {
  require_env_file
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
}

require_vars() {
  local missing=0
  local name
  for name in "$@"; do
    if [[ -z "${!name:-}" ]]; then
      echo "Required variable ${name} is empty" >&2
      missing=1
    fi
  done
  if [[ "${missing}" -ne 0 ]]; then
    exit 1
  fi
}

compose() {
  docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
}

certs_exist() {
  docker volume inspect patrn_certbot-etc >/dev/null 2>&1 || return 1
  docker run --rm \
    -v patrn_certbot-etc:/etc/letsencrypt \
    alpine:3.20 \
    test -f /etc/letsencrypt/live/patrn.ink/fullchain.pem
}

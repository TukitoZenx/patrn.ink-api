#!/usr/bin/env bash
# Pull tagged images and update the production Compose stack.
# Optional overrides from the environment (used by SSM / rollback):
#   API_IMAGE_TAG=...
#   UI_IMAGE_TAG=...
set -euo pipefail

# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

# Capture tags from SSM / rollback before .env overwrites them.
INCOMING_API_TAG="${API_IMAGE_TAG:-}"
INCOMING_UI_TAG="${UI_IMAGE_TAG:-}"

load_env

if [[ -n "${INCOMING_API_TAG}" ]]; then
  API_IMAGE_TAG="${INCOMING_API_TAG}"
fi
if [[ -n "${INCOMING_UI_TAG}" ]]; then
  UI_IMAGE_TAG="${INCOMING_UI_TAG}"
fi

require_vars \
  AWS_REGION \
  ECR_REGISTRY \
  ECR_API_REPOSITORY \
  ECR_UI_REPOSITORY \
  API_IMAGE_TAG \
  UI_IMAGE_TAG \
  JWT_SECRET \
  REDIS_ADDR \
  REDIS_USERNAME \
  REDIS_PASSWORD \
  GOOGLE_CLIENT_ID \
  GOOGLE_CLIENT_SECRET

if [[ "${JWT_SECRET}" == "dev-secret-key" ]]; then
  echo "JWT_SECRET must not be the development default in production" >&2
  exit 1
fi

# Persist tag overrides from SSM / rollback into .env so the next restart is consistent.
update_env_var() {
  local key="$1"
  local value="$2"
  if grep -q "^${key}=" "${ENV_FILE}"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
  else
    printf '\n%s=%s\n' "${key}" "${value}" >> "${ENV_FILE}"
  fi
}

update_env_var API_IMAGE_TAG "${API_IMAGE_TAG}"
update_env_var UI_IMAGE_TAG "${UI_IMAGE_TAG}"
load_env

echo "Logging in to ECR ${ECR_REGISTRY}..."
aws ecr get-login-password --region "${AWS_REGION}" \
  | docker login --username AWS --password-stdin "${ECR_REGISTRY}"

echo "Pulling images (api=${API_IMAGE_TAG} ui=${UI_IMAGE_TAG})..."
compose pull

if certs_exist; then
  "${DEPLOY_ROOT}/scripts/enable-https.sh"
else
  echo "No TLS certificates yet; Nginx will stay on HTTP until bootstrap-certs.sh runs."
fi

echo "Starting stack..."
compose up -d --remove-orphans

echo "Waiting for API health..."
healthy=0
for _ in $(seq 1 40); do
  if docker exec patrn-api wget -qO- http://127.0.0.1:8080/health >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 3
done

if [[ "${healthy}" -ne 1 ]]; then
  echo "API did not become healthy in time. Recent logs:" >&2
  compose logs --tail 80 api || true
  exit 1
fi

"${DEPLOY_ROOT}/scripts/healthcheck.sh"
echo "Deploy finished (api=${API_IMAGE_TAG} ui=${UI_IMAGE_TAG})."

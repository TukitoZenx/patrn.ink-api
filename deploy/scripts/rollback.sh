#!/usr/bin/env bash
# Roll back one service to a previously pushed image tag.
# Usage:
#   ./scripts/rollback.sh api v0.1.0
#   ./scripts/rollback.sh ui  a1b2c3d
set -euo pipefail

SERVICE="${1:-}"
TAG="${2:-}"

if [[ -z "${SERVICE}" || -z "${TAG}" ]]; then
  echo "Usage: $0 <api|ui> <image-tag>" >&2
  exit 1
fi

case "${SERVICE}" in
  api)
    export API_IMAGE_TAG="${TAG}"
    ;;
  ui)
    export UI_IMAGE_TAG="${TAG}"
    ;;
  *)
    echo "Service must be api or ui" >&2
    exit 1
    ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${SCRIPT_DIR}/deploy.sh"

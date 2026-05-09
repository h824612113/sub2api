#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BRANCH="codex/commercial-relay-mvp"
IMAGE="sub2api-commercial:codex-commercial-relay-mvp"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"

cd "${REPO_DIR}"

current_branch="$(git branch --show-current)"
if [[ "${current_branch}" != "${BRANCH}" ]]; then
  echo "Refusing to deploy: current branch is '${current_branch}', expected '${BRANCH}'." >&2
  exit 1
fi

if ! grep -q "image: ${IMAGE}" "${COMPOSE_FILE}"; then
  echo "Refusing to deploy: ${COMPOSE_FILE} is not pinned to ${IMAGE}." >&2
  exit 1
fi

if ! grep -q "./postgres_data:/var/lib/postgresql/data" "${COMPOSE_FILE}"; then
  echo "Refusing to deploy: postgres bind mount is missing." >&2
  exit 1
fi

docker compose -f "${COMPOSE_FILE}" up -d --force-recreate

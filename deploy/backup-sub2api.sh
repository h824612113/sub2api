#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${SUB2API_APP_DIR:-/root/sub2api}"
DEPLOY_DIR="${SUB2API_DEPLOY_DIR:-${APP_DIR}/deploy}"
BACKUP_ROOT="${SUB2API_BACKUP_DIR:-/root/sub2api-backups}"
RETENTION_DAYS="${SUB2API_BACKUP_RETENTION_DAYS:-14}"
POSTGRES_CONTAINER="${SUB2API_POSTGRES_CONTAINER:-sub2api-postgres}"
DB_USER="${SUB2API_DB_USER:-sub2api}"
DB_NAME="${SUB2API_DB_NAME:-sub2api}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="${BACKUP_ROOT}/${timestamp}"
tmp_dir="${backup_dir}.tmp"

mkdir -p "${BACKUP_ROOT}"
chmod 700 "${BACKUP_ROOT}"
rm -rf "${tmp_dir}"
mkdir -p "${tmp_dir}"
chmod 700 "${tmp_dir}"

cleanup() {
  if [[ -d "${tmp_dir}" ]]; then
    rm -rf "${tmp_dir}"
  fi
}
trap cleanup EXIT

if [[ "$(docker inspect -f '{{.State.Running}}' "${POSTGRES_CONTAINER}")" != "true" ]]; then
  echo "Postgres container ${POSTGRES_CONTAINER} is not running" >&2
  exit 1
fi

docker exec "${POSTGRES_CONTAINER}" pg_dump -U "${DB_USER}" -d "${DB_NAME}" -Fc > "${tmp_dir}/sub2api.dump"
docker exec "${POSTGRES_CONTAINER}" pg_dumpall -U "${DB_USER}" --globals-only > "${tmp_dir}/globals.sql"

docker exec "${POSTGRES_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -Atc \
  "select 'users=' || count(*) from users
   union all select 'api_keys=' || count(*) from api_keys
   union all select 'accounts=' || count(*) from accounts
   union all select 'user_subscriptions=' || count(*) from user_subscriptions
   union all select 'groups=' || count(*) from groups
   union all select 'redeem_codes=' || count(*) from redeem_codes
   union all select 'usage_logs=' || count(*) from usage_logs
   order by 1;" > "${tmp_dir}/table-counts.txt"

tar -C "${DEPLOY_DIR}" \
  -czf "${tmp_dir}/deploy-config.tgz" \
  .env \
  docker-compose.yml \
  data/config.yaml \
  data/pages \
  2>/dev/null || true

docker inspect sub2api "${POSTGRES_CONTAINER}" sub2api-redis > "${tmp_dir}/docker-inspect.json"
docker compose -f "${DEPLOY_DIR}/docker-compose.yml" config > "${tmp_dir}/docker-compose-rendered.yml"

docker run --rm -i postgres:18-alpine pg_restore -l > /dev/null < "${tmp_dir}/sub2api.dump"

(
  cd "${tmp_dir}"
  sha256sum deploy-config.tgz docker-compose-rendered.yml docker-inspect.json \
    globals.sql sub2api.dump table-counts.txt > SHA256SUMS
)
mv "${tmp_dir}" "${backup_dir}"
trap - EXIT

find "${BACKUP_ROOT}" -mindepth 1 -maxdepth 1 -type d -name '20??????T??????Z' -mtime +"${RETENTION_DAYS}" -exec rm -rf {} +

echo "Backup created: ${backup_dir}"

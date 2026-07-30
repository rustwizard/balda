#!/usr/bin/env bash
# Daily Postgres backup for the balda prod stack.
# Keeps the last 14 dumps in ./backups. Install into cron, e.g.:
#   17 3 * * * /opt/balda/scripts/backup.sh >> /var/log/balda-backup.log 2>&1
set -euo pipefail

BACKUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/backups"
KEEP_DAYS=14

mkdir -p "$BACKUP_DIR"
stamp="$(date +%Y%m%d-%H%M%S)"
out="$BACKUP_DIR/balda-$stamp.sql.gz"

docker compose -f "$(dirname "$BACKUP_DIR")/docker-compose.prod.yml" \
  exec -T pg pg_dump -U balda -d balda | gzip > "$out"

find "$BACKUP_DIR" -name 'balda-*.sql.gz' -mtime "+$KEEP_DAYS" -delete
echo "$(date +%Y-%m-%dT%H:%M:%S%z) backup written: $out ($(du -h "$out" | cut -f1))"

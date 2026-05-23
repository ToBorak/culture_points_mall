#!/usr/bin/env bash
# 重置数据库：drop → create → migrate → seed
# 每次执行后得到一个干净的 demo 数据库

set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

GREEN=$'\033[32m'; BLUE=$'\033[36m'; RESET=$'\033[0m'
log() { printf '%s[%s] %s%s\n' "$BLUE" "$(date +%H:%M:%S)" "$*" "$RESET"; }
ok()  { printf '%s✓ %s%s\n' "$GREEN" "$*" "$RESET"; }

MYSQL_CONTAINER="${MYSQL_CONTAINER:-cpm-mysql}"
MYSQL_DB="${MYSQL_DB:-cpm}"

wait_mysql() {
  log "等 MySQL 就绪…"
  for i in {1..60}; do
    if docker exec "$MYSQL_CONTAINER" mysqladmin ping -uroot -proot --silent >/dev/null 2>&1; then
      ok "MySQL 已就绪"
      return
    fi
    sleep 1
  done
  echo "MySQL 超时未就绪" >&2
  exit 1
}

drop_create_db() {
  log "Drop + Create database \`$MYSQL_DB\`…"
  docker exec "$MYSQL_CONTAINER" mysql -uroot -proot -e \
    "DROP DATABASE IF EXISTS \`$MYSQL_DB\`; CREATE DATABASE \`$MYSQL_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
  ok "Database 已重置"
}

run_migrate() {
  log "应用 migrations…"
  go run ./cmd/migrate -action=up
  ok "Migrations 已应用"
}

run_seed() {
  log "灌入 seed 数据…"
  go run ./cmd/migrate -action=seed
  ok "Seed 完成"
}

main() {
  wait_mysql
  drop_create_db
  run_migrate
  run_seed
  ok "=== 数据库已重置完成 ==="
}

main "$@"

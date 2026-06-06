#!/usr/bin/env bash
# 启动后端 + MCP + 前端 H5 + 前端 Admin
# 前端仓库默认从 ../culture-points-mall-web 读取
# 用环境变量 FRONTEND_REPO 可指定其它路径
# 用环境变量 FRONTEND_GIT 可指定首次克隆的远程地址

set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

GREEN=$'\033[32m'; YELLOW=$'\033[33m'; BLUE=$'\033[36m'; RED=$'\033[31m'; RESET=$'\033[0m'
log() { printf '%s[%s] %s%s\n' "$BLUE" "$(date +%H:%M:%S)" "$*" "$RESET"; }
ok()  { printf '%s✓ %s%s\n' "$GREEN" "$*" "$RESET"; }
warn(){ printf '%s⚠ %s%s\n' "$YELLOW" "$*" "$RESET"; }
err() { printf '%s✗ %s%s\n' "$RED" "$*" "$RESET" >&2; }

readonly LOG_DIR="$REPO_ROOT/.logs"
mkdir -p "$LOG_DIR"

FRONTEND_REPO="${FRONTEND_REPO:-$(cd "$REPO_ROOT/.." && pwd)/culture_points_mall_web}"
FRONTEND_GIT="${FRONTEND_GIT:-}"

ensure_frontend_repo() {
  if [[ -d "$FRONTEND_REPO/.git" ]]; then
    ok "前端仓库已存在：$FRONTEND_REPO"
    return
  fi
  if [[ -z "$FRONTEND_GIT" ]]; then
    err "前端仓库未找到：$FRONTEND_REPO"
    err "请克隆前端仓库到该路径，或设置环境变量 FRONTEND_GIT 让脚本帮你克隆，例如："
    err "  FRONTEND_GIT=git@github.com:YOUR/culture-points-mall-web.git ./scripts/start-services.sh"
    exit 1
  fi
  log "克隆前端仓库到 $FRONTEND_REPO …"
  git clone "$FRONTEND_GIT" "$FRONTEND_REPO"
  ok "前端仓库克隆完成"
}

install_frontend_deps() {
  log "安装前端依赖（pnpm install）…"
  (cd "$FRONTEND_REPO" && pnpm install --prefer-frozen-lockfile) >"$LOG_DIR/frontend-install.log" 2>&1 || {
    err "pnpm install 失败，详见 $LOG_DIR/frontend-install.log"
    exit 1
  }
  ok "前端依赖安装完成"
}

# kill_port <port>
kill_port() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    local pid
    pid="$(lsof -t -i ":${port}" 2>/dev/null || true)"
    [[ -n "$pid" ]] && kill -9 "$pid" 2>/dev/null || true
  fi
}

# spawn <name> <log-file> <cmd...>
spawn() {
  local name="$1"; local log="$2"; shift 2
  log "启动 $name → $log"
  ( "$@" ) >"$log" 2>&1 &
  echo "$!" > "$LOG_DIR/$name.pid"
}

start_backend() {
  kill_port 18080
  spawn backend "$LOG_DIR/backend.log" go run ./cmd/server
}

start_mcp() {
  kill_port 8090
  spawn mcp "$LOG_DIR/mcp.log" go run ./cmd/mcp
}

start_frontend() {
  kill_port 5173
  kill_port 5174
  spawn frontend "$LOG_DIR/frontend.log" bash -c "cd '$FRONTEND_REPO' && pnpm dev"
}

wait_ready() {
  local url="$1" name="$2"
  for i in {1..60}; do
    if curl -sf -m 2 "$url" >/dev/null 2>&1; then
      ok "$name 已就绪：$url"
      return
    fi
    sleep 1
  done
  warn "$name 启动超时（$url），查看日志：$LOG_DIR"
}

# MCP 没有 /healthz，用未授权访问 /mcp/sse 返回 401 来判定服务在跑
wait_ready_mcp() {
  local url="$1"
  for i in {1..30}; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' -m 2 "$url" 2>/dev/null || echo 0)"
    if [[ "$code" == "401" ]]; then
      ok "MCP 已就绪：$url（401 鉴权符合预期）"
      return
    fi
    sleep 1
  done
  warn "MCP 启动超时（$url），查看日志：$LOG_DIR/mcp.log"
}

print_summary() {
  cat <<EOF

$GREEN==============================
  culture-points-mall 已启动
==============================$RESET

  后端 API     http://localhost:18080
  MCP 服务     http://localhost:8090/mcp/sse
  员工 H5      http://localhost:5173
  管理后台     http://localhost:5174   (User ID 1 即可登录)

  日志目录     $LOG_DIR
  停止服务     ./scripts/stop.sh   或   make down

EOF
}

open_browser() {
  if [[ "${OPEN_BROWSER:-1}" != "1" ]]; then return; fi
  if [[ "$(uname -s)" == "Darwin" ]]; then
    sleep 2
    open http://localhost:5173 >/dev/null 2>&1 || true
    open http://localhost:5174 >/dev/null 2>&1 || true
  fi
}

main() {
  ensure_frontend_repo
  install_frontend_deps
  start_backend
  start_mcp
  wait_ready "http://localhost:18080/healthz" backend
  wait_ready_mcp "http://localhost:8090/mcp/sse"
  start_frontend
  wait_ready "http://localhost:5173/" h5
  wait_ready "http://localhost:5174/" admin
  print_summary
  open_browser
}

main "$@"

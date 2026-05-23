#!/usr/bin/env bash
# 停止所有由 bootstrap 启动的后台进程
set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$REPO_ROOT/.logs"

stop_pid_file() {
  local f="$1"
  [[ -f "$f" ]] || return
  local pid
  pid="$(cat "$f")"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    sleep 0.3
    kill -9 "$pid" 2>/dev/null || true
    echo "stopped $(basename "$f" .pid) (pid $pid)"
  fi
  rm -f "$f"
}

for f in "$LOG_DIR"/*.pid; do
  [[ -f "$f" ]] && stop_pid_file "$f"
done

# 兜底：按端口杀掉残留
for port in 8080 8090 5173 5174; do
  if command -v lsof >/dev/null 2>&1; then
    pid="$(lsof -t -i ":${port}" 2>/dev/null || true)"
    [[ -n "$pid" ]] && kill -9 "$pid" 2>/dev/null && echo "killed leftover on :$port"
  fi
done

echo "all services stopped"

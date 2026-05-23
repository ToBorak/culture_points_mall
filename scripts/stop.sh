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

# 兜底 1：按端口杀掉残留
for port in 8080 8090 5173 5174; do
  if command -v lsof >/dev/null 2>&1; then
    pids="$(lsof -t -i ":${port}" 2>/dev/null || true)"
    for pid in $pids; do
      kill -9 "$pid" 2>/dev/null && echo "killed leftover on :$port (pid $pid)" || true
    done
  fi
done

# 兜底 2：按进程名杀掉 cpm_server / cpm_mcp 老二进制（来源 /tmp/cpm_server 等手动启的）
for pattern in cpm_server cpm_mcp; do
  pids="$(pgrep -f "$pattern" 2>/dev/null || true)"
  for pid in $pids; do
    kill -9 "$pid" 2>/dev/null && echo "killed orphan $pattern (pid $pid)" || true
  done
done

echo "all services stopped"

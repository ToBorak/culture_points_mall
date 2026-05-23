#!/usr/bin/env bash
# culture-points-mall 一键启动入口
#
#   ./bootstrap.sh            # 默认：装依赖 → 起容器 → 重置数据库 → 启动后端+前端
#   ./bootstrap.sh --no-reset # 保留旧数据
#   ./bootstrap.sh --no-open  # 不自动打开浏览器
#
# 环境变量
#   FRONTEND_REPO  前端仓库本地路径，默认 ../culture-points-mall-web
#   FRONTEND_GIT   首次克隆前端时使用的 git 地址（缺路径时必填）

set -euo pipefail

readonly REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_ROOT"

GREEN=$'\033[32m'; YELLOW=$'\033[33m'; BLUE=$'\033[36m'; RESET=$'\033[0m'
log() { printf '%s[%s] %s%s\n' "$BLUE" "$(date +%H:%M:%S)" "$*" "$RESET"; }
ok()  { printf '%s✓ %s%s\n' "$GREEN" "$*" "$RESET"; }
warn(){ printf '%s⚠ %s%s\n' "$YELLOW" "$*" "$RESET"; }

NO_RESET=0
NO_OPEN=0
for arg in "$@"; do
  case "$arg" in
    --no-reset) NO_RESET=1 ;;
    --no-open)  NO_OPEN=1; export OPEN_BROWSER=0 ;;
    -h|--help)
      sed -n '1,15p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) warn "未知参数：$arg" ;;
  esac
done

ensure_config_yaml() {
  if [[ -f "$REPO_ROOT/configs/config.yaml" ]]; then
    ok "configs/config.yaml 已存在"
    return
  fi
  log "复制 configs/config.example.yaml → configs/config.yaml"
  cp "$REPO_ROOT/configs/config.example.yaml" "$REPO_ROOT/configs/config.yaml"
  warn "请在 configs/config.yaml 中填入你的 LLM API Key（DEEPSEEK_API_KEY / ANTHROPIC_API_KEY）"
}

start_docker_services() {
  log "启动 MySQL + Redis 容器…"
  docker compose up -d
  ok "容器已启动"
}

main() {
  log "=== culture-points-mall bootstrap ==="
  bash "$REPO_ROOT/scripts/ensure-deps.sh"
  ensure_config_yaml
  start_docker_services
  if [[ "$NO_RESET" -eq 0 ]]; then
    bash "$REPO_ROOT/scripts/reset-db.sh"
  else
    warn "跳过数据库重置（--no-reset）"
  fi
  bash "$REPO_ROOT/scripts/start-services.sh"
}

main

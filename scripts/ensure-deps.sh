#!/usr/bin/env bash
# 检测并安装运行 culture-points-mall 所需的本地依赖
# 自动适配 macOS（Homebrew）与 Linux（apt）

set -euo pipefail

readonly RED=$'\033[31m'
readonly GREEN=$'\033[32m'
readonly YELLOW=$'\033[33m'
readonly BLUE=$'\033[36m'
readonly RESET=$'\033[0m'

log() { printf '%s[%s] %s%s\n' "$BLUE" "$(date +%H:%M:%S)" "$*" "$RESET"; }
ok()  { printf '%s✓ %s%s\n' "$GREEN" "$*" "$RESET"; }
warn(){ printf '%s⚠ %s%s\n' "$YELLOW" "$*" "$RESET"; }
err() { printf '%s✗ %s%s\n' "$RED" "$*" "$RESET" >&2; }

OS_TYPE="$(uname -s)"

install_homebrew_if_needed() {
  if command -v brew >/dev/null 2>&1; then
    ok "Homebrew 已安装"
    return
  fi
  if [[ "$OS_TYPE" != "Darwin" ]]; then return; fi
  log "安装 Homebrew（首次需 5-10 分钟）…"
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  # Apple Silicon 路径
  if [[ -d "/opt/homebrew" ]]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
  fi
  ok "Homebrew 安装完成"
}

install_with_pkg_manager() {
  local pkg="$1"
  local cmd="${2:-$1}"
  if command -v "$cmd" >/dev/null 2>&1; then
    ok "$cmd 已安装 ($($cmd --version 2>&1 | head -n1))"
    return
  fi
  log "安装 $pkg …"
  if [[ "$OS_TYPE" == "Darwin" ]]; then
    brew install "$pkg"
  elif command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -y >/dev/null
    sudo apt-get install -y "$pkg"
  elif command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y "$pkg"
  else
    err "未识别的系统，请手动安装 $pkg"
    exit 1
  fi
  ok "$cmd 安装完成"
}

ensure_docker() {
  if command -v docker >/dev/null 2>&1; then
    ok "Docker 已安装"
    if ! docker info >/dev/null 2>&1; then
      warn "Docker daemon 未启动，请打开 Docker Desktop 后再次运行 bootstrap"
      exit 1
    fi
    return
  fi
  log "未检测到 Docker"
  if [[ "$OS_TYPE" == "Darwin" ]]; then
    log "用 Homebrew Cask 安装 Docker Desktop（需要授权）…"
    brew install --cask docker
    warn "Docker Desktop 已安装，请手动启动它（应用程序 → Docker）后重新运行本脚本"
    exit 1
  fi
  # Linux
  if [[ -f /etc/debian_version ]]; then
    log "用 apt 安装 docker.io …"
    sudo apt-get update -y >/dev/null
    sudo apt-get install -y docker.io docker-compose-plugin
    sudo systemctl enable --now docker
    sudo usermod -aG docker "$USER" || true
    warn "已将当前用户加入 docker 组，请退出登录或重新打开终端后重试"
    exit 1
  fi
  err "请手动安装 Docker：https://docs.docker.com/get-docker/"
  exit 1
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    local v
    v="$(go version | awk '{print $3}' | sed 's/go//')"
    ok "Go $v 已安装"
    # check >= 1.22
    if ! awk -v v="$v" 'BEGIN{split(v,a,"."); if(a[1]<1 || (a[1]==1 && a[2]<22)) exit 1}' ; then
      warn "Go 版本过低（需要 ≥ 1.22），建议升级"
    fi
    return
  fi
  install_with_pkg_manager go
}

ensure_node() {
  if command -v node >/dev/null 2>&1; then
    ok "Node $(node --version) 已安装"
  else
    install_with_pkg_manager node
  fi
}

ensure_pnpm() {
  if command -v pnpm >/dev/null 2>&1; then
    ok "pnpm $(pnpm --version) 已安装"
    return
  fi
  log "通过 corepack 启用 pnpm …"
  if command -v corepack >/dev/null 2>&1; then
    corepack enable
    corepack prepare pnpm@latest --activate
    ok "pnpm 启用完成"
    return
  fi
  npm install -g pnpm
  ok "pnpm 全局安装完成"
}

main() {
  log "=== 检查并安装依赖 ==="
  install_homebrew_if_needed
  ensure_docker
  ensure_go
  ensure_node
  ensure_pnpm
  ok "依赖全部就绪"
}

main "$@"

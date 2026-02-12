#!/bin/bash
set -e

# s-ui install script - curl | bash deployment
# Usage: install | update | uninstall

[[ $EUID -ne 0 ]] && echo "请使用 root 运行此脚本" && exit 1

GITHUB_REPO="${GITHUB_REPO:-HodgeWen/proxy-ui}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/s-ui}"
SERVICE_NAME="s-ui"

arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
  esac
}

get_latest_tag() {
  curl -sfL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | \
    grep '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
}

download_binary() {
  local tag="$1"
  local dest="$2"
  local url="https://github.com/${GITHUB_REPO}/releases/download/${tag}/s-ui-linux-$(arch)"
  echo "下载: $url"
  curl -fsSL "$url" -o "$dest"
}

write_systemd_unit() {
  local port="$1"
  cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=s-ui Panel
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/s-ui
WorkingDirectory=${INSTALL_DIR}
Restart=on-failure
RestartSec=5
Environment=CONFIG_PATH=${INSTALL_DIR}/config.json
Environment=DATA_DIR=${INSTALL_DIR}/data
Environment=ADDR=:${port}

[Install]
WantedBy=multi-user.target
EOF
}

do_install() {
  echo "=== s-ui 安装 ==="
  local tag
  tag=$(get_latest_tag) || { echo "获取最新版本失败" >&2; exit 1; }
  echo "版本: $tag"

  mkdir -p "$INSTALL_DIR"
  mkdir -p "${INSTALL_DIR}/data"

  local tmp_bin="/tmp/s-ui-$$"
  download_binary "$tag" "$tmp_bin"
  mv "$tmp_bin" "${INSTALL_DIR}/s-ui"
  chmod +x "${INSTALL_DIR}/s-ui"

  # Port prompt
  local port="8080"
  local config_file="${INSTALL_DIR}/config.json"
  if [[ -f "$config_file" ]]; then
    local existing
    existing=$(grep -oE '"addr"[[:space:]]*:[[:space:]]*":[0-9]+"' "$config_file" 2>/dev/null | grep -oE '[0-9]+' | head -1)
    [[ -n "$existing" ]] && port="$existing"
  fi
  read -p "监听端口 [${port}]: " input
  port="${input:-$port}"
  if ! [[ "$port" =~ ^[0-9]+$ ]]; then
    echo "端口必须是数字" >&2
    exit 1
  fi
  if command -v ss &>/dev/null; then
    if ss -ltn 2>/dev/null | grep -q ":${port} "; then
      echo "端口 ${port} 已被占用" >&2
      exit 1
    fi
  fi

  # Config
  if [[ ! -f "$config_file" ]]; then
    local secret
    secret=$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32)
    cat > "$config_file" << EOF
{
  "addr": ":${port}",
  "session_secret": "${secret}",
  "data_dir": "${INSTALL_DIR}/data",
  "singbox_config_path": "${INSTALL_DIR}/sing-box.json",
  "singbox_binary_path": ""
}
EOF
  fi

  write_systemd_unit "$port"

  systemctl daemon-reload
  systemctl enable ${SERVICE_NAME}
  systemctl start ${SERVICE_NAME}

  local ip
  ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "YOUR_IP")
  echo ""
  echo "安装完成！"
  echo "访问 http://${ip}:${port}/setup 设置管理员密码"
  echo "服务状态: systemctl status ${SERVICE_NAME}"
}

do_update() {
  echo "=== s-ui 更新 ==="
  [[ ! -f "${INSTALL_DIR}/s-ui" ]] && { echo "s-ui 未安装" >&2; exit 1; }
  local tag
  tag=$(get_latest_tag) || { echo "获取最新版本失败" >&2; exit 1; }
  echo "版本: $tag"
  systemctl stop ${SERVICE_NAME} 2>/dev/null || true
  local tmp_bin="/tmp/s-ui-$$"
  download_binary "$tag" "$tmp_bin"
  mv "$tmp_bin" "${INSTALL_DIR}/s-ui"
  chmod +x "${INSTALL_DIR}/s-ui"
  systemctl start ${SERVICE_NAME}
  echo "更新完成"
}

do_uninstall() {
  echo "=== s-ui 卸载 ==="
  systemctl stop ${SERVICE_NAME} 2>/dev/null || true
  systemctl disable ${SERVICE_NAME} 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload
  if [[ -d "$INSTALL_DIR" ]]; then
    read -p "是否删除数据目录 ${INSTALL_DIR}? (y/N): " confirm
    if [[ "${confirm}" =~ ^[yY]$ ]]; then
      rm -rf "$INSTALL_DIR"
      echo "已删除 $INSTALL_DIR"
    else
      echo "保留数据目录 $INSTALL_DIR"
    fi
  fi
  echo "卸载完成"
}

case "${1:-install}" in
  install) do_install ;;
  update)  do_update ;;
  uninstall) do_uninstall ;;
  *) echo "用法: $0 install | update | uninstall" >&2; exit 1 ;;
esac

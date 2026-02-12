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

  write_systemd_unit "8080"

  systemctl daemon-reload
  systemctl enable ${SERVICE_NAME}
  systemctl start ${SERVICE_NAME}

  echo ""
  echo "安装完成！"
  echo "访问 http://YOUR_IP:8080/setup 设置管理员密码"
  echo "服务状态: systemctl status ${SERVICE_NAME}"
}

do_update() {
  echo "update 子命令请运行 Task 2 完成"
  exit 1
}

do_uninstall() {
  echo "uninstall 子命令请运行 Task 2 完成"
  exit 1
}

case "${1:-install}" in
  install) do_install ;;
  update)  do_update ;;
  uninstall) do_uninstall ;;
  *) echo "用法: $0 install | update | uninstall" >&2; exit 1 ;;
esac

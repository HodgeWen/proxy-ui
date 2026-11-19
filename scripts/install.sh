#!/usr/bin/env bash
# proxy-ui linux systemd 安装脚本：安装/升级二进制并注册开机自启服务，支持卸载与状态查看。
#
# 用法：
#   sudo ./scripts/install.sh install [二进制路径]   # 默认取脚本同目录下的 proxy-ui-linux-amd64
#   sudo ./scripts/install.sh status
#   sudo ./scripts/install.sh uninstall              # 默认保留数据目录；PURGE_DATA=1 时一并删除
#
# 可用环境变量覆盖默认值：
#   PORT      监听端口（默认 8080）
#   DATA_DIR  数据目录（默认 /var/lib/proxy-ui，存数据库、内核、证书等）
#   PURGE_DATA=1 卸载时连同数据目录一起删除
set -euo pipefail

SERVICE_NAME=proxy-ui
INSTALL_BIN=/usr/local/bin/proxy-ui
DEFAULT_DATA_DIR=/var/lib/proxy-ui
UNIT_FILE=/etc/systemd/system/${SERVICE_NAME}.service
DATA_DIR=${DATA_DIR:-${DEFAULT_DATA_DIR}}
PORT=${PORT:-8080}

die() {
    echo "错误：$*" >&2
    exit 1
}

require_root() {
    [ "$(id -u)" -eq 0 ] || die "请以 root 运行（sudo $0 $*）"
}

require_systemd() {
    command -v systemctl >/dev/null 2>&1 || die "未找到 systemctl，需要 systemd 环境"
}

write_unit() {
    cat >"${UNIT_FILE}" <<EOF
[Unit]
Description=proxy-ui sing-box panel
Documentation=https://github.com/HodgeWen/proxy-ui
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_BIN} -addr :${PORT} -data ${DATA_DIR}
WorkingDirectory=${DATA_DIR}
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

do_install() {
    local bin=${1:-"$(dirname "$0")/proxy-ui-linux-amd64"}
    [ -f "${bin}" ] || die "二进制不存在：${bin}（传入路径或先下载 release 产物）"
    [ -x "${bin}" ] || die "二进制不可执行：${bin}"

    echo "安装二进制 ${bin} -> ${INSTALL_BIN}"
    install -m 0755 "${bin}" "${INSTALL_BIN}"

    echo "创建数据目录 ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"

    echo "写入 systemd 服务 ${UNIT_FILE}"
    write_unit

    systemctl daemon-reload
    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        echo "服务已存在，重启以应用新版本"
        systemctl restart "${SERVICE_NAME}"
    else
        systemctl enable --now "${SERVICE_NAME}"
    fi

    echo "安装完成：服务已启动并设置开机自启"
    echo "  面板地址：http://<服务器IP>:${PORT}（数据目录 ${DATA_DIR}）"
    systemctl --no-pager status "${SERVICE_NAME}" || true
}

do_uninstall() {
    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    rm -f "${UNIT_FILE}" "${INSTALL_BIN}"
    systemctl daemon-reload
    if [ "${PURGE_DATA:-0}" = "1" ]; then
        rm -rf "${DATA_DIR}"
        echo "已卸载服务并删除数据目录 ${DATA_DIR}"
    else
        echo "已卸载服务（数据目录 ${DATA_DIR} 已保留，PURGE_DATA=1 可一并删除）"
    fi
}

do_status() {
    systemctl --no-pager status "${SERVICE_NAME}" || true
}

main() {
    local cmd=${1:-install}
    shift || true
    case "${cmd}" in
    install)
        require_root "$@"
        require_systemd
        do_install "$@"
        ;;
    status)
        require_systemd
        do_status
        ;;
    uninstall)
        require_root "$@"
        require_systemd
        do_uninstall
        ;;
    *)
        die "未知命令：${cmd}（支持 install / status / uninstall）"
        ;;
    esac
}

main "$@"

#!/usr/bin/env bash
#
# Linux Helper — 单文件版
# 用法:
#   bash install.sh             进入管理菜单
#   bash install.sh --install   安装到系统
#   bash install.sh --uninstall 卸载
#
set -euo pipefail

# ============================================================
# 颜色变量
# ============================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}   $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; }

# ============================================================
# 通用函数
# ============================================================
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "此脚本需要 root 权限，请使用 sudo 运行。"
        exit 1
    fi
}

confirm() {
    local prompt="${1:-确认继续？}"
    local reply
    read -p "$prompt [y/N]: " -r reply
    [[ $reply =~ ^[Yy] ]]
}

header() {
    local title="$*"
    local width=50
    local pad=$(((width - ${#title}) / 2))
    echo ""
    printf "${CYAN}%s${NC}\n" "$(printf '=%.0s' $(seq 1 "$width"))"
    printf "${CYAN}%*s%s%*s${NC}\n" $pad '' "$title" $pad ''
    printf "${CYAN}%s${NC}\n" "$(printf '=%.0s' $(seq 1 "$width"))"
    echo ""
}

placeholder() {
    echo ""
    echo -e "${YELLOW}━━━ [待开发] ━━━${NC}"
    echo -e "${YELLOW}  功能：${NC}$*"
    echo -e "${YELLOW}  此功能尚未实现，敬请期待。${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# ============================================================
# 模块：网络优化
# ============================================================

# BBR + bpftune 一键开启
enable_bbr() {
    echo ""
    header "开启 BBR + fq + bpftune"

    # 1. 备份旧的 sysctl 配置
    local backup_dir="/etc/linux-helper/backups"
    local backup_file="$backup_dir/sysctl-backup-$(date +%Y%m%d-%H%M%S).conf"
    mkdir -p "$backup_dir"

    info "备份当前 sysctl 配置到 $backup_file ..."
    {
        echo "# Linux Helper 备份 - $(date)"
        sysctl net.core.default_qdisc 2>/dev/null || true
        sysctl net.ipv4.tcp_congestion_control 2>/dev/null || true
        sysctl net.ipv4.tcp_available_congestion_control 2>/dev/null || true
    } > "$backup_file"
    success "备份完成"

    # 2. 清理 sysctl.d 中已有的 BBR / congestion 相关配置
    info "清理已有的 BBR / congestion 配置..."
    local cleaned=0
    for f in /etc/sysctl.d/*.conf /etc/sysctl.conf; do
        [[ -f "$f" ]] || continue
        if grep -qE 'tcp_congestion_control|default_qdisc' "$f" 2>/dev/null; then
            cp "$f" "$backup_dir/$(basename "$f").bak.$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
            sed -i '/tcp_congestion_control/d; /default_qdisc/d' "$f"
            info "  已清理: $f"
            cleaned=$((cleaned + 1))
        fi
    done
    [[ $cleaned -eq 0 ]] && info "  无需清理"

    # 3. 写入新的 BBR 配置
    info "写入 BBR 配置..."
    cat > /etc/sysctl.d/90-bbr.conf << 'EOF'
# BBR + fq — Linux Helper
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
EOF
    success "写入 /etc/sysctl.d/90-bbr.conf"

    # 4. 应用 sysctl
    info "应用 sysctl 参数..."
    sysctl -p /etc/sysctl.d/90-bbr.conf
    success "sysctl 参数已生效"

    # 5. 验证 BBR
    echo ""
    header "验证结果"
    local current_cc
    current_cc=$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null)
    local current_qdisc
    current_qdisc=$(sysctl -n net.core.default_qdisc 2>/dev/null)
    echo "  TCP 拥塞控制算法: ${GREEN}${current_cc:-N/A}${NC}"
    echo "  默认队列规则:     ${GREEN}${current_qdisc:-N/A}${NC}"

    if [[ "$current_cc" == "bbr" ]]; then
        success "BBR 已启用 ✓"
    else
        warn "BBR 似乎未生效，请检查内核版本 (需 4.9+)"
    fi

    # 6. 安装 bpftune
    echo ""
    header "安装 bpftune"
    if command -v bpftune &>/dev/null; then
        success "bpftune 已安装，跳过"
    else
        info "正在安装 bpftune..."
        local install_ok=0

        if command -v apt &>/dev/null; then
            apt update -qq && apt install -y bpftune && install_ok=1
        elif command -v dnf &>/dev/null; then
            dnf install -y bpftune && install_ok=1
        elif command -v yum &>/dev/null; then
            yum install -y epel-release && yum install -y bpftune && install_ok=1
        elif command -v zypper &>/dev/null; then
            zypper install -y bpftune && install_ok=1
        else
            warn "不支持的包管理器，请手动安装 bpftune"
        fi

        if [[ $install_ok -eq 1 ]]; then
            success "bpftune 安装成功"
        else
            warn "bpftune 安装失败，请检查源或手动安装"
        fi
    fi

    # 7. 启用 bpftune 服务
    if systemctl list-unit-files bpftune.service &>/dev/null; then
        info "启动 bpftune 服务..."
        systemctl enable --now bpftune 2>/dev/null || true
        if systemctl is-active --quiet bpftune; then
            success "bpftune 服务运行中 ✓"
        else
            warn "bpftune 服务未运行，请检查: systemctl status bpftune"
        fi
    else
        warn "未找到 bpftune 服务单元，可能需重启后生效"
    fi

    echo ""
    success "BBR 优化完成！"
}

network_menu() {
    while true; do
        clear; header "网络优化"
        echo "  1) 启用 BBR + fq + bpftune"
        echo "  2) TCP 参数调优"
        echo "  3) 查看网络状态"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) enable_bbr ;;
            2) placeholder "TCP 参数调优" ;;
            3) placeholder "查看网络状态" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统配置
# ============================================================
system_menu() {
    while true; do
        clear; header "系统配置"
        echo "  1) 设置时区"
        echo "  2) 修改主机名"
        echo "  3) 配置语言环境"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "设置时区" ;;
            2) placeholder "修改主机名" ;;
            3) placeholder "配置语言环境" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统安全
# ============================================================
security_menu() {
    while true; do
        clear; header "系统安全"
        echo "  1) SSH 安全配置"
        echo "  2) 配置防火墙"
        echo "  3) Fail2Ban 管理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "SSH 安全配置" ;;
            2) placeholder "配置防火墙" ;;
            3) placeholder "Fail2Ban 管理" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统调优
# ============================================================
tuning_menu() {
    while true; do
        clear; header "系统调优"
        echo "  1) Swap 管理"
        echo "  2) 内核参数优化"
        echo "  3) 文件描述符限制"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "Swap 管理" ;;
            2) placeholder "内核参数优化" ;;
            3) placeholder "文件描述符限制" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统信息
# ============================================================
info_menu() {
    while true; do
        clear; header "系统信息"
        echo "  1) 系统概览"
        echo "  2) CPU 信息"
        echo "  3) 内存信息"
        echo "  4) 磁盘信息"
        echo "  5) 网络信息"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "系统概览" ;;
            2) placeholder "CPU 信息" ;;
            3) placeholder "内存信息" ;;
            4) placeholder "磁盘信息" ;;
            5) placeholder "网络信息" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统维护
# ============================================================
maintenance_menu() {
    while true; do
        clear; header "系统维护"
        echo "  1) 系统更新与清理"
        echo "  2) 日志清理"
        echo "  3) Docker 清理"
        echo "  4) 临时文件清理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "系统更新与清理" ;;
            2) placeholder "日志清理" ;;
            3) placeholder "Docker 清理" ;;
            4) placeholder "临时文件清理" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 模块：工具箱
# ============================================================
tools_menu() {
    while true; do
        clear; header "工具箱"
        echo "  1) 安装常用软件"
        echo "  2) 安装 Docker"
        echo "  3) 安装 Docker Compose"
        echo "  4) 安装系统监控工具"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) placeholder "安装常用软件" ;;
            2) placeholder "安装 Docker" ;;
            3) placeholder "安装 Docker Compose" ;;
            4) placeholder "安装系统监控工具" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

# ============================================================
# 主菜单
# ============================================================
menu_main() {
    while true; do
        clear
        header "Linux Helper — 系统管理助手"
        echo "  1)  网络优化"
        echo "  2)  系统配置"
        echo "  3)  系统安全"
        echo "  4)  系统调优"
        echo "  5)  系统信息"
        echo "  6)  系统维护"
        echo "  7)  工具箱"
        echo ""
        echo "  0)  退出脚本"
        echo ""
        echo -e "${BLUE}──────────────────────────────────────────────────${NC}"
        echo ""
        read -p "  请选择 [0-7]: " choice
        case "$choice" in
            1) network_menu ;;
            2) system_menu ;;
            3) security_menu ;;
            4) tuning_menu ;;
            5) info_menu ;;
            6) maintenance_menu ;;
            7) tools_menu ;;
            0)
                echo ""; success "感谢使用，再见！"; exit 0 ;;
            *)
                warn "无效选项，请重新选择。"
                read -p "按回车键继续..."
                ;;
        esac
    done
}

# ============================================================
# 安装 / 卸载
# ============================================================
INSTALL_DIR="/usr/local/linux-helper"
BIN_LINK="/usr/local/bin/linux-helper"

do_install() {
    local src
    src="$(readlink -f "${BASH_SOURCE[0]}")"

    echo ""; info "安装到: $INSTALL_DIR"

    mkdir -p "$INSTALL_DIR"
    cp "$src" "$INSTALL_DIR/install.sh"
    chmod +x "$INSTALL_DIR/install.sh"

    if [[ -L "$BIN_LINK" || -f "$BIN_LINK" ]]; then
        warn "已存在: $BIN_LINK，将覆盖..."
        rm -f "$BIN_LINK"
    fi
    ln -s "$INSTALL_DIR/install.sh" "$BIN_LINK"

    success "安装完成！"
    echo ""
    echo "  运行方式:"
    echo "    sudo linux-helper"
    echo "    或"
    echo "    sudo $INSTALL_DIR/install.sh"
    echo ""
}

do_uninstall() {
    echo ""; warn "将删除:"
    echo "  - $INSTALL_DIR"
    echo "  - $BIN_LINK"
    echo ""
    confirm "确认卸载？" || { info "取消卸载。"; exit 0; }

    rm -rf "$INSTALL_DIR"
    rm -f "$BIN_LINK"

    success "卸载完成。"
}

# ============================================================
# 入口
# ============================================================
case "${1:-}" in
    --install|install)
        do_install
        ;;
    --uninstall|uninstall)
        do_uninstall
        ;;
    *)
        check_root
        menu_main
        ;;
esac

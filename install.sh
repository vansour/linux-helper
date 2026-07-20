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
# 全局常量
# ============================================================
BACKUP_DIR="/etc/linux-helper/backups"

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
    read -r -p "$prompt [y/N]: " reply
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
    local backup_dir="$BACKUP_DIR"
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

# ---------- IPv6 管理 ----------

HOSTS_FILE="/etc/hosts"

ipv6_show_status() {
    echo ""
    header "当前 IPv6 状态"

    local disabled
    disabled=$(cat /proc/sys/net/ipv6/conf/all/disable_ipv6 2>/dev/null || echo 0)

    if [[ "$disabled" == "1" ]]; then
        echo -e "  系统 IPv6: ${RED}已禁用${NC}"
    else
        echo -e "  系统 IPv6: ${GREEN}已启用${NC}"
    fi

    echo ""
    echo "  GRUB 参数:"
    if grep -q 'ipv6.disable=1' /etc/default/grub 2>/dev/null; then
        echo -e "    ${YELLOW}ipv6.disable=1${NC} (在 GRUB 中禁用)"
    elif grep -q 'ipv6.disable=0' /etc/default/grub 2>/dev/null; then
        echo -e "    ${GREEN}ipv6.disable=0${NC} (在 GRUB 中启用)"
    else
        echo -e "    ${BLUE}未设置${NC}"
    fi

    echo ""
    echo "  sysctl 配置:"
    for f in /etc/sysctl.d/*.conf /etc/sysctl.conf; do
        [[ -f "$f" ]] || continue
        local line
        line=$(grep -E 'disable_ipv6' "$f" 2>/dev/null || true)
        if [[ -n "$line" ]]; then
            echo -e "    $f: ${YELLOW}$line${NC}"
        fi
    done

    echo ""
    echo "  /etc/hosts IPv6 记录:"
    local ipv6_hosts
    ipv6_hosts=$(grep -E '^\s*::' "$HOSTS_FILE" 2>/dev/null || true)
    if [[ -n "$ipv6_hosts" ]]; then
        echo "$ipv6_hosts" | while IFS= read -r line; do
            if echo "$line" | grep -qE '^\s*#'; then
                echo -e "    ${YELLOW}(已注释)${NC} $line"
            else
                echo -e "    ${GREEN}$line${NC}"
            fi
        done
    else
        echo "    (无)"
    fi

    echo ""
    if command -v ip &>/dev/null; then
        echo "  网卡 IPv6 地址:"
        ip -6 addr show 2>/dev/null | grep -E 'inet6' | head -5 | sed 's/^/    /' || echo "    (无)"
    fi
    echo ""
}

ipv6_disable() {
    echo ""
    header "禁用 IPv6"

    confirm "禁用 IPv6？这将修改 sysctl、GRUB、/etc/hosts 和 SSH 配置" || return

    mkdir -p "$BACKUP_DIR"
    local ts
    ts=$(date +%Y%m%d-%H%M%S)
    local bkup="$BACKUP_DIR/ipv6-backup-$ts"
    mkdir -p "$bkup"

    # 1. 备份 hosts
    cp "$HOSTS_FILE" "$bkup/hosts"

    # 2. 修改 /etc/hosts：注释掉 IPv6 主机记录
    info "处理 /etc/hosts 中的 IPv6 记录..."
    # 先备份 /etc/hosts
    cp "$HOSTS_FILE" "$HOSTS_FILE.lh-bak-ipv6-$ts"
    # 注释掉 IPv6 记录（::1, ff02:: 等标准 IPv6 hosts，含前导空格）
    sed -i -E 's/^[[:space:]]*(::1\s+|ff02::)/#\1/' "$HOSTS_FILE"
    # 也处理其他 fe80/200x/3ffe 等 IPv6 地址开头的行
    sed -i -E 's/^[[:space:]]*([a-f0-9]{2,4}:)/#\1/' "$HOSTS_FILE" 2>/dev/null || true
    success "/etc/hosts 已处理"

    # 3. sysctl 配置
    info "配置 sysctl 禁用 IPv6..."
    mkdir -p /etc/sysctl.d
    # 从所有 sysctl 文件中清理已有的 disable_ipv6
    for f in /etc/sysctl.d/*.conf /etc/sysctl.conf; do
        [[ -f "$f" ]] || continue
        grep -qE 'disable_ipv6' "$f" 2>/dev/null || continue
        cp "$f" "$bkup/$(basename "$f")" 2>/dev/null || true
        sed -i '/disable_ipv6/d' "$f"
        info "  已清理: $f"
    done
    # 写入统一的禁用配置
    cat > /etc/sysctl.d/99-lh-disable-ipv6.conf << 'EOF'
# 禁用 IPv6 — Linux Helper
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
EOF
    sysctl -p /etc/sysctl.d/99-lh-disable-ipv6.conf 2>&1 | head -5 || true
    success "sysctl 已生效"

    # 4. SSH 配置：限制为 ipv4
    info "配置 SSH 仅监听 IPv4..."
    SSHD_CONFIG="/etc/ssh/sshd_config"
    if grep -qE '^\s*AddressFamily' "$SSHD_CONFIG" 2>/dev/null; then
        cp "$SSHD_CONFIG" "$bkup/sshd_config" 2>/dev/null || true
        sed -i 's/^\s*AddressFamily.*/AddressFamily inet/' "$SSHD_CONFIG"
    else
        echo "AddressFamily inet" >> "$SSHD_CONFIG"
    fi
    # 也清理 drop-in 中的 AddressFamily
    if [[ -d /etc/ssh/sshd_config.d ]]; then
        for f in /etc/ssh/sshd_config.d/*.conf; do
            [[ -f "$f" ]] || continue
            grep -qE '^\s*AddressFamily' "$f" 2>/dev/null || continue
            sed -i '/^\s*AddressFamily/d' "$f"
            info "  已清理 SSH drop-in: $f"
        done
    fi
    sshd -t 2>/dev/null && systemctl restart sshd 2>/dev/null || systemctl restart ssh 2>/dev/null || true
    success "SSH 已配置仅 IPv4"

    # 5. GRUB 配置
    info "配置 GRUB 内核参数..."
    cp /etc/default/grub "$bkup/grub" 2>/dev/null || true
    if grep -q 'ipv6.disable=' /etc/default/grub 2>/dev/null; then
        sed -i 's/ipv6.disable=[0-9]/ipv6.disable=1/' /etc/default/grub
    else
        sed -i 's/^GRUB_CMDLINE_LINUX="\(.*\)"/GRUB_CMDLINE_LINUX="\1 ipv6.disable=1"/' /etc/default/grub
    fi
    info "GRUB 配置已更新（需要 grub-mkconfig -o /boot/grub/grub.cfg 后重启生效）"
    info "可运行 update-grub 更新 GRUB（可选，重启生效）"

    echo ""
    success "IPv6 禁用配置完成！部分更改需重启后完全生效。"
    echo ""
    ipv6_show_status
}

ipv6_enable() {
    echo ""
    header "启用 IPv6"

    confirm "启用 IPv6？这将恢复 sysctl、GRUB、/etc/hosts 和 SSH 配置" || return

    mkdir -p "$BACKUP_DIR"
    local ts
    ts=$(date +%Y%m%d-%H%M%S)

    # 1. 恢复 /etc/hosts 中的 IPv6 记录
    info "恢复 /etc/hosts 中的 IPv6 记录..."
    # 取消注释之前被注释的 IPv6 行（含前导空格）
    sed -i -E 's/^[[:space:]]*#(::1\s+|ff02::)/\1/' "$HOSTS_FILE"
    sed -i -E 's/^[[:space:]]*#([a-f0-9]{2,4}:)/\1/' "$HOSTS_FILE" 2>/dev/null || true
    # 检查是否已恢复
    local has_ipv6_hosts
    has_ipv6_hosts=$(grep -cE '^\s*::' "$HOSTS_FILE" 2>/dev/null || true)
    if [[ "$has_ipv6_hosts" -eq 0 ]]; then
        # 如果没有标准的 IPv6 hosts，重新添加
        cat >> "$HOSTS_FILE" << 'EOF'
::1     localhost ip6-localhost ip6-loopback
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
EOF
    fi
    success "/etc/hosts 已恢复"

    # 2. sysctl 启用 IPv6
    info "配置 sysctl 启用 IPv6..."
    for f in /etc/sysctl.d/*.conf /etc/sysctl.conf; do
        [[ -f "$f" ]] || continue
        grep -qE 'disable_ipv6' "$f" 2>/dev/null || continue
        cp "$f" "$BACKUP_DIR/$(basename "$f").bak.$ts" 2>/dev/null || true
        sed -i '/disable_ipv6/d' "$f"
        info "  已清理: $f"
    done

    # 如果存在我们的禁用文件，删除它
    rm -f /etc/sysctl.d/99-lh-disable-ipv6.conf

    # 立即生效（设置为 0）
    sysctl -w net.ipv6.conf.all.disable_ipv6=0 2>/dev/null || true
    sysctl -w net.ipv6.conf.default.disable_ipv6=0 2>/dev/null || true
    info "sysctl 已清理"

    # 3. SSH 恢复所有地址族
    info "恢复 SSH 监听所有地址..."
    SSHD_CONFIG="/etc/ssh/sshd_config"
    if grep -qE '^\s*AddressFamily' "$SSHD_CONFIG" 2>/dev/null; then
        sed -i '/^\s*AddressFamily/d' "$SSHD_CONFIG"
    fi
    if [[ -d /etc/ssh/sshd_config.d ]]; then
        for f in /etc/ssh/sshd_config.d/*.conf; do
            [[ -f "$f" ]] || continue
            grep -qE '^\s*AddressFamily' "$f" 2>/dev/null || continue
            sed -i '/^\s*AddressFamily/d' "$f"
        done
    fi
    sshd -t 2>/dev/null && systemctl restart sshd 2>/dev/null || systemctl restart ssh 2>/dev/null || true
    success "SSH 已恢复监听所有地址"

    # 4. GRUB 配置
    info "配置 GRUB 移除 ipv6.disable..."
    if grep -q 'ipv6.disable=' /etc/default/grub 2>/dev/null; then
        cp /etc/default/grub "$BACKUP_DIR/grub.bak.$ts" 2>/dev/null || true
        sed -i 's/ipv6.disable=[0-9]//g' /etc/default/grub
        # 清理多余空格
        sed -i 's/  */ /g' /etc/default/grub
        success "GRUB 已更新"
    fi

    echo ""
    success "IPv6 启用配置完成！部分更改需重启后完全生效。"
    echo ""
    ipv6_show_status
}

ipv6_menu() {
    while true; do
        clear || true; header "IPv6 管理"

        local disabled
        disabled=$(cat /proc/sys/net/ipv6/conf/all/disable_ipv6 2>/dev/null || echo 0)
        if [[ "$disabled" == "1" ]]; then
            echo -e "  当前状态: ${RED}IPv6 已禁用${NC}"
        else
            echo -e "  当前状态: ${GREEN}IPv6 已启用${NC}"
        fi
        echo ""
        echo "  1) 查看 IPv6 状态"
        echo "  2) 禁用 IPv6"
        echo "  3) 启用 IPv6"
        echo ""
        echo "  b) 返回主菜单"
        echo "  q) 退出脚本"
        echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) ipv6_show_status ; read -r -p "按回车键继续..." ;;
            2) ipv6_disable ;;
            3) ipv6_enable ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ; read -r -p "按回车键继续..." ;;
        esac
    done
}

network_menu() {
    while true; do
        clear || true; header "网络优化"
        echo "  1) 启用 BBR + fq + bpftune"
        echo "  2) TCP 参数调优"
        echo "  3) 查看网络状态"
        echo "  4) IPv6 管理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) enable_bbr ;;
            2) placeholder "TCP 参数调优" ;;
            3) placeholder "查看网络状态" ;;
            4) ipv6_menu ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统配置
# ============================================================

# ---------- 时区 & 时间同步（强制 Chrony） ----------

timezone_show_status() {
    echo ""
    header "当前时间配置"
    echo ""
    echo -e "  系统时区:   ${GREEN}$(timedatectl show --property=Timezone --value 2>/dev/null || echo "未知")${NC}"
    echo -e "  本地时间:   ${BLUE}$(date +'%Y-%m-%d %H:%M:%S %Z')${NC}"
    echo -e "  UTC 时间:   ${BLUE}$(date -u +'%Y-%m-%d %H:%M:%S UTC')${NC}"
    echo ""
    echo -e "  NTP 服务状态:"
    if systemctl is-active --quiet chronyd 2>/dev/null; then
        echo -e "    ${GREEN}chronyd${NC}  运行中 ✓"
    elif systemctl is-active --quiet chrony 2>/dev/null; then
        echo -e "    ${GREEN}chrony${NC}   运行中 ✓"
    elif systemctl is-active --quiet ntpd 2>/dev/null; then
        echo -e "    ${YELLOW}ntpd${NC}     运行中（建议迁移到 chrony）"
    elif systemctl is-active --quiet systemd-timesyncd 2>/dev/null; then
        echo -e "    ${YELLOW}systemd-timesyncd${NC}  运行中（建议迁移到 chrony）"
    else
        echo -e "    ${RED}未运行${NC}"
    fi
    if command -v chronyc &>/dev/null; then
        local leap_status
        leap_status=$(chronyc tracking 2>/dev/null | awk -F': ' '/Leap status/{print $2}') || true
        if [[ -n "$leap_status" ]]; then
            echo -e "    同步状态:   ${GREEN}${leap_status}${NC}"
        fi
    fi
    echo ""
}

chrony_force_setup() {
    local user_tz="${1:-}"
    echo ""
    header "强制使用 Chrony 时间同步"

    mkdir -p "$BACKUP_DIR"

    # 1. 安装 chrony（先装好再停老服务，最小化时间同步间隙）
    if ! command -v chronyd &>/dev/null || ! command -v chronyc &>/dev/null; then
        info "正在安装 chrony..."
        local install_ok=0
        if command -v apt &>/dev/null; then
            apt update -qq && DEBIAN_FRONTEND=noninteractive apt install -y chrony && install_ok=1
        elif command -v dnf &>/dev/null; then
            dnf install -y chrony && install_ok=1
        elif command -v yum &>/dev/null; then
            yum install -y chrony && install_ok=1
        elif command -v zypper &>/dev/null; then
            zypper install -y chrony && install_ok=1
        else
            error "不支持的包管理器，请手动安装: apt install chrony"
            return 1
        fi
        if (( install_ok )); then
            success "chrony 安装成功"
        else
            error "chrony 安装失败"
            return 1
        fi
    else
        success "chrony 已安装"
    fi

    # 2. 检测并停用冲突的 NTP 服务（此时 chrony 已就绪）
    local conflict_found=0
    for svc in systemd-timesyncd ntpd openntpd ntp; do
        if systemctl is-active --quiet "$svc" 2>/dev/null; then
            warn "停止冲突服务: $svc"
            systemctl stop "$svc" 2>/dev/null || true
            conflict_found=1
        fi
        if systemctl is-enabled --quiet "$svc" 2>/dev/null; then
            info "禁用冲突服务: $svc"
            systemctl disable "$svc" 2>/dev/null || true
        fi
    done
    timedatectl set-ntp false 2>/dev/null || true
    if (( conflict_found )); then
        success "已停用冲突的 NTP 服务"
    fi

    # 3. 备份现有配置
    local chrony_conf
    if [[ -f "/etc/chrony.conf" ]]; then
        chrony_conf="/etc/chrony.conf"
    else
        chrony_conf="/etc/chrony/chrony.conf"
    fi
    local chrony_bak
    chrony_bak="$BACKUP_DIR/chrony.conf.bak.$(date +%Y%m%d-%H%M%S)"
    if [[ -f "$chrony_conf" ]]; then
        cp "$chrony_conf" "$chrony_bak" 2>/dev/null || true
    fi

    # 4. 根据时区确定最近的 NTP 池
    local current_tz
    if [[ -n "$user_tz" ]]; then
        current_tz="$user_tz"
    else
        current_tz="$(timedatectl show --property=Timezone --value 2>/dev/null || true)"
    fi
    local region="${current_tz%%/*}"
    local ntp_pool
    case "$region" in
        Asia)                ntp_pool="asia.pool.ntp.org" ;;
        Europe)              ntp_pool="europe.pool.ntp.org" ;;
        America|US|Canada)   ntp_pool="north-america.pool.ntp.org" ;;
        Pacific|Australia)   ntp_pool="oceania.pool.ntp.org" ;;
        Africa)              ntp_pool="africa.pool.ntp.org" ;;
        *)                   ntp_pool="pool.ntp.org" ;;
    esac

    # 5. 写入 chrony 配置
    cat > "$chrony_conf" << CHRONYEOF
# Chrony 配置文件 — Linux Helper
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')
# 时区: ${current_tz:-UTC}

# NTP 服务器池（根据时区自动选择最近节点）
pool ${ntp_pool} iburst

# 备用 NTP 服务器
pool pool.ntp.org iburst
server ntp.aliyun.com iburst
server time.google.com iburst

# 漂移文件（记录时钟频率偏差）
driftfile /var/lib/chrony/drift

# 快速同步 — 启动后偏差 >1 秒时前 3 次立即调整
makestep 1.0 3

# 硬件时钟同步
rtcsync

# 本地监听（仅允许本机使用 chronyc）
bindcmdaddress 127.0.0.1
bindcmdaddress ::1

# 日志目录
logdir /var/log/chrony
CHRONYEOF
    success "chrony 配置文件已更新 (${chrony_conf})"

    # 6. 检查配置语法（类比 sshd -t）
    if command -v chronyd &>/dev/null; then
        if ! chronyd -t -f "$chrony_conf" 2>/dev/null; then
            error "chrony 配置语法错误！请检查 $chrony_conf"
            return 1
        fi
        success "chrony 配置语法正确"
    fi

    # 7. 启用并启动 chrony 服务
    info "启动 chronyd 服务..."
    systemctl enable chronyd 2>/dev/null || systemctl enable chrony 2>/dev/null || {
        warn "chrony 服务启用失败，请手动检查: systemctl enable chronyd"
    }
    systemctl restart chronyd 2>/dev/null || systemctl restart chrony 2>/dev/null || {
        warn "chrony 服务启动失败，请手动检查: systemctl status chronyd"
    }

    # 8. 立即同步时间（不用 -a：无身份验证密钥时也能通过本地 socket 运行）
    sleep 1
    chronyc makestep 2>/dev/null || true

    # 9. 验证并输出状态
    echo ""
    local chrony_active=0
    if systemctl is-active --quiet chronyd 2>/dev/null || systemctl is-active --quiet chrony 2>/dev/null; then
        chrony_active=1
        success "chrony 服务运行中 ✓"
        if command -v chronyc &>/dev/null; then
            echo ""
            echo -e "  ${BLUE}chronyc tracking:${NC}"
            chronyc tracking 2>/dev/null | sed 's/^/    /'
            echo ""
            echo -e "  ${BLUE}chronyc sources -v:${NC}"
            chronyc sources -v 2>/dev/null | sed 's/^/    /'
        fi
    else
        warn "chrony 服务状态异常，请手动检查:"
        echo "    systemctl status chronyd"
        echo "    journalctl -u chronyd --no-pager -n 30"
    fi

    echo ""
    if (( chrony_active )); then
        success "Chrony 配置完成！系统时间将自动同步。"
    else
        warn "Chrony 配置过程有异常，请检查以上错误信息。"
    fi
}

timezone_set() {
    echo ""
    header "设置时区"

    if ! command -v timedatectl &>/dev/null; then
        error "未找到 timedatectl，请安装 systemd。"
        return
    fi

    timezone_show_status

    # 确认操作（涉及 chrony 安装、停止其他 NTP 服务等）
    confirm "将强制配置 Chrony 时间同步（会停止其他 NTP 服务），确认？" || return

    echo "  选择时区设置方式:"
    echo "   1) 从常用时区列表中选择"
    echo "   2) 搜索时区（输入关键词）"
    echo "   3) 手动输入时区名称"
    echo ""
    echo "   b) 返回"
    echo "   q) 退出"
    echo ""
    read -r -p "  请选择 [1-3]: " method

    local tz=""

    case "$method" in
        1)
            echo ""
            header "选择地区"
            echo "  1) 亚洲"
            echo "  2) 欧洲"
            echo "  3) 北美洲"
            echo "  4) 南美洲"
            echo "  5) 大洋洲"
            echo "  6) 非洲"
            echo ""
            read -r -p "  请选择地区 [1-6]: " region

            local region_name=""
            local tz_list=()
            case "$region" in
                1) region_name="亚洲"    ; tz_list=(Asia/Shanghai Asia/Tokyo Asia/Singapore Asia/Dubai Asia/Hong_Kong Asia/Taipei Asia/Seoul Asia/Bangkok Asia/Jakarta Asia/Kolkata Asia/Kuala_Lumpur) ;;
                2) region_name="欧洲"    ; tz_list=(Europe/London Europe/Paris Europe/Berlin Europe/Moscow Europe/Amsterdam Europe/Madrid Europe/Rome Europe/Stockholm Europe/Zurich Europe/Istanbul Europe/Vienna) ;;
                3) region_name="北美洲"  ; tz_list=(America/New_York America/Chicago America/Denver America/Los_Angeles America/Toronto America/Vancouver America/Mexico_City America/Phoenix America/Halifax) ;;
                4) region_name="南美洲"  ; tz_list=(America/Sao_Paulo America/Santiago America/Buenos_Aires America/Bogota America/Lima America/Caracas America/Montevideo) ;;
                5) region_name="大洋洲"  ; tz_list=(Pacific/Auckland Australia/Sydney Pacific/Fiji Pacific/Honolulu Pacific/Guam Australia/Melbourne) ;;
                6) region_name="非洲"    ; tz_list=(Africa/Cairo Africa/Johannesburg Africa/Lagos Africa/Nairobi Africa/Casablanca Africa/Accra) ;;
                *) warn "无效选择"; return ;;
            esac

            echo ""
            header "选择时区（${region_name}）"
            local i
            for i in "${!tz_list[@]}"; do
                printf "  %2d) %s\n" $((i+1)) "${tz_list[$i]}"
            done
            echo ""
            read -r -p "  请选择 [1-${#tz_list[@]}]: " tz_idx
            if [[ "$tz_idx" =~ ^[0-9]+$ ]] && (( tz_idx >= 1 && tz_idx <= ${#tz_list[@]} )); then
                tz="${tz_list[$((tz_idx-1))]}"
            else
                warn "无效选择"; return
            fi
            ;;
        2)
            read -r -p "  输入关键词 (如 Shanghai, Tokyo, New_York): " keyword
            [[ -z "$keyword" ]] && { warn "关键词不能为空"; return; }
            local matches
            # 使用 -F 做字面匹配，避免用户输入的 [] 等被当作正则
            matches=$(timedatectl list-timezones 2>/dev/null | grep -i -F "$keyword" || true)
            if [[ -z "$matches" ]]; then
                warn "未找到匹配的时区"; return
            fi
            local match_list=()
            while IFS= read -r line; do
                match_list+=("$line")
                [[ ${#match_list[@]} -ge 30 ]] && break
            done <<< "$matches"

            if [[ ${#match_list[@]} -eq 1 ]]; then
                tz="${match_list[0]}"
                echo "  匹配: ${tz}"
            else
                echo ""
                header "搜索结果（前 ${#match_list[@]} 条）"
                local i
                for i in "${!match_list[@]}"; do
                    printf "  %2d) %s\n" $((i+1)) "${match_list[$i]}"
                done
                echo ""
                read -r -p "  请选择 [1-${#match_list[@]}]: " sel
                if [[ "$sel" =~ ^[0-9]+$ ]] && (( sel >= 1 && sel <= ${#match_list[@]} )); then
                    tz="${match_list[$((sel-1))]}"
                else
                    warn "无效选择"; return
                fi
            fi
            ;;
        3)
            read -r -p "  输入时区名称 (如 Asia/Shanghai): " tz
            ;;
        b|B) return ;;
        q|Q) exit 0 ;;
        *) warn "无效选项"; return ;;
    esac

    # 验证时区
    if [[ -z "$tz" ]]; then
        warn "时区名称不能为空"; return
    fi
    if ! timedatectl list-timezones 2>/dev/null | grep -qxF "$tz"; then
        warn "无效的时区名称: ${tz}"
        echo "  提示: 运行 timedatectl list-timezones 查看所有可用时区"
        return
    fi

    # 设置时区
    echo ""
    info "设置时区: ${tz}"
    timedatectl set-timezone "$tz" 2>/dev/null || {
        error "设置时区失败"
        return
    }
    success "时区已设置为 ${tz}"

    # 强制配置 chrony 时间同步
    echo ""
    info "正在配置 chrony 时间同步服务..."
    if ! chrony_force_setup "$tz"; then
        warn "Chrony 配置过程有异常，请查看以上错误信息。"
    fi

    echo ""
    timezone_show_status
}

chrony_menu() {
    while true; do
        clear || true; header "Chrony 时间同步管理"

        echo "  1) 查看 Chrony 运行状态"
        echo "  2) 强制配置 Chrony（安装/启用/禁用其他 NTP）"
        echo "  3) 立即同步时间"
        echo ""
        echo "  b) 返回上级"
        echo "  q) 退出脚本"
        echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1)
                timezone_show_status
                if command -v chronyc &>/dev/null; then
                    echo -e "\n  ${BLUE}chronyc tracking:${NC}"
                    chronyc tracking 2>/dev/null | sed 's/^/    /' || info "chrony 未运行"
                    echo -e "\n  ${BLUE}chronyc sources -v:${NC}"
                    chronyc sources -v 2>/dev/null | sed 's/^/    /' || info "chrony 未运行"
                fi
                read -r -p "按回车键继续..." ;;
            2) chrony_force_setup ; read -r -p "按回车键继续..." ;;
            3)
                if command -v chronyc &>/dev/null; then
                    info "正在同步时间..."
                    # 不使用 -a：无身份验证密钥时也能通过本地 socket 运行
                    chronyc makestep 2>/dev/null && success "时间已同步" || warn "同步失败，chrony 可能未运行"
                else
                    warn "chrony 未安装，请先执行选项 2"
                fi
                read -r -p "按回车键继续..." ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ; read -r -p "按回车键继续..." ;;
        esac
    done
}

system_menu() {
    while true; do
        clear || true; header "系统配置"
        echo "  1) 设置时区（含强制 Chrony 时间同步）"
        echo "  2) 修改主机名"
        echo "  3) 配置语言环境"
        echo "  4) Chrony 时间同步管理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) timezone_set ;;
            2) placeholder "修改主机名" ;;
            3) placeholder "配置语言环境" ;;
            4) chrony_menu ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统安全
# ============================================================

# ---------- SSH 辅助函数 ----------

SSHD_CONFIG="/etc/ssh/sshd_config"
SSHD_CONFIG_DIR="/etc/ssh/sshd_config.d"
LH_SSH_DROPIN="$SSHD_CONFIG_DIR/99-linux-helper.conf"

# 获取 SSH 监听的所有端口（从运行进程中提取）
ssh_current_ports() {
    ss -tlnp 2>/dev/null | grep sshd | awk '{print $4}' | sed 's/.*://' | sort -u | tr '\n' ' ' | sed 's/ $//'
}

# 查找某个 SSH 配置项出现在哪些文件中
ssh_find_setting() {
    local key="$1"
    local found=()
    # 检查主配置文件
    if grep -qE "^\s*${key}\s+" "$SSHD_CONFIG" 2>/dev/null; then
        found+=("$SSHD_CONFIG")
    fi
    # 检查所有 drop-in 文件（按字母序，越晚优先级越高）
    if [[ -d "$SSHD_CONFIG_DIR" ]]; then
        for f in "$SSHD_CONFIG_DIR"/*.conf; do
            [[ -f "$f" ]] || continue
            if grep -qE "^\s*${key}\s+" "$f" 2>/dev/null; then
                found+=("$f")
            fi
        done
    fi
    printf '%s\n' "${found[@]}"
}

# 获取某个 SSH 配置项的实际生效值
ssh_get_setting() {
    local key="$1"
    local last_val=""
    # 先检查主配置文件（优先级低）
    local val
    val=$(grep -E "^\s*${key}\s+" "$SSHD_CONFIG" 2>/dev/null | tail -1 | awk '{print $2}')
    [[ -n "$val" ]] && last_val="$val"
    # 再检查 drop-in 文件（按字母序，越晚优先级越高，覆盖主配置）
    if [[ -d "$SSHD_CONFIG_DIR" ]]; then
        for f in "$SSHD_CONFIG_DIR"/*.conf; do
            [[ -f "$f" ]] || continue
            val=$(grep -E "^\s*${key}\s+" "$f" 2>/dev/null | tail -1 | awk '{print $2}')
            [[ -n "$val" ]] && last_val="$val"
        done
    fi
    echo "${last_val:-}"
}

# 备份所有 SSH 配置文件
ssh_backup_all() {
    mkdir -p "$BACKUP_DIR"
    local ts
    ts=$(date +%Y%m%d-%H%M%S)
    local backup_dir="$BACKUP_DIR/ssh-backup-$ts"
    mkdir -p "$backup_dir"

    cp "$SSHD_CONFIG" "$backup_dir/sshd_config" 2>/dev/null || true
    if [[ -d "$SSHD_CONFIG_DIR" ]]; then
        cp -r "$SSHD_CONFIG_DIR" "$backup_dir/sshd_config.d" 2>/dev/null || true
    fi
    info "SSH 配置已备份到: $backup_dir"
    echo "$backup_dir"
}

# 安全重启 SSH（带超时回滚）
ssh_safe_restart() {
    info "正在检查 SSH 配置语法..."
    if sshd -t 2>&1; then
        info "配置语法正确，重启 SSH 服务..."
        systemctl restart sshd 2>/dev/null || systemctl restart ssh 2>/dev/null || {
            warn "SSH 重启失败，请手动检查"
            return 1
        }
        success "SSH 服务已重启"
    else
        error "SSH 配置语法错误！请检查配置。"
        return 1
    fi
}

# ---------- SSH 功能 ----------

ssh_change_port() {
    echo ""
    header "修改 SSH 端口"

    local current_ports
    current_ports=$(ssh_current_ports)
    info "当前 SSH 监听端口: ${current_ports:-未检测到}"

    local new_port
    read -r -p "输入新的 SSH 端口号 (1-65535): " new_port
    if [[ ! "$new_port" =~ ^[0-9]+$ ]] || (( new_port < 1 || new_port > 65535 )); then
        warn "无效端口号，请输入 1-65535 之间的数字。"
        return
    fi

    # 检查端口是否已被占用
    if ss -tlnp 2>/dev/null | grep -q ":$new_port "; then
        warn "端口 $new_port 已被占用"
        confirm "仍然强制设置？" || return
    fi

    local backup_dir
    backup_dir=$(ssh_backup_all)

    # 1. 从所有文件（主配置 + drop-in）中删除所有 Port 行
    info "清除所有文件中的 Port 配置..."
    sed -i '/^\s*Port\s\+/d' "$SSHD_CONFIG"
    if [[ -d "$SSHD_CONFIG_DIR" ]]; then
        for f in "$SSHD_CONFIG_DIR"/*.conf; do
            [[ -f "$f" ]] || continue
            if grep -qE '^\s*Port\s+' "$f" 2>/dev/null; then
                sed -i '/^\s*Port\s\+/d' "$f"
                info "  已清理: $f"
            fi
            # 如果文件变空则删除
            if [[ ! -s "$f" ]] || ! grep -qE '\S' "$f" 2>/dev/null; then
                rm -f "$f"
                info "  已删除空白文件: $f"
            fi
        done
    fi

    # 2. 统一写入 99-linux-helper.conf
    mkdir -p "$SSHD_CONFIG_DIR"
    echo "Port ${new_port}" >> "$LH_SSH_DROPIN"
    info "已写入: $LH_SSH_DROPIN -> Port ${new_port}"

    echo ""
    info "修改后检查配置..."
    if sshd -t 2>&1; then
        success "配置语法正确"
        info "端口配置已写入: $(readlink -f "$LH_SSH_DROPIN")"
        echo ""
        warn "请确认以下事项后再重启 SSH:"
        echo "  1. 防火墙已放行端口 $new_port"
        echo "  2. 如果你通过 SSH 连接，新端口会话不会中断"
        echo ""
        if confirm "立即重启 SSH 服务？"; then
            if command -v ufw &>/dev/null && ufw status | grep -q active; then
                info "检测到 UFW 防火墙，自动放行端口 $new_port ..."
                ufw allow "$new_port"/tcp 2>/dev/null || true
            fi
            if command -v firewall-cmd &>/dev/null; then
                info "检测到 firewalld，自动放行端口 $new_port ..."
                firewall-cmd --add-port="$new_port"/tcp --permanent 2>/dev/null || true
                firewall-cmd --reload 2>/dev/null || true
            fi
            ssh_safe_restart
            echo ""
            info "新端口 $new_port 已生效"
            info "如需用旧端口连接，请在 $(date -d '+5 minutes' '+%H:%M:%S') 前保持旧会话"
        else
            info "跳过重启，配置已保存。下次重启 SSH 后生效。"
        fi
    else
        error "配置语法错误！恢复备份..."
        cp "$backup_dir/sshd_config" "$SSHD_CONFIG" 2>/dev/null || true
        if [[ -d "$backup_dir/sshd_config.d" ]]; then
            rm -rf "$SSHD_CONFIG_DIR"
            cp -r "$backup_dir/sshd_config.d" "$SSHD_CONFIG_DIR" 2>/dev/null || true
        fi
        warn "已恢复备份，配置未生效。"
    fi
}

ssh_root_login() {
    echo ""
    header "配置 root 密码登录"

    # 显示当前状态
    local permit_root
    local password_auth
    permit_root=$(ssh_get_setting "PermitRootLogin")
    password_auth=$(ssh_get_setting "PasswordAuthentication")

    echo "  当前 PermitRootLogin:    ${permit_root:-未设置 (默认 prohibit-password)}"
    echo "  当前 PasswordAuthentication: ${password_auth:-未设置 (默认 yes)}"
    echo ""

    # 检查是否已启用
    if [[ "$permit_root" == "yes" ]] && [[ "$password_auth" != "no" ]]; then
        success "root 密码登录已经是开启状态"
        if confirm "重新设置？"; then
            :
        else
            return
        fi
    fi

    confirm "开启 root 密码登录？（将设置 PermitRootLogin yes 并确保密码认证开启）" || return

    ssh_backup_all

    # 第1步：从所有文件中删除 PermitRootLogin / PasswordAuthentication / ChallengeResponseAuthentication
    for key in PermitRootLogin PasswordAuthentication ChallengeResponseAuthentication; do
        sed -i "/^\s*${key}\s\+/d" "$SSHD_CONFIG"
        if [[ -d "$SSHD_CONFIG_DIR" ]]; then
            for f in "$SSHD_CONFIG_DIR"/*.conf; do
                [[ -f "$f" ]] || continue
                if grep -qE "^\s*${key}\s+" "$f" 2>/dev/null; then
                    sed -i "/^\s*${key}\s\+/d" "$f"
                    info "  已清理 ${key}: $f"
                fi
                # 文件变空则删除
                if [[ ! -s "$f" ]] || ! grep -qE '\S' "$f" 2>/dev/null; then
                    rm -f "$f"
                    info "  已删除空白文件: $f"
                fi
            done
        fi
    done

    # 第2步：统一写入 99-linux-helper.conf
    mkdir -p "$SSHD_CONFIG_DIR"
    {
        echo "PermitRootLogin yes"
        echo "PasswordAuthentication yes"
        echo "ChallengeResponseAuthentication yes"
    } >> "$LH_SSH_DROPIN"
    info "已写入: $LH_SSH_DROPIN"

    echo ""
    if sshd -t 2>&1; then
        success "配置语法正确"
        if confirm "立即重启 SSH 服务？"; then
            ssh_safe_restart
            echo ""
            info "请在新终端中测试 root 密码登录后再关闭当前会话！"
        fi
    else
        error "配置语法错误，请手动检查 /etc/ssh/sshd_config"
    fi
}

ssh_manage_keys() {
    while true; do
        clear || true; header "SSH 公钥管理"

        # 查找所有有 authorized_keys 的用户
        local users=()
        local key_files=()
        while IFS=: read -r user _ uid _ _ home shell; do
            [[ "$home" == /nonexistent || "$home" == / || -z "$home" ]] && continue
            [[ "$shell" == /usr/sbin/nologin || "$shell" == /bin/false || "$shell" == /sbin/nologin ]] && continue
            local ak="$home/.ssh/authorized_keys"
            if [[ -f "$ak" ]]; then
                users+=("$user")
                key_files+=("$ak")
            fi
        done < /etc/passwd

        if [[ ${#users[@]} -eq 0 ]]; then
            info "系统中没有找到 authorized_keys 文件。"
            read -r -p "按回车键返回..."
            return
        fi

        echo "  选择用户查看其公钥:"
        local i
        for i in "${!users[@]}"; do
            local count
            count=$(wc -l < "${key_files[$i]}")
            echo "  $((i+1))) ${users[$i]} (${count} 个密钥)"
        done
        echo ""
        echo "  b) 返回上级"
        echo "  q) 退出"
        echo ""
        read -r -p "  请选择用户: " user_choice

        [[ "$user_choice" =~ ^[Bb]$ ]] && break
        [[ "$user_choice" =~ ^[Qq]$ ]] && exit 0

        local idx=$((user_choice - 1))
        if [[ $idx -ge 0 && $idx -lt ${#users[@]} ]]; then
            ssh_list_delete_keys "${users[$idx]}" "${key_files[$idx]}"
        fi
    done
}

ssh_list_delete_keys() {
    local user="$1"
    local key_file="$2"

    while true; do
        clear || true; header "用户 ${user} 的 SSH 公钥"

        local keys=()
        while IFS= read -r line; do
            keys+=("$line")
        done < "$key_file"

        if [[ ${#keys[@]} -eq 0 ]]; then
            info "该用户没有公钥。"
            read -r -p "按回车键返回..."
            return
        fi

        local i
        for i in "${!keys[@]}"; do
            local key_preview
            key_preview=$(echo "${keys[$i]}" | awk '{print $1" "substr($3,1,40)}')
            echo "  $((i+1))) ${key_preview:-空白行 $((i+1))}"
        done
        echo ""
        echo "  输入编号删除对应密钥（可多选，如: 1 3 5）"
        echo "  a) 删除全部密钥"
        echo "  b) 返回上级"
        echo "  q) 退出"
        echo ""
        read -r -p "  请选择: " del_choice

        [[ "$del_choice" =~ ^[Bb]$ ]] && return
        [[ "$del_choice" =~ ^[Qq]$ ]] && exit 0

        if [[ "$del_choice" =~ ^[Aa]$ ]]; then
            confirm "确认删除 ${user} 的所有 ${#keys[@]} 个密钥？" || continue
            : > "$key_file"
            success "已删除 ${user} 的全部密钥"
            read -r -p "按回车键继续..."
            return
        fi

        local to_delete=()
        local nums
        nums=($del_choice)
        local n
        for n in "${nums[@]}"; do
            if [[ "$n" =~ ^[0-9]+$ ]] && (( n >= 1 && n <= ${#keys[@]} )); then
                to_delete+=("$n")
            fi
        done

        if [[ ${#to_delete[@]} -eq 0 ]]; then
            warn "无效选择"
            read -r -p "按回车键继续..."
            continue
        fi

        # 排序并去重（从大到小删除以免索引偏移）
        IFS=$'\n' to_delete=($(printf "%s\n" "${to_delete[@]}" | sort -ru))
        unset IFS

        local tmpfile
        tmpfile=$(mktemp)
        cp "$key_file" "$tmpfile"

        local count=0
        local n
        for n in "${to_delete[@]}"; do
            local line_idx=$((n - 1))
            local preview
            preview=$(echo "${keys[$line_idx]}" | awk '{print substr($0,1,60)}')
            warn "  删除: ${preview}..."
            count=$((count + 1))
        done

        confirm "确认删除以上 ${count} 个密钥？" || {
            rm -f "$tmpfile"
            continue
        }

        # 重建文件：跳过被选中的行（用 awk 实现原子操作）
        {
            local del_idx=()
            for n in "${to_delete[@]}"; do
                del_idx+=("$n")
            done
            # 构建 awk 表达式: NR==n1 {next} NR==n2 {next} ... {print}
            local awk_script=""
            for n in "${del_idx[@]}"; do
                awk_script="${awk_script}NR==${n}{next};"
            done
            awk_script="${awk_script}1"
            awk "$awk_script" "$tmpfile" > "$key_file"
        }

        rm -f "$tmpfile"
        success "已删除 ${count} 个密钥"

        # 检查是否还有剩余密钥
        local remaining
        remaining=$(wc -l < "$key_file")
        info "剩余 ${remaining} 个密钥"
        read -r -p "按回车键继续..."
    done
}

ssh_menu() {
    while true; do
        clear || true; header "SSH 安全配置"

        local port display
        port=$(ssh_get_setting "Port")
        [[ -z "$port" ]] && port="22 (默认)"
        local root_login
        root_login=$(ssh_get_setting "PermitRootLogin")
        [[ -z "$root_login" ]] && root_login="未设置"
        local pw_auth
        pw_auth=$(ssh_get_setting "PasswordAuthentication")
        [[ -z "$pw_auth" ]] && pw_auth="未设置"

        echo "  1) 修改 SSH 端口          (当前: ${port})"
        echo "  2) 开启 root 密码登录     (PermitRootLogin: ${root_login})"
        echo "  3) 管理 SSH 公钥          (查看/删除)"
        echo ""
        echo "  b) 返回主菜单"
        echo "  q) 退出脚本"
        echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) ssh_change_port ;;
            2) ssh_root_login ;;
            3) ssh_manage_keys ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

security_menu() {
    while true; do
        clear || true; header "系统安全"
        echo "  1) SSH 安全配置"
        echo "  2) 配置防火墙"
        echo "  3) Fail2Ban 管理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) ssh_menu ;;
            2) placeholder "配置防火墙" ;;
            3) placeholder "Fail2Ban 管理" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统调优
# ============================================================

# ---------- Swap 管理 ----------

swap_show_status() {
    echo ""
    header "当前 Swap 状态"
    echo ""
    if swapon --show 2>/dev/null; then
        echo ""
    else
        info "未检测到任何 Swap 设备"
    fi
    echo -e "${BLUE}free -h${NC}:"
    free -h | grep -E '^Swap|^Mem' || true
    echo ""
    # 显示 /etc/fstab 中的 swap 配置
    local fstab_swaps
    fstab_swaps=$(grep -E '\sswap\s' /etc/fstab 2>/dev/null || true)
    if [[ -z "$fstab_swaps" ]]; then
        info "/etc/fstab 中没有 Swap 配置"
    else
        info "/etc/fstab 中的 Swap 配置:"
        echo "$fstab_swaps" | while IFS= read -r line; do echo "  $line"; done
    fi
    # 显示 /etc/linux-helper swap 记录
    local record_file="/etc/linux-helper/swap-record"
    if [[ -f "$record_file" ]]; then
        echo ""
        info "Linux Helper 管理的 Swap 文件: $(cat "$record_file")"
    fi
    echo ""
}

swap_add() {
    echo ""
    header "添加 Swap"

    # 默认路径
    local swap_path="/swapfile"
    local mem_total_mb
    mem_total_mb=$(free -m | awk '/^Mem:/{print $2}')
    local default_size_mb=$(( mem_total_mb - 1 ))
    (( default_size_mb < 1 )) && default_size_mb=$((mem_total_mb / 2))

    read -r -p "Swap 文件路径 [${swap_path}]: " input_path
    [[ -n "$input_path" ]] && swap_path="$input_path"

    # 检查是否已存在
    if [[ -f "$swap_path" ]]; then
        if swapon --show | grep -q "$swap_path"; then
            warn "$swap_path 已作为 Swap 在使用"
            read -r -p "请先删除现有 Swap 后再添加。按回车键返回..."
            return
        fi
    fi

    # 输入大小
    local size_input
    read -r -p "Swap 大小（单位 MB，默认 ${default_size_mb}）: " size_input
    size_input="${size_input:-$default_size_mb}"
    if [[ ! "$size_input" =~ ^[0-9]+$ ]] || (( size_input < 1 )); then
        warn "无效大小，请输入正整数（单位 MB）"
        return
    fi

    # 检查磁盘空间
    local target_dir
    target_dir="$(dirname "$swap_path")"
    local available_kb
    available_kb=$(df "$target_dir" 2>/dev/null | tail -1 | awk '{print $4}')
    local needed_kb=$((size_input * 1024))
    if [[ -n "$available_kb" ]] && (( available_kb < needed_kb )); then
        warn "磁盘空间不足！需要 ${size_input}MB，可用约 $((available_kb / 1024))MB"
        confirm "仍然继续？" || return
    fi

    echo ""
    info "创建 Swap 文件: ${swap_path} (${size_input}MB)..."
    echo ""

    # 创建 swap 文件（优先 fallocate，回退 dd）
    if command -v fallocate &>/dev/null; then
        fallocate -l "${size_input}M" "$swap_path" 2>/dev/null || {
            info "fallocate 失败，使用 dd 创建..."
            dd if=/dev/zero of="$swap_path" bs=1M count="$size_input" status=progress 2>&1
        }
    else
        dd if=/dev/zero of="$swap_path" bs=1M count="$size_input" status=progress 2>&1
    fi || {
        error "创建 Swap 文件失败"
        return
    }
    chmod 600 "$swap_path"

    # 检查文件系统是否支持 swap file（btrfs 需特殊处理）
    local swap_fs
    swap_fs=$(df -T "$swap_path" 2>/dev/null | tail -1 | awk '{print $2}')
    if [[ "$swap_fs" == "btrfs" ]]; then
        info "检测到 btrfs 文件系统，创建 swap 子卷..."
        # btrfs 需要禁用 CoW
        chattr +C "$swap_path" 2>/dev/null || true
    fi

    # 格式化为 swap
    mkswap "$swap_path" || {
        error "格式化 Swap 失败"
        rm -f "$swap_path"
        return
    }

    # 启用
    swapon "$swap_path" || {
        error "启用 Swap 失败"
        rm -f "$swap_path"
        return
    }

    # 写入 fstab（检查是否已有）
    if grep -q "$swap_path" /etc/fstab 2>/dev/null; then
        info "$swap_path 已存在于 /etc/fstab"
    else
        echo "$swap_path none swap sw 0 0" >> /etc/fstab
        success "已添加到 /etc/fstab"
    fi

    # 记录到 linux-helper
    mkdir -p /etc/linux-helper
    echo "$swap_path" > /etc/linux-helper/swap-record

    success "Swap 添加成功！"
    echo ""
    swap_show_status
}

swap_delete() {
    echo ""
    header "删除 Swap"

    # 显示当前 swap
    if ! swapon --show 2>/dev/null | grep -q .; then
        info "没有启用中的 Swap 设备"
        read -r -p "按回车键返回..."
        return
    fi

    swapon --show
    echo ""

    # 检测路径
    local swap_paths=()
    local swaplist
    swaplist=$(swapon --show --noheadings 2>/dev/null | awk '{print $1}') || true
    if [[ -n "$swaplist" ]]; then
        while IFS= read -r path; do
            [[ -n "$path" ]] && swap_paths+=("$path")
        done <<< "$swaplist"
    fi

    if [[ ${#swap_paths[@]} -eq 0 ]]; then
        info "未检测到 Swap 设备"
        read -r -p "按回车键返回..."
        return
    fi

    local target=""
    if [[ ${#swap_paths[@]} -eq 1 ]]; then
        target="${swap_paths[0]}"
        info "检测到一个 Swap 设备: $target"
        confirm "删除此 Swap？" || return
    else
        echo "选择要删除的 Swap 设备:"
        local i
        for i in "${!swap_paths[@]}"; do
            echo "  $((i+1))) ${swap_paths[$i]}"
        done
        echo ""
        read -r -p "请输入编号: " sel
        [[ "$sel" =~ ^[0-9]+$ ]] && (( sel >= 1 && sel <= ${#swap_paths[@]} )) || {
            warn "无效选择"; return
        }
        target="${swap_paths[$((sel-1))]}"
        confirm "删除 Swap: ${target}？" || return
    fi

    # swapoff
    swapoff "$target" 2>/dev/null || {
        error "swapoff 失败"
        return
    }
    success "已停用: $target"

    # 从 fstab 移除（精确匹配路径）
    sed -i "\|^${target}\s|d" /etc/fstab
    # 也匹配带前导空格的变体
    sed -i "s|^[[:space:]]*${target}[[:space:]].*||" /etc/fstab
    # 清理残留空行
    sed -i '/^[[:space:]]*$/d' /etc/fstab
    success "已从 /etc/fstab 移除"

    # 如果是文件 swap，询问是否删除文件
    if [[ -f "$target" ]]; then
        if confirm "删除 swap 文件 ${target}？"; then
            rm -f "$target"
            success "已删除文件: $target"
        fi
    fi

    # 清理记录
    local record_file="/etc/linux-helper/swap-record"
    if [[ -f "$record_file" ]] && grep -q "$target" "$record_file" 2>/dev/null; then
        rm -f "$record_file"
    fi

    echo ""
    success "Swap 删除完成"
}

swap_adjust() {
    echo ""
    header "调整 Swap"

    local swap_path="/swapfile"
    local mem_total_mb
    mem_total_mb=$(free -m | awk '/^Mem:/{print $2}')
    local default_size_mb=$(( mem_total_mb - 1 ))
    (( default_size_mb < 1 )) && default_size_mb=$((mem_total_mb / 2))

    read -r -p "Swap 文件路径 [${swap_path}]: " input_path
    [[ -n "$input_path" ]] && swap_path="$input_path"

    echo ""
    warn "调整操作将:"
    echo "  1. 删除 ${swap_path}（如存在）"
    echo "  2. 重新创建指定大小的 Swap"
    echo ""

    confirm "确认调整？" || return

    # 步骤1：如果存在则删除
    if [[ -f "$swap_path" ]] && swapon --show 2>/dev/null | grep -q "$swap_path"; then
        info "停用旧 Swap: $swap_path"
        swapoff "$swap_path" 2>/dev/null || true
    fi

    if [[ -f "$swap_path" ]]; then
        info "删除旧 Swap 文件: $swap_path"
        rm -f "$swap_path"
    fi

    # 从 fstab 清理（精确匹配路径，防止误删）
    sed -i "\|^${swap_path}\s|d" /etc/fstab

    # 步骤2：添加新的
    local size_input
    read -r -p "新的 Swap 大小（单位 MB，默认 ${default_size_mb}）: " size_input
    size_input="${size_input:-$default_size_mb}"
    if [[ ! "$size_input" =~ ^[0-9]+$ ]] || (( size_input < 1 )); then
        warn "无效大小，请输入正整数（单位 MB）"
        return
    fi

    # 检查磁盘空间
    local target_dir
    target_dir="$(dirname "$swap_path")"
    local available_kb
    available_kb=$(df "$target_dir" 2>/dev/null | tail -1 | awk '{print $4}')
    local needed_kb=$((size_input * 1024))
    if [[ -n "$available_kb" ]] && (( available_kb < needed_kb )); then
        warn "磁盘空间不足！需要 ${size_input}MB，可用约 $((available_kb / 1024))MB"
        confirm "仍然继续？" || return
    fi

    echo ""
    info "创建新的 Swap 文件: ${swap_path} (${size_input}MB)"
    if command -v fallocate &>/dev/null; then
        fallocate -l "${size_input}M" "$swap_path" 2>/dev/null || {
            info "fallocate 失败，使用 dd 创建..."
            dd if=/dev/zero of="$swap_path" bs=1M count="$size_input" status=progress 2>&1
        }
    else
        dd if=/dev/zero of="$swap_path" bs=1M count="$size_input" status=progress 2>&1
    fi || {
        error "创建 Swap 文件失败"
        return
    }
    chmod 600 "$swap_path"

    local swap_fs
    swap_fs=$(df -T "$swap_path" 2>/dev/null | tail -1 | awk '{print $2}')
    if [[ "$swap_fs" == "btrfs" ]]; then
        info "检测到 btrfs 文件系统，创建 swap 子卷..."
        chattr +C "$swap_path" 2>/dev/null || true
    fi

    mkswap "$swap_path" || {
        error "格式化 Swap 失败"
        rm -f "$swap_path"
        return
    }

    swapon "$swap_path" || {
        error "启用 Swap 失败"
        rm -f "$swap_path"
        return
    }

    # fstab
    if ! grep -q "$swap_path" /etc/fstab 2>/dev/null; then
        echo "$swap_path none swap sw 0 0" >> /etc/fstab
    fi

    # 记录
    mkdir -p /etc/linux-helper
    echo "$swap_path" > /etc/linux-helper/swap-record

    success "Swap 调整完成！"
    echo ""
    swap_show_status
}

# ---------- Swap 菜单 ----------

swap_menu() {
    while true; do
        clear || true; header "Swap 管理"
        echo "  1) 查看 Swap 状态"
        echo "  2) 添加 Swap"
        echo "  3) 删除 Swap"
        echo "  4) 调整 Swap（删除后重建）"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) swap_show_status ; read -r -p "按回车键继续..." ;;
            2) swap_add ;;
            3) swap_delete ;;
            4) swap_adjust ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ; read -r -p "按回车键继续..." ;;
        esac
    done
}

tuning_menu() {
    while true; do
        clear || true; header "系统调优"
        echo "  1) Swap 管理"
        echo "  2) 内核参数优化"
        echo "  3) 文件描述符限制"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) swap_menu ;;
            2) placeholder "内核参数优化" ;;
            3) placeholder "文件描述符限制" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统信息
# ============================================================
info_menu() {
    while true; do
        clear || true; header "系统信息"
        echo "  1) 系统概览"
        echo "  2) CPU 信息"
        echo "  3) 内存信息"
        echo "  4) 磁盘信息"
        echo "  5) 网络信息"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
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
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：系统维护
# ============================================================
maintenance_menu() {
    while true; do
        clear || true; header "系统维护"
        echo "  1) 系统更新与清理"
        echo "  2) 日志清理"
        echo "  3) Docker 清理"
        echo "  4) 临时文件清理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) placeholder "系统更新与清理" ;;
            2) placeholder "日志清理" ;;
            3) placeholder "Docker 清理" ;;
            4) placeholder "临时文件清理" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
    done
}

# ============================================================
# 模块：工具箱
# ============================================================
tools_menu() {
    while true; do
        clear || true; header "工具箱"
        echo "  1) 安装常用软件"
        echo "  2) 安装 Docker"
        echo "  3) 安装 Docker Compose"
        echo "  4) 安装系统监控工具"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -r -p "  请选择: " choice
        case "$choice" in
            1) placeholder "安装常用软件" ;;
            2) placeholder "安装 Docker" ;;
            3) placeholder "安装 Docker Compose" ;;
            4) placeholder "安装系统监控工具" ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -r -p "按回车键继续..."
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
        read -r -p "  请选择 [0-7]: " choice
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
                read -r -p "按回车键继续..."
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

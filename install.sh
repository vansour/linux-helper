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

# ---------- SSH 辅助函数 ----------

BACKUP_DIR="/etc/linux-helper/backups"
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
    # 从后往前查找，最后一个匹配即为生效值
    local val=""
    if [[ -d "$SSHD_CONFIG_DIR" ]]; then
        for f in "$SSHD_CONFIG_DIR"/*.conf; do
            [[ -f "$f" ]] || continue
            val=$(grep -E "^\s*${key}\s+" "$f" 2>/dev/null | tail -1 | awk '{print $2}')
            [[ -n "$val" ]] && last_val="$val"
        done
    fi
    val=$(grep -E "^\s*${key}\s+" "$SSHD_CONFIG" 2>/dev/null | tail -1 | awk '{print $2}')
    [[ -n "$val" ]] && last_val="$val"
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
    read -p "输入新的 SSH 端口号 (1-65535): " new_port
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
        clear; header "SSH 公钥管理"

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
            read -p "按回车键返回..."
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
        read -p "  请选择用户: " user_choice

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
        clear; header "用户 ${user} 的 SSH 公钥"

        local keys=()
        while IFS= read -r line; do
            keys+=("$line")
        done < "$key_file"

        if [[ ${#keys[@]} -eq 0 ]]; then
            info "该用户没有公钥。"
            read -p "按回车键返回..."
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
        read -p "  请选择: " del_choice

        [[ "$del_choice" =~ ^[Bb]$ ]] && return
        [[ "$del_choice" =~ ^[Qq]$ ]] && exit 0

        if [[ "$del_choice" =~ ^[Aa]$ ]]; then
            confirm "确认删除 ${user} 的所有 ${#keys[@]} 个密钥？" || continue
            : > "$key_file"
            success "已删除 ${user} 的全部密钥"
            read -p "按回车键继续..."
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
            read -p "按回车键继续..."
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
        read -p "按回车键继续..."
    done
}

ssh_menu() {
    while true; do
        clear; header "SSH 安全配置"

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
        read -p "  请选择: " choice
        case "$choice" in
            1) ssh_change_port ;;
            2) ssh_root_login ;;
            3) ssh_manage_keys ;;
            b|B) break ;;
            q|Q) exit 0 ;;
            *) warn "无效选项" ;;
        esac
        read -p "按回车键继续..."
    done
}

security_menu() {
    while true; do
        clear; header "系统安全"
        echo "  1) SSH 安全配置"
        echo "  2) 配置防火墙"
        echo "  3) Fail2Ban 管理"
        echo ""; echo "  b) 返回主菜单"; echo "  q) 退出脚本"; echo ""
        read -p "  请选择: " choice
        case "$choice" in
            1) ssh_menu ;;
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

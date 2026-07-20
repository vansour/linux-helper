# Linux Helper — Bash 到 Go 完整迁移计划

> 将 `install.sh`（~1850 行 Bash）完全重写为 Go 语言，零外部依赖，静态编译单文件分发。

## 为何迁移

| 对比 | Bash | Go |
|------|------|-----|
| 数据结构 | ❌ 只能靠字符串拼凑数组/对象 | ✅ struct + interface + 类型安全 |
| 错误处理 | ❌ `$?` + `|| true` 纠错 | ✅ `error` 显式处理 |
| 并发 | ❌ `&` 后台无法控制 | ✅ goroutine + channel |
| 跨平台分发 | ❌ 每条 curl 都执行一次解释 | ✅ 编译一次，到处运行 |
| 模块化 | ❌ source 多文件增加复杂度 | ✅ 天然包机制 |
| 测试 | ❌ 基本无测试框架 | ✅ `go test` 原生 |

## 项目结构

```
linux-helper/
├── main.go                       # 入口：CLI 路由
├── go.mod
├── go.sum
├── Makefile                      # 构建 / 交叉编译 / 发布
├── .goreleaser.yaml              # 自动发布到 GitHub Releases
├── cmd/
│   ├── root.go                   # cobra 根命令（默认进入交互菜单）
│   └── install.go                # --install / --uninstall
├── internal/
│   ├── tui/                      # 文本交互菜单引擎
│   │   └── menu.go               #   Menu 结构体：标题、选项、处理函数
│   ├── network/                  # 模块：网络优化
│   │   ├── bbr.go                #   BBR + bpftune 一键开启
│   │   └── ipv6.go               #   IPv6 启用/禁用/状态查看
│   ├── system/                   # 模块：系统配置
│   │   ├── timezone.go           #   时区设置（列表/搜索/手动）
│   │   └── chrony.go             #   Chrony 时间同步（安装/配置/验证）
│   ├── security/                 # 模块：系统安全
│   │   └── ssh.go                #   SSH 配置（端口/root登录/密钥管理）
│   ├── tuning/                   # 模块：系统调优
│   │   └── swap.go               #   Swap 管理（添加/删除/调整）
│   ├── info/                     # 模块：系统信息（新功能）
│   │   ├── overview.go           #   系统概览
│   │   ├── cpu.go                #   CPU 信息
│   │   ├── memory.go             #   内存信息
│   │   ├── disk.go               #   磁盘信息
│   │   └── network.go            #   网络信息
│   ├── maintenance/              # 模块：系统维护（新功能）
│   │   ├── update.go             #   系统更新与清理
│   │   ├── log.go                #   日志清理
│   │   ├── docker.go             #   Docker 清理
│   │   └── temp.go               #   临时文件清理
│   ├── tools/                    # 模块：工具箱（新功能）
│   │   ├── docker.go             #   安装 Docker
│   │   ├── compose.go            #   安装 Docker Compose
│   │   └── monitor.go            #   安装系统监控工具
│   └── shell/                    # 系统命令封装层
│       ├── exec.go               #   exec.Command 统一封装
│       ├── systemctl.go          #   systemctl 常用操作
│       ├── pkg.go                #   包管理器检测（apt/dnf/yum/zypper）
│       ├── sysctl.go             #   sysctl 内核参数读写
│       ├── grub.go               #   GRUB 配置编辑
│       └── backup.go             #   备份到 /etc/linux-helper/backups/
├── scripts/
│   └── install.sh                # 下载器（最终版，约 30 行）
└── README.md
```

## 菜单层级（与 Bash 版本完全对齐）

```
主菜单
├── 1) 网络优化
│   ├── 1) 启用 BBR + fq + bpftune
│   ├── 2) TCP 参数调优          [待开发]
│   ├── 3) 查看网络状态          [待开发]
│   └── 4) IPv6 管理
│       ├── 1) 查看 IPv6 状态
│       ├── 2) 禁用 IPv6
│       └── 3) 启用 IPv6
├── 2) 系统配置
│   ├── 1) 设置时区（含 Chrony）
│   ├── 2) 修改主机名            [待开发]
│   ├── 3) 配置语言环境          [待开发]
│   └── 4) Chrony 时间同步管理
│       ├── 1) 查看运行状态
│       ├── 2) 强制配置 Chrony
│       └── 3) 立即同步时间
├── 3) 系统安全
│   ├── 1) SSH 安全配置
│   │   ├── 1) 修改 SSH 端口
│   │   ├── 2) 开启 root 密码登录
│   │   └── 3) 管理 SSH 公钥
│   ├── 2) 配置防火墙            [待开发]
│   └── 3) Fail2Ban 管理         [待开发]
├── 4) 系统调优
│   ├── 1) Swap 管理
│   │   ├── 1) 查看 Swap 状态
│   │   ├── 2) 添加 Swap
│   │   ├── 3) 删除 Swap
│   │   └── 4) 调整 Swap
│   ├── 2) 内核参数优化          [待开发]
│   └── 3) 文件描述符限制        [待开发]
├── 5) 系统信息（全部新功能）
│   ├── 1) 系统概览
│   ├── 2) CPU 信息
│   ├── 3) 内存信息
│   ├── 4) 磁盘信息
│   └── 5) 网络信息
├── 6) 系统维护（全部新功能）
│   ├── 1) 系统更新与清理
│   ├── 2) 日志清理
│   ├── 3) Docker 清理
│   └── 4) 临时文件清理
└── 7) 工具箱（全部新功能）
    ├── 1) 安装常用软件
    ├── 2) 安装 Docker
    ├── 3) 安装 Docker Compose
    └── 4) 安装系统监控工具
```

## 核心设计

### 菜单引擎（internal/tui/menu.go）

```go
type Menu struct {
    Title   string
    Options []Option
}

type Option struct {
    Key     string                // "1", "2", "b", "q"
    Label   string                // 显示文本
    Handler func() error          // 处理函数（可选）
    Submenu *Menu                 // 子菜单（优先级高于 Handler）
    Back    bool                  // 返回上级
    Quit    bool                  // 退出
}
```

菜单递归渲染，按键后自动调度到子菜单或处理函数。无需关心菜单栈管理。

### 系统命令封装（internal/shell/exec.go）

```go
// Run 执行命令，返回 stdout
func Run(name string, args ...string) (string, error)

// RunPipe 带 stdin 管道输入
func RunPipe(stdin string, name string, args ...string) (string, error)

// RunSilent 静默执行，只关心成功/失败
func RunSilent(name string, args ...string) error
```

所有系统命令统一走此封装，方便测试时 mock。

### 包管理器抽象（internal/shell/pkg.go）

```go
type Manager struct {
    Binary string   // "apt-get", "dnf", "yum", "zypper"
    UpdateArgs []string
    InstallArgs []string
}

func Detect() *Manager     // 自动检测系统包管理器
func (m *Manager) Update() error
func (m *Manager) Install(pkg string) error
```

### 用户交互

**交互模式**（默认）：`sudo linux-helper` → 进入分层菜单，键盘选择

**子命令模式**（Go 独有优势，Bash 无法优雅做到）：

```bash
sudo linux-helper network bbr              # 直接开启 BBR
sudo linux-helper system timezone           # 进入时区设置
sudo linux-helper system timezone Asia/Shanghai   # 一步设置时区
sudo linux-helper security ssh port 2222    # 修改 SSH 端口
sudo linux-helper tuning swap create --size 2048  # 创建 Swap
sudo linux-helper info overview             # 查看系统概览
```

### 分发方式（用户侧体验不变）

```bash
# 方式一：curl 直接运行
sudo bash -c "$(curl -sSL https://github.com/vansour/linux-helper/releases/latest/download/install.sh)"

# 方式二：安装到系统（推荐）
sudo curl -sSL https://github.com/vansour/linux-helper/releases/latest/download/linux-helper \
  -o /usr/local/bin/linux-helper && sudo chmod +x /usr/local/bin/linux-helper
sudo linux-helper
```

`scripts/install.sh`（约 30 行）：

```bash
#!/bin/bash
# Linux Helper 下载安装 — 自动检测架构，下载对应二进制
VERSION="$(curl -sSL https://api.github.com/repos/vansour/linux-helper/releases/latest | grep tag_name | cut -d'"' -f4)"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64"  ;;
    *)       echo "不支持的架构: $ARCH"; exit 1 ;;
esac
curl -sSL "https://github.com/vansour/linux-helper/releases/download/$VERSION/linux-helper-linux-$ARCH" \
  -o /usr/local/bin/linux-helper
chmod +x /usr/local/bin/linux-helper
echo "安装完成！运行: sudo linux-helper"
```

### 二进制大小优化

```makefile
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/linux-helper .
# 编译后约 5MB，strip 后 ~2MB，upx 后 ~700KB
```

## 阶段划分

### Phase 1 — 基础设施（预计 2-3 天）

| 任务 | 输入（Bash 参考） | 输出（Go 文件） | 关键点 |
|------|-------------------|-----------------|--------|
| 初始化项目 | — | `go.mod`, `main.go` | 用 `go mod init` 初始化 |
| Cobra CLI 框架 | `install.sh` case 路由 | `cmd/root.go`, `cmd/install.go` | 子命令注册模式 |
| 菜单引擎 | `header()`, `case` 循环 | `internal/tui/menu.go` | 递归渲染、自动调度 |
| 命令执行封装 | `$()` 命令替换 | `internal/shell/exec.go` | `exec.Command` 统一接口 |
| 包管理器检测 | apt/dnf/yum/zypper 分支 | `internal/shell/pkg.go` | `exec.LookPath` 自动检测 |
| systemctl 封装 | `systemctl enable/start/status` | `internal/shell/systemctl.go` | 常用操作的函数化 |
| sysctl 读写 | `sysctl -n` / `sysctl -p` | `internal/shell/sysctl.go` | 读/写/持久化 |
| GRUB 配置编辑 | `sed -i` GRUB 命令行 | `internal/shell/grub.go` | `/etc/default/grub` 解析 |
| 备份工具 | `cp file backup` 模式 | `internal/shell/backup.go` | 统一 /etc/linux-helper/backups/ |
| Makefile | — | `Makefile` | build + cross-build + compress |
| GitHub Actions | — | `.github/workflows/release.yml` | 自动构建发布 |

### Phase 2a — Swap 管理（预计 1 天）

| 任务 | Bash 源函数 | 行数 |
|------|------------|------|
| `swap_show_status` | `cat /proc/swaps` + `free -h` + `fstab` | ~30 行 |
| `swap_add` | `fallocate` / `dd` → `mkswap` → `swapon` → `fstab` | ~100 行 |
| `swap_delete` | `swapoff` → `fstab` 清理 → 文件删除 | ~50 行 |
| `swap_adjust` | 组合 delete + add | ~80 行 |
| `swap_menu` | 菜单循环 | ~40 行 |

**关键调用**:
- `shell.Run("fallocate", "-l", size, path)`
- `shell.Run("mkswap", path)`
- `shell.Run("swapon", path)`
- `/etc/fstab` 追加行

### Phase 2b — BBR + bpftune（预计 1 天）

| 任务 | Bash 源函数 | 行数 |
|------|------------|------|
| `enable_bbr` | sysctl 备份 → BBR 配置 → 验证 → 安装 bpftune | ~110 行 |

**关键调用**:
- `shell.Sysctl` 读写 `net.core.default_qdisc` 和 `net.ipv4.tcp_congestion_control`
- 写 `/etc/sysctl.d/90-bbr.conf`
- `shell.PackageManager.Install("bpftune")`
- `shell.Systemctl.Enable("bpftune")`

### Phase 2c — IPv6 管理（预计 1 天）

| 任务 | Bash 源函数 | 行数 |
|------|------------|------|
| `ipv6_show_status` | 读取 sysctl + GRUB + hosts | ~60 行 |
| `ipv6_disable` | sysctl + GRUB + SSH + hosts 修改 | ~80 行 |
| `ipv6_enable` | 恢复全部修改 | ~80 行 |
| `ipv6_menu` | 菜单循环 | ~30 行 |

**关键调用**:
- 读 `/proc/sys/net/ipv6/conf/all/disable_ipv6`
- `shell.Grub` 编辑内核参数 `ipv6.disable=1`
- `shell.SSH` 编辑 `AddressFamily`
- `/etc/hosts` 文件中注释/取消注释 IPv6 行

### Phase 2d — SSH 配置（预计 2 天）

| 任务 | Bash 源函数 | 行数 |
|------|------------|------|
| `ssh_current_ports` | `ss -tlnp` 过滤 | ~5 行 |
| `ssh_find_setting` | grep SSH 配置 | ~15 行 |
| `ssh_get_setting` | 带覆盖优先级读取 | ~15 行 |
| `ssh_backup_all` | 备份 SSH 配置 | ~15 行 |
| `ssh_safe_restart` | `sshd -t` 验证 + restart | ~15 行 |
| `ssh_change_port` | 清理 Port 行 → 写入新端口 → 验证 | ~80 行 |
| `ssh_root_login` | PermitRootLogin + PasswordAuthentication | ~70 行 |
| `ssh_manage_keys` | 查找 authorized_keys 用户 | ~40 行 |
| `ssh_list_delete_keys` | 列出/删除公钥 | ~100 行 |
| `ssh_menu` | 菜单循环 | ~30 行 |

**关键设计**: SSH 配置文件的解析/编辑是迁移中最复杂的部分。Bash 用 `sed -i` 和 `grep` 逐行操作，Go 中用结构化的配置解析：

```go
// SSH 配置文件的表示
type SSHConfig struct {
    Path    string
    Entries []SSHEntry
}

type SSHEntry struct {
    Key   string
    Value string
}

// 读 /etc/ssh/sshd_config + drop-in 目录
// 按优先级合并
// 修改后写回（保留注释和空行）
```

### Phase 2e — 时区 & Chrony（预计 2 天）

| 任务 | Bash 源函数 | 行数 |
|------|------------|------|
| `timezone_show_status` | timedatectl + systemctl + chronyc | ~30 行 |
| `chrony_force_setup` | 安装 → 配置 → 验证（最复杂函数） | ~170 行 |
| `timezone_set` | 交互式选择 → 设置 → 配置 Chrony | ~140 行 |
| `chrony_menu` | 菜单循环 | ~40 行 |

**关键设计**:
- `timedatectl` 调用封装：设置时区、查看状态
- chrony 配置模板生成（替代 heredoc）
- `chronyd -t` 配置验证
- NTP 池按地理区域选择

### Phase 3 — 新功能开发（按需）

**系统信息** — 纯文件读取，不需要 shell 子进程：

```go
// internal/info/memory.go
func Memory() (*MemoryInfo, error) {
    data, err := os.ReadFile("/proc/meminfo")
    // 解析 MemTotal / MemFree / SwapTotal / SwapFree
}
```

| 功能 | 信息来源 |
|------|---------|
| 系统概览 | `uname -a`, `/etc/os-release`, `uptime` |
| CPU 信息 | `/proc/cpuinfo`, `lscpu` |
| 内存信息 | `/proc/meminfo` |
| 磁盘信息 | `df -h`, `lsblk` |
| 网络信息 | `ip addr`, `/proc/net/dev` |

### Phase 4 — Bash 退役（最后 0.5 天）

1. 删除 `install.sh`（Bash 本体）
2. 从 `scripts/` 复制 `install.sh` 到仓库根目录 → 仅 30 行下载器
3. 更新 `README.md`（安装命令不变）
4. 关闭指向 Bash 版本的 Issue

## 迁移风险与应对

| 风险 | 概率 | 应对 |
|------|------|------|
| Go 二进制 >3MB | 确定 | `-ldflags="-s -w"` + upx 压缩到 ~700KB |
| 非标准 Linux 缺少命令 | 低 | `exec.LookPath()` 预检，给中文错误提示 |
| `sed -i` 文件编辑在 Go 中更啰嗦 | 中 | 封装 `EditFile()` 一行调用 |
| 跨架构兼容性（x86/ARM） | 低 | CI 中同时编译 `linux/amd64` 和 `linux/arm64` |
| Bash→Go 功能对等验证 | 中 | 逐个模块输出 diff 对比，写 `go test` |
| 用户习惯 `bash <(curl ...)` 方式 | 低 | install.sh 下载器保持 100% 兼容 |

## 时间预估

| 阶段 | 工作量 | 日历时间 |
|------|--------|----------|
| Phase 1：基础设施 | ~500 行 Go | 2-3 天 |
| Phase 2a：Swap | ~400 行 Go | 1 天 |
| Phase 2b：BBR | ~250 行 Go | 1 天 |
| Phase 2c：IPv6 | ~350 行 Go | 1 天 |
| Phase 2d：SSH | ~500 行 Go | 2 天 |
| Phase 2e：时区 + Chrony | ~450 行 Go | 2 天 |
| Phase 3：新功能 | 按需 | 按需 |
| Phase 4：Bash 退役 | 30 行脚本 | 0.5 天 |
| **总计** | **~2500 行 Go** | **~10 天** |

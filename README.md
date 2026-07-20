# Linux Helper 🐧

> 一个实用的 Linux 服务器辅助管理工具，**Go 语言静态编译，单文件即用**。

![License](https://img.shields.io/badge/license-MIT-blue)
![Go Version](https://img.shields.io/badge/go-1.24-green)
![Release](https://img.shields.io/github/v/release/vansour/linux-helper)

---

## 功能模块

| 模块 | 功能 |
|------|------|
| 🌐 **网络优化** | BBR + bpftune 加速、IPv6 管理（启用/禁用/状态查看） |
| ⚙️ **系统配置** | 时区设置（列表/搜索/手动）、Chrony 时间同步 |
| 🔒 **系统安全** | SSH 端口修改、root 密码登录、公钥管理 |
| 🚀 **系统调优** | Swap 管理（添加/删除/调整/状态） |
| 📊 **系统信息** | 系统概览、CPU / 内存 / 磁盘 / 网络信息 |
| 🧹 **系统维护** | 系统更新、日志清理、Docker 清理、临时文件清理 |
| 📦 **工具箱** | 常用软件、Docker、Docker Compose 一键安装 |

---

## 安装

### 方式一：下载安装（推荐）

```bash
# 一行命令下载并安装
sudo bash -c "$(curl -sSL https://github.com/vansour/linux-helper/releases/latest/download/install.sh)"

# 之后直接运行
sudo linux-helper
```

### 方式二：从源码编译

```bash
git clone https://github.com/vansour/linux-helper.git
cd linux-helper
make build
sudo ./dist/linux-helper
```

---

## 使用

```bash
sudo linux-helper
```

进入交互菜单后，输入数字选择功能模块，按 `b` 返回上级，按 `q` 退出。

```
==================================================
       Linux Helper — 系统管理助手
==================================================

  1)  网络优化
  2)  系统配置
  3)  系统安全
  4)  系统调优
  5)  系统信息
  6)  系统维护
  7)  工具箱

  q)  退出

  请选择:
```

### 直接子命令模式（跳过菜单）

```bash
sudo linux-helper install       # 安装到系统
sudo linux-helper uninstall     # 卸载
```

---

## 卸载

```bash
sudo linux-helper uninstall
```

或直接删除二进制文件：

```bash
sudo rm /usr/local/bin/linux-helper
```

---

## 项目结构

```
linux-helper/
├── main.go                   # 入口
├── cmd/                      # CLI 命令
│   ├── root.go               # cobra 根命令
│   ├── menu.go               # 交互菜单组装
│   └── install.go            # 安装/卸载
├── internal/
│   ├── shell/                # 系统命令封装层
│   │   ├── exec.go           #   exec.Command 统一接口
│   │   ├── systemctl.go      #   systemd 服务管理
│   │   ├── pkg.go            #   包管理器（apt/dnf/yum/zypper）
│   │   ├── sysctl.go         #   内核参数读写
│   │   ├── grub.go           #   GRUB 配置编辑
│   │   ├── backup.go         #   系统备份
│   │   └── ui.go             #   终端 UI 辅助函数
│   ├── tui/                  # 交互菜单引擎
│   ├── network/              # 网络优化
│   ├── system/               # 系统配置
│   ├── security/             # 系统安全
│   ├── tuning/               # 系统调优
│   ├── info/                 # 系统信息
│   ├── maintenance/          # 系统维护
│   └── tools/                # 工具箱
├── scripts/install.sh        # 下载安装脚本
├── Makefile                  # 构建
├── .goreleaser.yaml          # 自动发布
└── MIGRATE.md                # 从 Bash 迁移到此版本的记录
```

---

## 开发

```bash
make build      # 编译当前平台
make build-all  # 交叉编译（linux/amd64, arm64, 386）
make test       # 运行测试
make lint       # 代码检查
```

## 环境要求

- **操作系统**: Linux（兼容 Ubuntu / Debian / CentOS / Rocky Linux / Alpine 等）
- **依赖**: 无（静态编译，零外部依赖）

---

## 协议

[MIT License](LICENSE)

---

<p align="center">
  Made with ❤️ for the Linux community
</p>

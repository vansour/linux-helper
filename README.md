# Linux Helper 🐧

> 一个实用的 Linux 服务器辅助管理脚本，单文件即用，告别繁琐的命令记忆。

![License](https://img.shields.io/badge/license-MIT-blue)
![Shell](https://img.shields.io/badge/shell-bash-green)
![Version](https://img.shields.io/badge/version-1.0.0-orange)

---

## 功能模块

| 模块 | 功能 |
|------|------|
| 🌐 **网络优化** | BBR 加速、TCP 参数调优、网络状态查看 |
| ⚙️ **系统配置** | 时区设置、主机名修改、语言环境配置 |
| 🔒 **系统安全** | SSH 加固、防火墙规则、Fail2Ban 管理 |
| 🚀 **系统调优** | Swap 管理、内核参数优化、文件描述符限制 |
| 📊 **系统信息** | CPU / 内存 / 磁盘 / 网络 / OS 一键概览 |
| 🧹 **系统维护** | 系统更新、日志清理、Docker 清理、临时文件清理 |
| 📦 **工具箱** | Docker、Compose、系统监控工具一键安装 |

> 💡 模块功能持续开发中，欢迎提交 Issue 或 PR。

---

## 快速开始

### 方式一：直接运行（无需安装）

```bash
# 一句话下载并运行
bash <(curl -sSL https://raw.githubusercontent.com/vansour/linux-helper/main/install.sh)
```

### 方式二：安装到系统

```bash
# 克隆或下载
git clone https://github.com/vansour/linux-helper.git
cd linux-helper

# 安装到系统目录，创建 linux-helper 命令
sudo bash install.sh --install

# 之后可直接运行
sudo linux-helper
```

### 方式三：本地直接运行

```bash
git clone https://github.com/vansour/linux-helper.git
cd linux-helper
sudo bash install.sh
```

---

## 使用

```bash
sudo linux-helper
```

进入主菜单后，输入数字选择功能模块，按 `b` 返回上级，按 `q` 或 `0` 退出。

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

  0)  退出脚本

  请选择 [0-7]:
```

---

## 卸载

```bash
sudo linux-helper --uninstall
# 或
sudo /usr/local/linux-helper/install.sh --uninstall
```

---

## 文件结构

```
linux-helper/
├── install.sh    ← 单文件脚本（含全部功能）
├── README.md     ← 本文件
└── VERSION       ← 版本号
```

整个脚本仅一个文件，即拷即用，无需依赖。

---

## 开发

脚本内部按模块分区组织，添加新功能只需在 `install.sh` 中按模板添加菜单和功能函数：

```bash
# 1. 添加菜单函数
example_menu() {
    while true; do
        clear; header "示例模块"
        echo "  1) 示例功能"
        echo "  b) 返回"
        read -p "请选择: " choice
        case "$choice" in
            1) placeholder "示例功能" ;;
            b|B) break ;;
        esac
        read -p "按回车键继续..."
    done
}

# 2. 在主菜单中添加跳转
# 找到 menu_main() 中的 case，追加:
#    x) example_menu ;;
```

---

## 环境要求

- **操作系统**: Linux（兼容 Ubuntu / Debian / CentOS / Rocky Linux 等主流发行版）
- **Shell**: Bash 4.0+
- **权限**: 部分功能需要 root 权限

---

## 协议

[MIT License](LICENSE)

---

<p align="center">
  Made with ❤️ for the Linux community
</p>

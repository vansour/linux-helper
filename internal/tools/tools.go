package tools

import (
	"fmt"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// InstallCommon installs common useful packages.
func InstallCommon() {
	shell.Header("安装常用软件")

	pkgs := []string{"curl", "wget", "git", "vim", "htop", "iotop", "net-tools", "dnsutils", "lsof", "tcpdump", "screen", "tree"}
	shell.Info("将安装: %s", fmt.Sprintf("%v", pkgs))
	if !shell.Confirm("确认安装？") {
		return
	}

	for _, pkg := range pkgs {
		shell.Info("正在安装 %s...", pkg)
		if err := shell.InstallPackage(pkg); err != nil {
			shell.Warn("%s 安装失败: %v", pkg, err)
		}
	}
	shell.Success("常用软件安装完成")
	fmt.Println("")
}

// InstallDocker installs Docker CE.
func InstallDocker() {
	shell.Header("安装 Docker")

	if shell.Has("docker") {
		shell.Success("Docker 已安装，跳过")
		fmt.Println("")
		return
	}

	if !shell.Confirm("确认安装 Docker？") {
		return
	}

	shell.Info("正在使用官方脚本安装 Docker...")
	if _, err := shell.Run("curl", "-fsSL", "https://get.docker.com"); err != nil {
		shell.Error("下载 Docker 安装脚本失败: %v", err)
		fmt.Println("")
		return
	}
	shell.RunSilent("sh", "-c", "curl -fsSL https://get.docker.com | sh")

	if shell.Has("docker") {
		shell.Success("Docker 安装成功")
		shell.RunSilent("systemctl", "enable", "--now", "docker")
	} else {
		shell.Warn("Docker 安装可能未完成，请手动检查")
	}
	fmt.Println("")
}

// InstallCompose installs Docker Compose.
func InstallCompose() {
	shell.Header("安装 Docker Compose")

	if shell.Has("docker-compose") {
		shell.Success("Docker Compose 已安装，跳过")
		fmt.Println("")
		return
	}

	if !shell.Confirm("确认安装 Docker Compose？") {
		return
	}

	arch, _ := shell.Run("uname", "-m")
	arch = strings.TrimSpace(arch)

	url := fmt.Sprintf("https://github.com/docker/compose/releases/latest/download/docker-compose-linux-%s", arch)
	shell.Info("正在下载 Docker Compose...")
	if _, err := shell.Run("curl", "-sSL", url, "-o", "/usr/local/bin/docker-compose"); err != nil {
		shell.Error("下载失败: %v", err)
		fmt.Println("")
		return
	}
	shell.RunSilent("chmod", "+x", "/usr/local/bin/docker-compose")
	shell.Success("Docker Compose 安装成功")
	fmt.Println("")
}

// InstallMonitor installs monitoring tools.
func InstallMonitor() {
	shell.Header("安装系统监控工具")

	items := []struct {
		name string
		cmd  string
		pkg  string
	}{
		{"netdata", "netdata", "netdata"},
		{"glances", "glances", "glances"},
		{"htop", "htop", "htop"},
		{"iftop", "iftop", "iftop"},
	}

	shell.Info("将安装以下监控工具:")
	for _, t := range items {
		if !shell.Has(t.cmd) {
			fmt.Printf("  - %s\n", t.name)
		}
	}
	fmt.Println("")

	if !shell.Confirm("确认安装？") {
		return
	}

	for _, t := range items {
		if !shell.Has(t.cmd) {
			shell.Info("正在安装 %s...", t.name)
			if err := shell.InstallPackage(t.pkg); err != nil {
				shell.Warn("%s 安装失败: %v", t.name, err)
			} else {
				shell.Success("%s 安装成功", t.name)
			}
		} else {
			shell.Info("%s 已安装，跳过", t.name)
		}
	}
	shell.Success("监控工具安装完成")
	fmt.Println("")
}

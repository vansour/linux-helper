package maintenance

import (
	"fmt"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// UpdateSystem performs system update and cleanup.
func UpdateSystem() {
	shell.Header("系统更新与清理")

	pm := shell.DetectPackageManager()
	if pm == nil {
		shell.Error("不支持的包管理器")
		return
	}

	shell.Info("检测到包管理器: %s", pm.Name)
	shell.Info("正在更新软件包列表...")

	switch pm.Name {
	case "apt":
		shell.RunSilent("apt", "update")
		shell.Info("正在升级软件包...")
		shell.RunSilent("apt", "upgrade", "-y")
		shell.Info("正在清理...")
		shell.RunSilent("apt", "autoremove", "-y")
		shell.RunSilent("apt", "autoclean")
	case "dnf", "yum":
		shell.Info("正在升级软件包...")
		shell.RunSilent(pm.Binary, "upgrade", "-y")
		shell.Info("正在清理...")
		shell.RunSilent(pm.Binary, "autoremove", "-y")
		shell.RunSilent(pm.Binary, "clean", "all")
	case "zypper":
		shell.RunSilent("zypper", "refresh")
		shell.Info("正在升级软件包...")
		shell.RunSilent("zypper", "update", "-y")
		shell.Info("正在清理...")
		shell.RunSilent("zypper", "clean")
	}

	shell.Success("系统更新完成")
	fmt.Println("")
}

// CleanLogs cleans old system logs.
func CleanLogs() {
	shell.Header("日志清理")

	if shell.Has("journalctl") {
		shell.Info("清理 journalctl 日志（保留最近 7 天）...")
		shell.RunSilent("journalctl", "--vacuum-time=7d")
		shell.Success("journalctl 日志已清理")
	}

	shell.Info("清理 /var/log 中的旧日志...")
	files, _ := shell.Run("find", "/var/log", "-name", "*.gz", "-o", "-name", "*.old", "-o", "-name", "*.1")
	for _, f := range strings.Split(strings.TrimSpace(files), "\n") {
		if f != "" {
			shell.RunSilent("rm", "-f", f)
		}
	}
	shell.Success("/var/log 旧日志已清理")
	fmt.Println("")
}

// CleanDocker cleans Docker resources.
func CleanDocker() {
	shell.Header("Docker 清理")

	if !shell.Has("docker") {
		shell.Warn("Docker 未安装")
		fmt.Println("")
		return
	}

	shell.Info("清理未使用的 Docker 资源...")
	shell.RunSilent("docker", "system", "prune", "-f", "--volumes")
	shell.Success("Docker 清理完成")
	fmt.Println("")
}

// CleanTemp cleans temporary files.
func CleanTemp() {
	shell.Header("临时文件清理")

	dirs := []string{"/tmp", "/var/tmp"}
	for _, dir := range dirs {
		shell.Info("清理 %s 中 7 天前的文件...", dir)
		shell.RunSilent("find", dir, "-type", "f", "-atime", "+7", "-delete")
	}
	shell.Success("临时文件清理完成")
	fmt.Println("")
}

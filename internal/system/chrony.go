package system

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vansour/linux-helper/internal/shell"
)

const (
	chronyConfPath    = "/etc/chrony.conf"
	chronyAltConfPath = "/etc/chrony/chrony.conf"
	chronyDriftfile   = "/var/lib/chrony/drift"
	chronyLogdir      = "/var/log/chrony"
)

// ChronyShowConfig displays chrony status, tracking, and source information.
func ChronyShowConfig() {
	fmt.Println()
	shell.Header("Chrony 时间同步状态")

	chronyd := &shell.SystemdService{Name: "chronyd", Alt: "chrony"}
	if chronyd.IsActive() {
		shell.Success("chrony 服务运行中 ✓")
	} else {
		shell.Warn("chrony 服务未运行")
	}

	if shell.Has("chronyc") {
		fmt.Println()
		fmt.Printf("  %s\n", shell.Blue("chronyc tracking:"))
		tracking, _ := shell.Run("chronyc", "tracking")
		if tracking != "" {
			for _, line := range strings.Split(tracking, "\n") {
				fmt.Printf("    %s\n", line)
			}
		} else {
			shell.Info("chrony 未运行")
		}

		fmt.Println()
		fmt.Printf("  %s\n", shell.Blue("chronyc sources -v:"))
		sources, _ := shell.Run("chronyc", "sources", "-v")
		if sources != "" {
			for _, line := range strings.Split(sources, "\n") {
				fmt.Printf("    %s\n", line)
			}
		} else {
			shell.Info("chrony 未运行")
		}
	} else {
		shell.Warn("chronyc 未安装")
	}
	fmt.Println()
}

// SetupChrony installs, configures, and starts chrony for NTP time synchronization.
// The userTz parameter is used to select the nearest NTP pool.
func SetupChrony(userTz string) error {
	fmt.Println()
	shell.Header("强制使用 Chrony 时间同步")

	// 1. Install chrony
	if !shell.Has("chronyd") || !shell.Has("chronyc") {
		shell.Info("正在安装 chrony...")
		if err := shell.InstallPackage("chrony"); err != nil {
			shell.Error("chrony 安装失败")
			return fmt.Errorf("chrony install failed: %w", err)
		}
		shell.Success("chrony 安装成功")
	} else {
		shell.Success("chrony 已安装")
	}

	// 2. Stop and disable conflicting NTP services
	conflictFound := false
	conflictingServices := []*shell.SystemdService{
		{Name: "systemd-timesyncd"},
		{Name: "ntpd"},
		{Name: "openntpd"},
		{Name: "ntp"},
	}
	for _, svc := range conflictingServices {
		if svc.IsActive() {
			shell.Warn("停止冲突服务: %s", svc.Name)
			_ = svc.Stop()
			conflictFound = true
		}
		if svc.IsEnabled() {
			shell.Info("禁用冲突服务: %s", svc.Name)
			svc.Disable()
		}
	}

	shell.RunSilent("timedatectl", "set-ntp", "false")

	if conflictFound {
		shell.Success("已停用冲突的 NTP 服务")
	}

	// 3. Determine chrony config path and backup
	configPath := resolveChronyConfigPath()
	if configPath == "" {
		configPath = chronyConfPath
	}

	if _, err := os.Stat(configPath); err == nil {
		bakPath, err := shell.BackupFile(configPath)
		if err == nil {
			shell.Info("已备份 chrony 配置: %s", bakPath)
		}
	}

	// 4. Determine NTP pool based on timezone region
	ntpPool := resolveNTPPool(userTz)

	// 5. Write chrony configuration
	config := generateChronyConfig(userTz, ntpPool)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		shell.Error("写入 chrony 配置失败: %s", err.Error())
		return fmt.Errorf("write chrony config failed: %w", err)
	}
	shell.Success("chrony 配置文件已更新 (%s)", configPath)

	// 6. Validate configuration syntax
	if shell.Has("chronyd") {
		if err := shell.RunSilent("chronyd", "-t", "-f", configPath); err != nil {
			shell.Error("chrony 配置语法错误！请检查 %s", configPath)
			return fmt.Errorf("chrony config validation failed: %w", err)
		}
		shell.Success("chrony 配置语法正确")
	}

	// 7. Enable and start chrony service
	shell.Info("启动 chronyd 服务...")
	chronyd := &shell.SystemdService{Name: "chronyd", Alt: "chrony"}
	if err := chronyd.Enable(); err != nil {
		shell.Warn("chrony 服务启用失败，请手动检查: systemctl enable chronyd")
	}
	if err := chronyd.Restart(); err != nil {
		shell.Warn("chrony 服务启动失败，请手动检查: systemctl status chronyd")
	}

	// 8. Wait for chronyd socket to be ready, then sync time
	time.Sleep(1 * time.Second)
	shell.RunSilent("chronyc", "makestep")

	// 9. Show status
	fmt.Println()
	if chronyd.IsActive() {
		shell.Success("chrony 服务运行中 ✓")
		if shell.Has("chronyc") {
			fmt.Println()
			fmt.Printf("  %s\n", shell.Blue("chronyc tracking:"))
			tracking, _ := shell.Run("chronyc", "tracking")
			if tracking != "" {
				for _, line := range strings.Split(tracking, "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
			fmt.Println()
			fmt.Printf("  %s\n", shell.Blue("chronyc sources -v:"))
			sources, _ := shell.Run("chronyc", "sources", "-v")
			if sources != "" {
				for _, line := range strings.Split(sources, "\n") {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	} else {
		shell.Warn("chrony 服务状态异常，请手动检查:")
		fmt.Println("    systemctl status chronyd")
		fmt.Println("    journalctl -u chronyd --no-pager -n 30")
	}

	fmt.Println()
	if chronyd.IsActive() {
		shell.Success("Chrony 配置完成！系统时间将自动同步。")
	} else {
		shell.Warn("Chrony 配置过程有异常，请检查以上错误信息。")
	}

	return nil
}

// SyncTime immediately steps the system clock via chronyc makestep.
func SyncTime() {
	if !shell.Has("chronyc") {
		shell.Warn("chrony 未安装，请先配置 chrony")
		return
	}
	shell.Info("正在同步时间...")
	if err := shell.RunSilent("chronyc", "makestep"); err == nil {
		shell.Success("时间已同步")
	} else {
		shell.Warn("同步失败，chrony 可能未运行")
	}
}

// resolveChronyConfigPath returns the path to the chrony configuration file.
func resolveChronyConfigPath() string {
	if _, err := os.Stat(chronyConfPath); err == nil {
		return chronyConfPath
	}
	if _, err := os.Stat(chronyAltConfPath); err == nil {
		return chronyAltConfPath
	}
	// Default to standard path if neither exists
	return chronyConfPath
}

// resolveNTPPool returns the closest NTP pool name based on timezone region.
func resolveNTPPool(userTz string) string {
	region := ""
	if userTz != "" {
		region = strings.SplitN(userTz, "/", 2)[0]
	}

	switch region {
	case "Asia":
		return "asia.pool.ntp.org"
	case "Europe":
		return "europe.pool.ntp.org"
	case "America", "US", "Canada":
		return "north-america.pool.ntp.org"
	case "Pacific", "Australia":
		return "oceania.pool.ntp.org"
	case "Africa":
		return "africa.pool.ntp.org"
	default:
		return "pool.ntp.org"
	}
}

// generateChronyConfig returns the complete chrony configuration file content.
func generateChronyConfig(userTz, ntpPool string) string {
	currentTz := userTz
	if currentTz == "" {
		currentTz = "UTC"
	}

	return fmt.Sprintf(`# Chrony Configuration File — Linux Helper
# Timezone: %s

# NTP server pool (auto-selected based on timezone region)
pool %s iburst

# Fallback NTP servers
pool pool.ntp.org iburst
server ntp.aliyun.com iburst
server time.google.com iburst

# Drift file (records clock frequency deviation)
driftfile %s

# Fast sync — step time if drift >1 sec within first 3 updates
makestep 1.0 3

# Hardware clock synchronization
rtcsync

# Local command listening (allow chronyc from localhost only)
bindcmdaddress 127.0.0.1
bindcmdaddress ::1

# Log directory
logdir %s
`, currentTz, ntpPool, chronyDriftfile, chronyLogdir)
}

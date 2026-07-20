package network

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

const hostsFile = "/etc/hosts"

// isIPv6Addr checks if a trimmed hosts line starts with an IPv6 address.
// Matches standard IPv6 address prefixes: 2000::/3, fc00::/7, fe80::/10, ff00::/8, ::1, etc.
func isIPv6Addr(s string) bool {
	// ::1 loopback
	if strings.HasPrefix(s, "::") {
		return true
	}
	// Any hex-prefixed IPv6 address: ends with ":" or continues with hex digits + ":"
	if len(s) >= 3 && s[0] >= '0' && s[0] <= '9' {
		return strings.Contains(s, ":")
	}
	if len(s) >= 2 {
		c := s[0]
		if (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			return strings.Contains(s, ":")
		}
	}
	return false
}

// ShowIPv6Status displays current IPv6 configuration.
func ShowIPv6Status() error {
	shell.Header("当前 IPv6 状态")

	disabled, _ := shell.SysctlGetBool("net.ipv6.conf.all.disable_ipv6")
	if disabled {
		fmt.Printf("  系统 IPv6: %s\n", shell.Red("已禁用"))
	} else {
		fmt.Printf("  系统 IPv6: %s\n", shell.Green("已启用"))
	}

	fmt.Println("")
	fmt.Println("  GRUB 参数:")
	grubData, _ := os.ReadFile("/etc/default/grub")
	grubContent := string(grubData)
	if strings.Contains(grubContent, "ipv6.disable=1") {
		fmt.Printf("    %s (在 GRUB 中禁用)\n", shell.Yellow("ipv6.disable=1"))
	} else if strings.Contains(grubContent, "ipv6.disable=0") {
		fmt.Printf("    %s (在 GRUB 中启用)\n", shell.Green("ipv6.disable=0"))
	} else {
		fmt.Printf("    %s\n", shell.Blue("未设置"))
	}

	fmt.Println("")
	fmt.Println("  sysctl 配置:")
	sysctlFiles := findSysctlFilesWithKey("disable_ipv6")
	for _, f := range sysctlFiles {
		data, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "disable_ipv6") {
				fmt.Printf("    %s: %s\n", f, shell.Yellow(strings.TrimSpace(line)))
			}
		}
	}
	if len(sysctlFiles) == 0 {
		fmt.Println("    (无)")
	}

	fmt.Println("")
	fmt.Println("  /etc/hosts IPv6 记录:")
	data, _ := os.ReadFile(hostsFile)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && isIPv6Addr(trimmed) {
			found = true
			if strings.HasPrefix(trimmed, "#") {
				fmt.Printf("    %s %s\n", shell.Yellow("(已注释)"), line)
			} else {
				fmt.Printf("    %s %s\n", shell.Green(""), line)
			}
		}
	}
	if !found {
		fmt.Println("    (无)")
	}

	fmt.Println("")
	if shell.Has("ip") {
		fmt.Println("  网卡 IPv6 地址:")
		out, _ := shell.Run("ip", "-6", "addr", "show")
		lines := strings.Split(out, "\n")
		count := 0
		for _, l := range lines {
			if strings.Contains(l, "inet6") {
				if count >= 5 {
					break
				}
				fmt.Printf("    %s\n", l)
				count++
			}
		}
		if count == 0 {
			fmt.Println("    (无)")
		}
	}
	fmt.Println("")
	return nil
}

// DisableIPv6 disables IPv6 system-wide.
func DisableIPv6() error {
	shell.Header("禁用 IPv6")

	if !shell.Confirm("禁用 IPv6？这将修改 sysctl、GRUB、/etc/hosts 和 SSH 配置") {
		return nil
	}

	ts := shell.Timestamp()
	backupDir := "/etc/linux-helper/backups/ipv6-backup-" + ts
	os.MkdirAll(backupDir, 0755)

	// 1. Backup /etc/hosts
	shell.BackupFile(hostsFile)

	// 2. Modify /etc/hosts — comment out IPv6 entries
	shell.Info("处理 /etc/hosts 中的 IPv6 记录...")
	data, _ := os.ReadFile(hostsFile)
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isIPv6Addr(trimmed) {
			newLines = append(newLines, "#"+line)
		} else {
			newLines = append(newLines, line)
		}
	}
	os.WriteFile(hostsFile, []byte(strings.Join(newLines, "\n")), 0644)
	shell.Success("/etc/hosts 已处理")

	// 3. sysctl config
	shell.Info("配置 sysctl 禁用 IPv6...")
	os.MkdirAll("/etc/sysctl.d", 0755)

	for _, f := range findSysctlFilesWithKey("disable_ipv6") {
		data, _ := os.ReadFile(f)
		os.WriteFile(filepath.Join(backupDir, filepath.Base(f)), data, 0644)
		var filtered []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "disable_ipv6") {
				filtered = append(filtered, line)
			}
		}
		os.WriteFile(f, []byte(strings.Join(filtered, "\n")), 0644)
	}

	shell.SysctlPersist("/etc/sysctl.d/99-lh-disable-ipv6.conf",
		"net.ipv6.conf.all.disable_ipv6", "1")
	shell.SysctlPersist("/etc/sysctl.d/99-lh-disable-ipv6.conf",
		"net.ipv6.conf.default.disable_ipv6", "1")
	shell.Success("sysctl 已生效")

	// 4. SSH — restrict to IPv4
	shell.Info("配置 SSH 仅监听 IPv4...")
	sshdConfig := "/etc/ssh/sshd_config"
	data, _ = os.ReadFile(sshdConfig)
	lines = strings.Split(string(data), "\n")
	var sshdLines []string
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "AddressFamily") {
			sshdLines = append(sshdLines, "AddressFamily inet")
			found = true
		} else {
			sshdLines = append(sshdLines, line)
		}
	}
	if !found {
		sshdLines = append(sshdLines, "AddressFamily inet")
	}
	os.WriteFile(sshdConfig, []byte(strings.Join(sshdLines, "\n")), 0644)

	dropDir := "/etc/ssh/sshd_config.d"
	if entries, err := os.ReadDir(dropDir); err == nil {
		for _, e := range entries {
			fp := dropDir + "/" + e.Name()
			data, _ := os.ReadFile(fp)
			var filtered []string
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "AddressFamily") {
					filtered = append(filtered, line)
				}
			}
			os.WriteFile(fp, []byte(strings.Join(filtered, "\n")+"\n"), 0644)
		}
	}

	if _, err := shell.Run("sshd", "-t"); err == nil {
		shell.RunSilent("systemctl", "restart", "sshd")
	} else {
		shell.RunSilent("systemctl", "restart", "ssh")
	}
	shell.Success("SSH 已配置仅 IPv4")

	// 5. GRUB
	shell.Info("配置 GRUB 内核参数...")
	if err := shell.GrubSetParam("ipv6.disable", "1"); err != nil {
		shell.Warn("GRUB 配置更新失败: %v", err)
	} else {
		shell.Info("GRUB 配置已更新（需要 grub-mkconfig -o /boot/grub/grub.cfg 后重启生效）")
		shell.Info("可运行 update-grub 更新 GRUB（可选，重启生效）")
	}

	fmt.Println("")
	shell.Success("IPv6 禁用配置完成！部分更改需重启后完全生效。")
	fmt.Println("")
	ShowIPv6Status()
	return nil
}

// EnableIPv6 re-enables IPv6 system-wide.
func EnableIPv6() error {
	shell.Header("启用 IPv6")

	if !shell.Confirm("启用 IPv6？这将恢复 sysctl、GRUB、/etc/hosts 和 SSH 配置") {
		return nil
	}

	// 1. Restore /etc/hosts
	shell.Info("恢复 /etc/hosts 中的 IPv6 记录...")
	data, _ := os.ReadFile(hostsFile)
	lines := strings.Split(string(data), "\n")
	var newLines []string
	hasIpv6 := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Uncomment lines that were commented by DisableIPv6
		if strings.HasPrefix(trimmed, "#") && isIPv6Addr(strings.TrimSpace(trimmed[1:])) {
			newLines = append(newLines, trimmed[1:])
			hasIpv6 = true
		} else if isIPv6Addr(trimmed) {
			newLines = append(newLines, line)
			hasIpv6 = true
		} else {
			newLines = append(newLines, line)
		}
	}
	if !hasIpv6 {
		newLines = append(newLines, "::1     localhost ip6-localhost ip6-loopback")
		newLines = append(newLines, "ff02::1 ip6-allnodes")
		newLines = append(newLines, "ff02::2 ip6-allrouters")
	}
	os.WriteFile(hostsFile, []byte(strings.Join(newLines, "\n")), 0644)
	shell.Success("/etc/hosts 已恢复")

	// 2. sysctl enable IPv6
	shell.Info("配置 sysctl 启用 IPv6...")
	for _, f := range findSysctlFilesWithKey("disable_ipv6") {
		data, _ := os.ReadFile(f)
		var filtered []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "disable_ipv6") {
				filtered = append(filtered, line)
			}
		}
		os.WriteFile(f, []byte(strings.Join(filtered, "\n")), 0644)
	}
	os.Remove("/etc/sysctl.d/99-lh-disable-ipv6.conf")
	shell.SysctlSet("net.ipv6.conf.all.disable_ipv6", "0")
	shell.SysctlSet("net.ipv6.conf.default.disable_ipv6", "0")
	shell.Info("sysctl 已清理")

	// 3. SSH restore all address families
	shell.Info("恢复 SSH 监听所有地址...")
	sshdConfig := "/etc/ssh/sshd_config"
	data, _ = os.ReadFile(sshdConfig)
	lines = strings.Split(string(data), "\n")
	var sshdLines []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "AddressFamily") {
			sshdLines = append(sshdLines, line)
		}
	}
	os.WriteFile(sshdConfig, []byte(strings.Join(sshdLines, "\n")), 0644)

	dropDir := "/etc/ssh/sshd_config.d"
	if entries, err := os.ReadDir(dropDir); err == nil {
		for _, e := range entries {
			fp := dropDir + "/" + e.Name()
			data, _ := os.ReadFile(fp)
			var filtered []string
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "AddressFamily") {
					filtered = append(filtered, line)
				}
			}
			os.WriteFile(fp, []byte(strings.Join(filtered, "\n")+"\n"), 0644)
		}
	}

	if _, err := shell.Run("sshd", "-t"); err == nil {
		shell.RunSilent("systemctl", "restart", "sshd")
	} else {
		shell.RunSilent("systemctl", "restart", "ssh")
	}
	shell.Success("SSH 已恢复监听所有地址")

	// 4. GRUB
	shell.Info("配置 GRUB 移除 ipv6.disable...")
	if err := shell.GrubRemoveParam("ipv6.disable"); err != nil {
		shell.Warn("GRUB 更新失败: %v", err)
	} else {
		shell.Success("GRUB 已更新")
	}

	fmt.Println("")
	shell.Success("IPv6 启用配置完成！部分更改需重启后完全生效。")
	fmt.Println("")
	ShowIPv6Status()
	return nil
}

// findSysctlFilesWithKey returns sysctl.d files and /etc/sysctl.conf that contain the given key.
func findSysctlFilesWithKey(key string) []string {
	var result []string
	candidates := []string{"/etc/sysctl.conf"}
	globFiles, _ := filepath.Glob("/etc/sysctl.d/*.conf")
	candidates = append(candidates, globFiles...)
	for _, f := range candidates {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), key) {
			result = append(result, f)
		}
	}
	return result
}

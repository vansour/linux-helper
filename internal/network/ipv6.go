package network

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

const hostsFile = "/etc/hosts"

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
	grubParams, _ := shell.Run("grep", "ipv6.disable", "/etc/default/grub")
	if strings.Contains(grubParams, "ipv6.disable=1") {
		fmt.Printf("    %s (在 GRUB 中禁用)\n", shell.Yellow("ipv6.disable=1"))
	} else if strings.Contains(grubParams, "ipv6.disable=0") {
		fmt.Printf("    %s (在 GRUB 中启用)\n", shell.Green("ipv6.disable=0"))
	} else {
		fmt.Printf("    %s\n", shell.Blue("未设置"))
	}

	fmt.Println("")
	fmt.Println("  sysctl 配置:")
	files, _ := shell.Run("grep", "-rl", "disable_ipv6", "/etc/sysctl.d/", "/etc/sysctl.conf")
	for _, f := range strings.Split(strings.TrimSpace(files), "\n") {
		if f == "" {
			continue
		}
		line, _ := shell.Run("grep", "disable_ipv6", f)
		fmt.Printf("    %s: %s\n", f, shell.Yellow(line))
	}
	if strings.TrimSpace(files) == "" {
		fmt.Println("    (无)")
	}

	fmt.Println("")
	fmt.Println("  /etc/hosts IPv6 记录:")
	data, _ := os.ReadFile(hostsFile)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "::") {
			found = true
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
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
		if strings.HasPrefix(trimmed, "::1") || strings.HasPrefix(trimmed, "ff02::") {
			newLines = append(newLines, "#"+line)
		} else if len(trimmed) >= 4 && strings.Contains(trimmed, ":") &&
			(trimmed[0] >= 'a' && trimmed[0] <= 'f' || trimmed[0] >= '0' && trimmed[0] <= '9') {
			// Also comment other IPv6 address lines
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

	// Clean disable_ipv6 from all sysctl files
	sysctlFiles, _ := shell.Run("grep", "-rl", "disable_ipv6", "/etc/sysctl.d/", "/etc/sysctl.conf")
	for _, f := range strings.Split(strings.TrimSpace(sysctlFiles), "\n") {
		if f == "" {
			continue
		}
		shell.Run("cp", f, backupDir+"/")
		data, _ := os.ReadFile(f)
		var filtered []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "disable_ipv6") {
				filtered = append(filtered, line)
			}
		}
		os.WriteFile(f, []byte(strings.Join(filtered, "\n")), 0644)
	}

	// Write unified disable config
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

	// Also clean drop-in AddressFamily
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

	// Restart SSH
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
		if strings.HasPrefix(trimmed, "#::1") || strings.HasPrefix(trimmed, "#ff02::") {
			newLines = append(newLines, trimmed[1:])
			hasIpv6 = true
		} else if strings.HasPrefix(trimmed, "::1") || strings.HasPrefix(trimmed, "ff02::") {
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
	sysctlFiles, _ := shell.Run("grep", "-rl", "disable_ipv6", "/etc/sysctl.d/", "/etc/sysctl.conf")
	for _, f := range strings.Split(strings.TrimSpace(sysctlFiles), "\n") {
		if f == "" {
			continue
		}
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

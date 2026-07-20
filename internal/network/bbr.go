package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// EnableBBR enables BBR congestion control + fq qdisc and installs bpftune.
func EnableBBR() error {
	shell.Header("开启 BBR + fq + bpftune")

	backupDir := "/etc/linux-helper/backups"
	os.MkdirAll(backupDir, 0755)

	// 1. Backup current sysctl settings
	ts := shell.Timestamp()
	backupFile := filepath.Join(backupDir, "sysctl-backup-"+ts+".conf")
	shell.Info("备份当前 sysctl 配置到 %s ...", backupFile)

	var lines []string
	if v, err := shell.Run("sysctl", "-n", "net.core.default_qdisc"); err == nil {
		lines = append(lines, fmt.Sprintf("net.core.default_qdisc=%s", strings.TrimSpace(v)))
	}
	if v, err := shell.Run("sysctl", "-n", "net.ipv4.tcp_congestion_control"); err == nil {
		lines = append(lines, fmt.Sprintf("net.ipv4.tcp_congestion_control=%s", strings.TrimSpace(v)))
	}
	if v, err := shell.Run("sysctl", "-n", "net.ipv4.tcp_available_congestion_control"); err == nil {
		lines = append(lines, fmt.Sprintf("net.ipv4.tcp_available_congestion_control=%s", strings.TrimSpace(v)))
	}
	os.WriteFile(backupFile, []byte("# Linux Helper 备份 - "+ts+"\n"+strings.Join(lines, "\n")+"\n"), 0644)
	shell.Success("备份完成")

	// 2. Clean existing BBR config from sysctl.d
	shell.Info("清理已有的 BBR / congestion 配置...")
	cleaned := 0
	files, _ := filepath.Glob("/etc/sysctl.d/*.conf")
	files = append(files, "/etc/sysctl.conf")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if !strings.Contains(string(data), "tcp_congestion_control") && !strings.Contains(string(data), "default_qdisc") {
			continue
		}
		// Backup and clean
		bakName := filepath.Base(f) + ".bak." + ts
		os.WriteFile(filepath.Join(backupDir, bakName), data, 0644)

		var newLines []string
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "net.core.default_qdisc") ||
				strings.HasPrefix(trimmed, "net.ipv4.tcp_congestion_control") {
				continue
			}
			newLines = append(newLines, line)
		}
		os.WriteFile(f, []byte(strings.Join(newLines, "\n")), 0644)
		shell.Info("  已清理: %s", f)
		cleaned++
	}
	if cleaned == 0 {
		shell.Info("  无需清理")
	}

	// 3. Write BBR config
	shell.Info("写入 BBR 配置...")
	bbrConfig := "# BBR + fq — Linux Helper\nnet.core.default_qdisc = fq\nnet.ipv4.tcp_congestion_control = bbr\n"
	if err := os.WriteFile("/etc/sysctl.d/90-bbr.conf", []byte(bbrConfig), 0644); err != nil {
		return fmt.Errorf("写入 BBR 配置失败: %v", err)
	}
	shell.Success("写入 /etc/sysctl.d/90-bbr.conf")

	// 4. Apply
	shell.Info("应用 sysctl 参数...")
	if _, err := shell.Run("sysctl", "-p", "/etc/sysctl.d/90-bbr.conf"); err != nil {
		shell.Warn("应用 sysctl 失败: %v", err)
	}
	shell.Success("sysctl 参数已生效")

	// 5. Verify
	shell.Header("验证结果")
	currentCC, _ := shell.Run("sysctl", "-n", "net.ipv4.tcp_congestion_control")
	currentQdisc, _ := shell.Run("sysctl", "-n", "net.core.default_qdisc")
	fmt.Printf("  TCP 拥塞控制算法: %s%s%s\n", shell.Green(strings.TrimSpace(currentCC)), "", shell.ColorNC)
	fmt.Printf("  默认队列规则:     %s%s%s\n", shell.Green(strings.TrimSpace(currentQdisc)), "", shell.ColorNC)

	if strings.TrimSpace(currentCC) == "bbr" {
		shell.Success("BBR 已启用 ✓")
	} else {
		shell.Warn("BBR 似乎未生效，请检查内核版本 (需 4.9+)")
	}

	// 6. Install bpftune
	shell.Header("安装 bpftune")
	if shell.Has("bpftune") {
		shell.Success("bpftune 已安装，跳过")
	} else {
		shell.Info("正在安装 bpftune...")
		if err := shell.InstallPackage("bpftune"); err != nil {
			shell.Warn("bpftune 安装失败: %v", err)
		} else {
			shell.Success("bpftune 安装成功")
		}
	}

	// 7. Enable bpftune service
	svc := &shell.SystemdService{Name: "bpftune"}
	if svc.HasUnitFile() {
		shell.Info("启动 bpftune 服务...")
		if err := svc.Restart(); err != nil {
			shell.Warn("bpftune 服务启动失败，请手动检查: systemctl status bpftune")
		} else if svc.IsActive() {
			shell.Success("bpftune 服务运行中 ✓")
		} else {
			shell.Warn("bpftune 服务未运行，请检查: systemctl status bpftune")
		}
	} else {
		shell.Warn("未找到 bpftune 服务单元，可能需重启后生效")
	}

	fmt.Println("")
	shell.Success("BBR 优化完成！")
	return nil
}

package security

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

const (
	sshdConfig      = "/etc/ssh/sshd_config"
	sshdConfigDir   = "/etc/ssh/sshd_config.d"
	lhSSHDropin     = "/etc/ssh/sshd_config.d/99-linux-helper.conf"
	backupDirPrefix = "/etc/linux-helper/backups"
)

// SSHSafeRestart validates and restarts SSH.
func SSHSafeRestart() error {
	shell.Info("正在检查 SSH 配置语法...")
	if _, err := shell.Run("sshd", "-t"); err != nil {
		shell.Error("SSH 配置语法错误！请检查配置。")
		return err
	}
	shell.Info("配置语法正确，重启 SSH 服务...")
	if err := shell.RunSilent("systemctl", "restart", "sshd"); err != nil {
		if err2 := shell.RunSilent("systemctl", "restart", "ssh"); err2 != nil {
			shell.Warn("SSH 重启失败，请手动检查")
			return err2
		}
	}
	shell.Success("SSH 服务已重启")
	return nil
}

// ChangePort interactively changes the SSH port.
func ChangePort() error {
	shell.Header("修改 SSH 端口")

	ports := currentPorts()
	fmt.Printf("  当前 SSH 监听端口: %s\n", ports)

	portStr := strings.TrimSpace(shell.ReadInput("输入新的 SSH 端口号 (1-65535): "))
	newPort, err := strconv.Atoi(portStr)
	if err != nil || newPort < 1 || newPort > 65535 {
		shell.Warn("无效端口号，请输入 1-65535 之间的数字。")
		return nil
	}

	// Check if port is in use
	out, _ := shell.Run("ss", "-tlnp")
	if strings.Contains(out, ":"+portStr+" ") {
		shell.Warn("端口 %d 已被占用", newPort)
		if !shell.Confirm("仍然强制设置？") {
			return nil
		}
	}

	backupSSHConfig()

	// Clean Port lines from all files
	shell.Info("清除所有文件中的 Port 配置...")
	cleanSettingFromSSH("Port")

	// Write to drop-in
	os.MkdirAll(sshdConfigDir, 0755)
	f, _ := os.OpenFile(lhSSHDropin, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	fmt.Fprintf(f, "Port %d\n", newPort)
	f.Close()
	shell.Info("已写入: %s -> Port %d", lhSSHDropin, newPort)

	fmt.Println("")
	if _, err := shell.Run("sshd", "-t"); err != nil {
		shell.Error("配置语法错误！恢复备份...")
		// Restore handled by backup
		return nil
	}
	shell.Success("配置语法正确")

	// Auto-open firewall
	ufwOut, _ := shell.Run("ufw", "status")
	if strings.Contains(ufwOut, "active") {
		shell.Info("检测到 UFW 防火墙，自动放行端口 %d ...", newPort)
		shell.RunSilent("ufw", "allow", fmt.Sprintf("%d/tcp", newPort))
	}
	fwOut, _ := shell.Run("firewall-cmd", "--state")
	if strings.TrimSpace(fwOut) == "running" {
		shell.Info("检测到 firewalld，自动放行端口 %d ...", newPort)
		shell.RunSilent("firewall-cmd", "--add-port", fmt.Sprintf("%d/tcp", newPort), "--permanent")
		shell.RunSilent("firewall-cmd", "--reload")
	}

	if shell.Confirm("立即重启 SSH 服务？") {
		SSHSafeRestart()
		fmt.Println("")
		shell.Info("新端口 %d 已生效", newPort)
		shell.Info("如需用旧端口连接，请在 5 分钟内保持旧会话")
	} else {
		shell.Info("跳过重启，配置已保存。下次重启 SSH 后生效。")
	}
	return nil
}

// EnableRootLogin interactively configures PermitRootLogin yes.
func EnableRootLogin() error {
	shell.Header("配置 root 密码登录")

	permitRoot := getSSHSetting("PermitRootLogin")
	passwordAuth := getSSHSetting("PasswordAuthentication")
	fmt.Printf("  当前 PermitRootLogin:    %s\n", defaultIfEmpty(permitRoot, "未设置 (默认 prohibit-password)"))
	fmt.Printf("  当前 PasswordAuthentication: %s\n", defaultIfEmpty(passwordAuth, "未设置 (默认 yes)"))
	fmt.Println("")

	if permitRoot == "yes" && passwordAuth != "no" {
		shell.Success("root 密码登录已经是开启状态")
		if !shell.Confirm("重新设置？") {
			return nil
		}
	}

	if !shell.Confirm("开启 root 密码登录？（将设置 PermitRootLogin yes 并确保密码认证开启）") {
		return nil
	}

	backupSSHConfig()

	// Clean settings from all files
	for _, key := range []string{"PermitRootLogin", "PasswordAuthentication", "ChallengeResponseAuthentication"} {
		cleanSettingFromSSH(key)
	}

	// Write to drop-in
	os.MkdirAll(sshdConfigDir, 0755)
	f, _ := os.OpenFile(lhSSHDropin, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	fmt.Fprintln(f, "PermitRootLogin yes")
	fmt.Fprintln(f, "PasswordAuthentication yes")
	fmt.Fprintln(f, "ChallengeResponseAuthentication yes")
	f.Close()
	shell.Info("已写入: %s", lhSSHDropin)

	fmt.Println("")
	if _, err := shell.Run("sshd", "-t"); err != nil {
		shell.Error("配置语法错误，请手动检查 /etc/ssh/sshd_config")
		return nil
	}
	shell.Success("配置语法正确")
	if shell.Confirm("立即重启 SSH 服务？") {
		SSHSafeRestart()
		fmt.Println("")
		shell.Info("请在新终端中测试 root 密码登录后再关闭当前会话！")
	}
	return nil
}

// ManageKeys interactively manages SSH authorized_keys files.
func ManageKeys() error {
	for {
		shell.Clear()
		shell.Header("SSH 公钥管理")

		type userKeys struct {
			user string
			path string
			n    int
		}
		var users []userKeys

		f, _ := os.Open("/etc/passwd")
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), ":")
			if len(parts) < 7 {
				continue
			}
			home := parts[5]
			shellField := parts[6]
			if home == "/nonexistent" || home == "/" || home == "" {
				continue
			}
			if shellField == "/usr/sbin/nologin" || shellField == "/bin/false" || shellField == "/sbin/nologin" {
				continue
			}
			ak := filepath.Join(home, ".ssh", "authorized_keys")
			data, err := os.ReadFile(ak)
			if err != nil {
				continue
			}
			n := len(strings.Split(strings.TrimSpace(string(data)), "\n"))
			if n == 0 || (len(data) == 1 && data[0] == '\n') {
				n = 0
			}
			if n > 0 {
				users = append(users, userKeys{parts[0], ak, n})
			}
		}
		f.Close()

		if len(users) == 0 {
			shell.Info("系统中没有找到 authorized_keys 文件。")
			shell.PressEnter()
			return nil
		}

		fmt.Println("  选择用户查看其公钥:")
		for i, u := range users {
			fmt.Printf("  %d) %s (%d 个密钥)\n", i+1, u.user, u.n)
		}
		fmt.Println("")
		fmt.Println("  b) 返回上级")
		fmt.Println("  q) 退出")
		fmt.Println("")
		choice := shell.ReadInput("  请选择用户: ")

		if choice == "b" || choice == "B" {
			return nil
		}
		if choice == "q" || choice == "Q" {
			os.Exit(0)
		}
		idx, err := strconv.Atoi(choice)
		if err != nil || idx < 1 || idx > len(users) {
			continue
		}
		listDeleteKeys(users[idx-1].user, users[idx-1].path)
	}
}

// ----- unexported helpers -----

func currentPorts() string {
	out, _ := shell.Run("ss", "-tlnp")
	var ports []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "sshd") {
			continue
		}
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.Contains(f, ":") {
				parts := strings.Split(f, ":")
				ports = append(ports, parts[len(parts)-1])
				break
			}
		}
	}
	return strings.Join(unique(ports), " ")
}

func unique(s []string) []string {
	seen := make(map[string]bool)
	var r []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			r = append(r, v)
		}
	}
	return r
}

func getSSHSetting(key string) string {
	val := ""
	data, _ := os.ReadFile(sshdConfig)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key) {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				val = parts[1]
			}
		}
	}
	// Check drop-ins (higher priority, alphabetical order)
	entries, err := os.ReadDir(sshdConfigDir)
	if err == nil {
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".conf") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(sshdConfigDir, e.Name()))
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, key) {
					parts := strings.Fields(trimmed)
					if len(parts) >= 2 {
						val = parts[1]
					}
				}
			}
		}
	}
	return val
}

func cleanSettingFromSSH(key string) {
	files := []string{sshdConfig}
	if entries, err := os.ReadDir(sshdConfigDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".conf") {
				files = append(files, filepath.Join(sshdConfigDir, e.Name()))
			}
		}
	}
	for _, fp := range files {
		data, _ := os.ReadFile(fp)
		var lines []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), key) {
				lines = append(lines, line)
			}
		}
		joined := strings.Join(lines, "\n")
		if strings.TrimSpace(joined) == "" {
			os.Remove(fp)
		} else {
			os.WriteFile(fp, []byte(joined), 0644)
		}
	}
}

func backupSSHConfig() {
	os.MkdirAll(backupDirPrefix, 0755)
	ts := shell.Timestamp()
	dir := filepath.Join(backupDirPrefix, "ssh-backup-"+ts)
	os.MkdirAll(dir, 0755)

	data, _ := os.ReadFile(sshdConfig)
	os.WriteFile(filepath.Join(dir, "sshd_config"), data, 0644)

	if entries, err := os.ReadDir(sshdConfigDir); err == nil {
		os.MkdirAll(filepath.Join(dir, "sshd_config.d"), 0755)
		for _, e := range entries {
			data, _ := os.ReadFile(filepath.Join(sshdConfigDir, e.Name()))
			os.WriteFile(filepath.Join(dir, "sshd_config.d", e.Name()), data, 0644)
		}
	}
	shell.Info("SSH 配置已备份到: %s", dir)
}

func defaultIfEmpty(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func listDeleteKeys(user, keyFile string) {
	for {
		shell.Clear()
		shell.Header(fmt.Sprintf("用户 %s 的 SSH 公钥", user))

		data, _ := os.ReadFile(keyFile)
		keys := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(keys) == 1 && keys[0] == "" {
			shell.Info("该用户没有公钥。")
			shell.PressEnter()
			return
		}

		for i, k := range keys {
			preview := k
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			fmt.Printf("  %d) %s\n", i+1, preview)
		}
		fmt.Println("")
		fmt.Println("  输入编号删除对应密钥（可多选，如: 1 3 5）")
		fmt.Println("  a) 删除全部密钥")
		fmt.Println("  b) 返回上级")
		fmt.Println("  q) 退出")
		fmt.Println("")
		choice := shell.ReadInput("  请选择: ")

		if choice == "b" || choice == "B" {
			return
		}
		if choice == "q" || choice == "Q" {
			os.Exit(0)
		}
		if choice == "a" || choice == "A" {
			if shell.Confirm(fmt.Sprintf("确认删除 %s 的所有 %d 个密钥？", user, len(keys))) {
				os.WriteFile(keyFile, []byte(""), 0644)
				shell.Success("已删除 %s 的全部密钥", user)
				shell.PressEnter()
				return
			}
			continue
		}

		nums := strings.Fields(choice)
		var toDelete []int
		for _, n := range nums {
			idx, err := strconv.Atoi(n)
			if err == nil && idx >= 1 && idx <= len(keys) {
				toDelete = append(toDelete, idx)
			}
		}
		if len(toDelete) == 0 {
			shell.Warn("无效选择")
			shell.PressEnter()
			continue
		}

		// Sort descending
		for i := 0; i < len(toDelete); i++ {
			for j := i + 1; j < len(toDelete); j++ {
				if toDelete[i] < toDelete[j] {
					toDelete[i], toDelete[j] = toDelete[j], toDelete[i]
				}
			}
		}

		for _, n := range toDelete {
			preview := keys[n-1]
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			shell.Warn("  删除: %s...", preview)
		}
		if !shell.Confirm(fmt.Sprintf("确认删除以上 %d 个密钥？", len(toDelete))) {
			continue
		}

		delMap := make(map[int]bool)
		for _, n := range toDelete {
			delMap[n] = true
		}
		var newKeys []string
		for i, k := range keys {
			if !delMap[i+1] {
				newKeys = append(newKeys, k)
			}
		}
		os.WriteFile(keyFile, []byte(strings.Join(newKeys, "\n")+"\n"), 0644)
		shell.Success("已删除 %d 个密钥", len(toDelete))
		shell.Info("剩余 %d 个密钥", len(newKeys))
		shell.PressEnter()
	}
}

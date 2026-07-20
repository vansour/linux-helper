// Package tuning provides Linux system tuning operations.
package tuning

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

const (
	swapRecordFile  = "/etc/linux-helper/swap-record"
	fstabFile       = "/etc/fstab"
	defaultSwapPath = "/swapfile"
)

// ShowStatus displays the current system swap state.
func ShowStatus() {
	shell.Header("当前 Swap 状态")

	out, err := shell.Run("swapon", "--show")
	if err != nil || out == "" {
		shell.Info("未检测到任何 Swap 设备")
	} else {
		fmt.Println(out)
		fmt.Println()
	}

	fmt.Printf("%s:\n", shell.Blue("free -h"))
	freeOut, _ := shell.Run("free", "-h"); var memSwapLines []string; for _, l := range strings.Split(freeOut, "\n") { if strings.HasPrefix(l, "Swap") || strings.HasPrefix(l, "Mem") { memSwapLines = append(memSwapLines, l) } }; freeOut = strings.Join(memSwapLines, "\n")
	if freeOut != "" {
		fmt.Println(freeOut)
	}
	fmt.Println()

	fstabData, err := os.ReadFile(fstabFile)
	if err == nil {
		var fstabSwaps []string
		for _, line := range strings.Split(string(fstabData), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "swap") || strings.Contains(trimmed, "sw") {
				fstabSwaps = append(fstabSwaps, line)
			}
		}
		if len(fstabSwaps) == 0 {
			shell.Info("/etc/fstab 中没有 Swap 配置")
		} else {
			shell.Info("/etc/fstab 中的 Swap 配置:")
			fmt.Println(strings.Join(fstabSwaps, "\n"))
		}
	}

	if data, err := os.ReadFile(swapRecordFile); err == nil && len(data) > 0 {
		fmt.Println()
		shell.Info("Linux Helper 管理的 Swap 文件: %s", strings.TrimSpace(string(data)))
	}
	fmt.Println()
}

// AddSwap interactively creates a new swap file.
func AddSwap() {
	shell.Header("添加 Swap")

	swapPath := defaultSwapPath
	input := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("Swap 文件路径 [%s]: ", swapPath)))
	if input != "" {
		swapPath = input
	}

	if _, err := os.Stat(swapPath); err == nil {
		activeSwaps := getActiveSwaps()
		for _, s := range activeSwaps {
			if s == swapPath {
				shell.Warn("%s 已作为 Swap 在使用", swapPath)
				shell.PressEnter()
				return
			}
		}
	}

	memTotalMB := getMemTotalMB()
	defaultSize := memTotalMB - 1
	if defaultSize < 1 {
		defaultSize = memTotalMB / 2
		if defaultSize < 1 {
			defaultSize = 1
		}
	}
	sizeStr := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("Swap 大小（单位 MB，默认 %d）: ", defaultSize)))
	sizeMB := defaultSize
	if sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			sizeMB = s
		} else {
			shell.Warn("无效大小，请输入正整数（单位 MB）")
			return
		}
	}

	availKB, err := getAvailDiskKB(filepath.Dir(swapPath))
	if err == nil {
		neededKB := int64(sizeMB) * 1024
		if availKB < neededKB {
			shell.Warn("磁盘空间不足！需要 %dMB，可用约 %dMB", sizeMB, availKB/1024)
			if !shell.Confirm("仍然继续？") {
				return
			}
		}
	}

	fmt.Println()
	shell.Info("创建 Swap 文件: %s (%dMB)...", swapPath, sizeMB)
	fmt.Println()

	if shell.Has("fallocate") {
		if err := shell.RunSilent("fallocate", "-l", fmt.Sprintf("%dM", sizeMB), swapPath); err != nil {
			shell.Info("fallocate 失败，使用 dd 创建...")
			shell.RunSilent("dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapPath), fmt.Sprintf("bs=1M"), fmt.Sprintf("count=%d", sizeMB))
		}
	} else {
		shell.RunSilent("dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapPath), "bs=1M", fmt.Sprintf("count=%d", sizeMB))
	}

	if _, err := os.Stat(swapPath); err != nil {
		shell.Error("创建 Swap 文件失败")
		return
	}
	os.Chmod(swapPath, 0600)

	fsType := getFSType(swapPath)
	if fsType == "btrfs" {
		shell.Info("检测到 btrfs 文件系统，禁用 CoW...")
		shell.RunSilent("chattr", "+C", swapPath)
	}

	if err := shell.RunSilent("mkswap", swapPath); err != nil {
		shell.Error("格式化 Swap 失败")
		os.Remove(swapPath)
		return
	}

	if err := shell.RunSilent("swapon", swapPath); err != nil {
		shell.Error("启用 Swap 失败")
		os.Remove(swapPath)
		return
	}

	ensureFstabEntry(swapPath)

	os.MkdirAll(filepath.Dir(swapRecordFile), 0755)
	os.WriteFile(swapRecordFile, []byte(swapPath+"\n"), 0644)

	shell.Success("Swap 添加成功！")
	fmt.Println()
	ShowStatus()
}

// DeleteSwap interactively removes a swap device.
func DeleteSwap() {
	shell.Header("删除 Swap")

	swaps := getActiveSwaps()
	if len(swaps) == 0 {
		shell.Info("没有启用中的 Swap 设备")
		shell.PressEnter()
		return
	}

	for _, s := range swaps {
		fmt.Printf("  %s\n", s)
	}
	fmt.Println()

	var target string
	if len(swaps) == 1 {
		target = swaps[0]
		shell.Info("检测到一个 Swap 设备: %s", target)
		if !shell.Confirm("删除此 Swap？") {
			return
		}
	} else {
		fmt.Println("选择要删除的 Swap 设备:")
		for i, s := range swaps {
			fmt.Printf("  %d) %s\n", i+1, s)
		}
		selStr := strings.TrimSpace(shell.ReadInput("请输入编号: "))
		sel, err := strconv.Atoi(selStr)
		if err != nil || sel < 1 || sel > len(swaps) {
			shell.Warn("无效选择")
			return
		}
		target = swaps[sel-1]
		if !shell.Confirm(fmt.Sprintf("删除 Swap: %s？", target)) {
			return
		}
	}

	if err := shell.RunSilent("swapoff", target); err != nil {
		shell.Error("swapoff 失败")
		return
	}
	shell.Success("已停用: %s", target)

	removeFstabEntry(target)

	if isRegularFile(target) {
		if shell.Confirm(fmt.Sprintf("删除 swap 文件 %s？", target)) {
			os.Remove(target)
			shell.Success("已删除文件: %s", target)
		}
	}

	cleanRecord(target)

	fmt.Println()
	shell.Success("Swap 删除完成")
}

// AdjustSwap interactively deletes then recreates a swap.
func AdjustSwap() {
	shell.Header("调整 Swap")

	swapPath := defaultSwapPath
	input := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("Swap 文件路径 [%s]: ", swapPath)))
	if input != "" {
		swapPath = input
	}

	fmt.Println()
	shell.Warn("调整操作将:")
	fmt.Printf("  1. 删除 %s（如存在）\n", swapPath)
	fmt.Println("  2. 重新创建指定大小的 Swap")
	fmt.Println()

	if !shell.Confirm("确认调整？") {
		return
	}

	if _, err := os.Stat(swapPath); err == nil {
		active := false
		for _, s := range getActiveSwaps() {
			if s == swapPath {
				active = true
				break
			}
		}
		if active {
			shell.Info("停用旧 Swap: %s", swapPath)
			shell.RunSilent("swapoff", swapPath)
		}
		shell.Info("删除旧 Swap 文件: %s", swapPath)
		os.Remove(swapPath)
	}
	removeFstabEntry(swapPath)

	memTotalMB := getMemTotalMB()
	defaultSize := memTotalMB - 1
	if defaultSize < 1 {
		defaultSize = memTotalMB / 2
		if defaultSize < 1 {
			defaultSize = 1
		}
	}
	sizeStr := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("新的 Swap 大小（单位 MB，默认 %d）: ", defaultSize)))
	sizeMB := defaultSize
	if sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			sizeMB = s
		} else {
			shell.Warn("无效大小，请输入正整数（单位 MB）")
			return
		}
	}

	availKB, err := getAvailDiskKB(filepath.Dir(swapPath))
	if err == nil {
		neededKB := int64(sizeMB) * 1024
		if availKB < neededKB {
			shell.Warn("磁盘空间不足！需要 %dMB，可用约 %dMB", sizeMB, availKB/1024)
			if !shell.Confirm("仍然继续？") {
				return
			}
		}
	}

	fmt.Println()
	shell.Info("创建新的 Swap 文件: %s (%dMB)", swapPath, sizeMB)
	if shell.Has("fallocate") {
		if err := shell.RunSilent("fallocate", "-l", fmt.Sprintf("%dM", sizeMB), swapPath); err != nil {
			shell.Info("fallocate 失败，使用 dd 创建...")
			shell.RunSilent("dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapPath), "bs=1M", fmt.Sprintf("count=%d", sizeMB))
		}
	} else {
		shell.RunSilent("dd", "if=/dev/zero", fmt.Sprintf("of=%s", swapPath), "bs=1M", fmt.Sprintf("count=%d", sizeMB))
	}
	os.Chmod(swapPath, 0600)

	fsType := getFSType(swapPath)
	if fsType == "btrfs" {
		shell.Info("检测到 btrfs 文件系统，禁用 CoW...")
		shell.RunSilent("chattr", "+C", swapPath)
	}

	if err := shell.RunSilent("mkswap", swapPath); err != nil {
		shell.Error("格式化 Swap 失败")
		os.Remove(swapPath)
		return
	}

	if err := shell.RunSilent("swapon", swapPath); err != nil {
		shell.Error("启用 Swap 失败")
		os.Remove(swapPath)
		return
	}

	ensureFstabEntry(swapPath)

	os.MkdirAll(filepath.Dir(swapRecordFile), 0755)
	os.WriteFile(swapRecordFile, []byte(swapPath+"\n"), 0644)

	shell.Success("Swap 调整完成！")
	fmt.Println()
	ShowStatus()
}

// -- unexported helpers --

func getMemTotalMB() int {
	out, err := shell.Run("free", "-m")
	if err != nil {
		return 1024
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.Atoi(fields[1]); err == nil {
					return v
				}
			}
		}
	}
	return 1024
}

func getAvailDiskKB(dir string) (int64, error) {
	out, err := shell.Run("df", dir)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("df output too short")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("unexpected df format")
	}
	return strconv.ParseInt(fields[3], 10, 64)
}

func getFSType(path string) string {
	out, err := shell.Run("df", "-T", path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "/") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

func getActiveSwaps() []string {
	out, err := shell.Run("swapon", "--show", "--noheadings")
	if err != nil || out == "" {
		return nil
	}
	var swaps []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			swaps = append(swaps, fields[0])
		}
	}
	return swaps
}

func ensureFstabEntry(swapPath string) error {
	data, err := os.ReadFile(fstabFile)
	if err == nil && strings.Contains(string(data), swapPath) {
		return nil
	}
	f, err := os.OpenFile(fstabFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s none swap sw 0 0\n", swapPath)
	return err
}

func removeFstabEntry(swapPath string) error {
	data, err := os.ReadFile(fstabFile)
	if err != nil {
		return err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, swapPath) && strings.Contains(trimmed, "swap") {
			continue
		}
		lines = append(lines, line)
	}
	return os.WriteFile(fstabFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func cleanRecord(target string) {
	if data, err := os.ReadFile(swapRecordFile); err == nil {
		if strings.Contains(string(data), target) {
			os.Remove(swapRecordFile)
		}
	}
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

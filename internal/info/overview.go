package info

import (
	"fmt"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// ShowOverview displays a comprehensive system overview.
func ShowOverview() {
	shell.Header("系统概览")

	osInfo, _ := shell.Run("cat", "/etc/os-release")
	for _, line := range strings.Split(osInfo, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			fmt.Printf("  操作系统:   %s\n", strings.Trim(line[13:], `"`))
			break
		}
	}

	hostname, _ := shell.Run("hostname")
	fmt.Printf("  主机名:     %s\n", hostname)

	kernel, _ := shell.Run("uname", "-r")
	fmt.Printf("  内核版本:   %s\n", kernel)

	arch, _ := shell.Run("uname", "-m")
	fmt.Printf("  架构:       %s\n", arch)

	uptime, _ := shell.Run("uptime", "-p")
	fmt.Printf("  运行时间:   %s\n", strings.TrimPrefix(uptime, "up "))

	load, _ := shell.Run("cat", "/proc/loadavg")
	fields := strings.Fields(load)
	if len(fields) >= 3 {
		fmt.Printf("  负载:       %s %s %s\n", fields[0], fields[1], fields[2])
	}

	cpu, _ := shell.Run("nproc")
	fmt.Printf("  CPU:        %s 核心\n", strings.TrimSpace(cpu))

	mem, _ := shell.Run("free", "-h")
	for _, line := range strings.Split(mem, "\n") {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				fmt.Printf("  内存:       %s 总量 / %s 已用\n", fields[1], fields[2])
			}
			break
		}
	}

	disk, _ := shell.Run("df", "-h", "/")
	for _, line := range strings.Split(disk, "\n") {
		if strings.HasPrefix(line, "/") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				fmt.Printf("  磁盘(/):    %s 总量 / %s 已用 / %s 可用\n", fields[1], fields[2], fields[3])
			}
			break
		}
	}

	fmt.Println()
}

// ShowCPU displays CPU information.
func ShowCPU() {
	shell.Header("CPU 信息")
	data, _ := shell.Run("lscpu")
	for _, line := range strings.Split(data, "\n") {
		if strings.Contains(line, ":") {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Println()
}

// ShowMemory displays memory information.
func ShowMemory() {
	shell.Header("内存信息")
	data, _ := shell.Run("free", "-h")
	fmt.Println(data)
	fmt.Println("")
}

// ShowDisk displays disk information.
func ShowDisk() {
	shell.Header("磁盘信息")
	data, _ := shell.Run("df", "-h")
	fmt.Println(data)
	fmt.Println("")
}

// ShowNetwork displays network information.
func ShowNetwork() {
	shell.Header("网络信息")
	data, _ := shell.Run("ip", "addr")
	fmt.Println(data)
	fmt.Println("")
}

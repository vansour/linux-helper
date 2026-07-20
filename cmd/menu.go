package cmd

import (
	"fmt"

	"github.com/vansour/linux-helper/internal/info"
	"github.com/vansour/linux-helper/internal/maintenance"
	"github.com/vansour/linux-helper/internal/network"
	"github.com/vansour/linux-helper/internal/security"
	"github.com/vansour/linux-helper/internal/shell"
	"github.com/vansour/linux-helper/internal/system"
	"github.com/vansour/linux-helper/internal/tools"
	"github.com/vansour/linux-helper/internal/tui"
	"github.com/vansour/linux-helper/internal/tuning"
)

// mainMenu is the top-level interactive menu with all modules wired.
func buildMainMenu() *tui.Menu {
	m := tui.NewMenu("Linux Helper — 系统管理助手",
		"1", "网络优化",
		"2", "系统配置",
		"3", "系统安全",
		"4", "系统调优",
		"5", "系统信息",
		"6", "系统维护",
		"7", "工具箱",
	)

	// 1) Network optimization
	net := tui.NewMenu("网络优化",
		"1", "开启 BBR + fq + bpftune",
		"2", "TCP 参数调优",
		"3", "查看网络状态",
	)
	ipv6 := tui.NewMenu("IPv6 管理",
		"1", "查看 IPv6 状态",
		"2", "禁用 IPv6",
		"3", "启用 IPv6",
	)

	net.Handle("1", func() error { return network.EnableBBR() })
	net.Handle("2", tcpPlaceholder)
	net.Handle("3", netStatPlaceholder)
	net.Add("4", ipv6)

	ipv6.Handle("1", func() error { return network.ShowIPv6Status() })
	ipv6.Handle("2", func() error { return network.DisableIPv6() })
	ipv6.Handle("3", func() error { return network.EnableIPv6() })

	m.Add("1", net)

	// 2) System config
	sys := tui.NewMenu("系统配置",
		"1", "设置时区",
		"2", "修改主机名",
		"3", "配置语言环境",
	)
	chronyM := tui.NewMenu("Chrony 时间同步管理",
		"1", "查看 Chrony 状态",
		"2", "安装/配置 Chrony",
		"3", "立即同步时间",
	)

	sys.Handle("1", func() error { system.SetTimezone(); return nil })
	sys.Handle("2", hostnamePlaceholder)
	sys.Handle("3", localePlaceholder)
	sys.Add("4", chronyM)

	chronyM.Handle("1", func() error { system.ChronyShowConfig(); return nil })
	chronyM.Handle("2", func() error { return system.SetupChrony("") })
	chronyM.Handle("3", func() error { system.SyncTime(); return nil })

	m.Add("2", sys)

	// 3) Security
	sec := tui.NewMenu("系统安全",
		"1", "SSH 安全配置",
		"2", "配置防火墙",
		"3", "Fail2Ban 管理",
	)
	ssh := tui.NewMenu("SSH 安全配置",
		"1", "修改 SSH 端口",
		"2", "启用 root 密码登录",
		"3", "管理 SSH 公钥",
	)

	sec.Add("1", ssh)
	sec.Handle("2", firewallPlaceholder)
	sec.Handle("3", fail2banPlaceholder)

	ssh.Handle("1", func() error { return security.ChangePort() })
	ssh.Handle("2", func() error { return security.EnableRootLogin() })
	ssh.Handle("3", func() error { return security.ManageKeys() })

	m.Add("3", sec)

	// 4) Tuning
	tun := tui.NewMenu("系统调优",
		"1", "Swap 管理",
		"2", "内核参数优化",
		"3", "文件描述符限制",
	)
	swap := tui.NewMenu("Swap 管理",
		"1", "查看 Swap 状态",
		"2", "添加 Swap",
		"3", "删除 Swap",
		"4", "调整 Swap",
	)

	tun.Add("1", swap)
	tun.Handle("2", kernelPlaceholder)
	tun.Handle("3", fdPlaceholder)

	swap.Handle("1", func() error { return tuning.ShowStatus() })
	swap.Handle("2", func() error { return tuning.AddSwap() })
	swap.Handle("3", func() error { return tuning.DeleteSwap() })
	swap.Handle("4", func() error { return tuning.AdjustSwap() })

	m.Add("4", tun)

	// 5) Info
	inf := tui.NewMenu("系统信息",
		"1", "系统概览",
		"2", "CPU 信息",
		"3", "内存信息",
		"4", "磁盘信息",
		"5", "网络信息",
	)
	inf.Handle("1", func() error { info.ShowOverview(); return nil })
	inf.Handle("2", func() error { info.ShowCPU(); return nil })
	inf.Handle("3", func() error { info.ShowMemory(); return nil })
	inf.Handle("4", func() error { info.ShowDisk(); return nil })
	inf.Handle("5", func() error { info.ShowNetwork(); return nil })
	m.Add("5", inf)

	// 6) Maintenance
	maint := tui.NewMenu("系统维护",
		"1", "系统更新与清理",
		"2", "日志清理",
		"3", "Docker 清理",
		"4", "临时文件清理",
	)
	maint.Handle("1", func() error { maintenance.UpdateSystem(); return nil })
	maint.Handle("2", func() error { maintenance.CleanLogs(); return nil })
	maint.Handle("3", func() error { maintenance.CleanDocker(); return nil })
	maint.Handle("4", func() error { maintenance.CleanTemp(); return nil })
	m.Add("6", maint)

	// 7) Tools
	tl := tui.NewMenu("工具箱",
		"1", "安装常用软件",
		"2", "安装 Docker",
		"3", "安装 Docker Compose",
		"4", "安装系统监控工具",
	)
	tl.Handle("1", func() error { tools.InstallCommon(); return nil })
	tl.Handle("2", func() error { tools.InstallDocker(); return nil })
	tl.Handle("3", func() error { tools.InstallCompose(); return nil })
	tl.Handle("4", func() error { tools.InstallMonitor(); return nil })
	m.Add("7", tl)

	return m
}

// mainMenu is the package-level menu variable used by root.go
var mainMenu = buildMainMenu()

// Placeholder functions for unimplemented features
func tcpPlaceholder() error      { placeholderMsg("TCP 参数调优"); return nil }
func netStatPlaceholder() error  { placeholderMsg("查看网络状态"); return nil }
func hostnamePlaceholder() error { placeholderMsg("修改主机名"); return nil }
func localePlaceholder() error   { placeholderMsg("配置语言环境"); return nil }
func firewallPlaceholder() error { placeholderMsg("配置防火墙"); return nil }
func fail2banPlaceholder() error { placeholderMsg("Fail2Ban 管理"); return nil }
func kernelPlaceholder() error   { placeholderMsg("内核参数优化"); return nil }
func fdPlaceholder() error       { placeholderMsg("文件描述符限制"); return nil }

func placeholderMsg(name string) {
	fmt.Println("")
	fmt.Println(shell.ColorNC + "━━━ [待开发] ━━━")
	fmt.Printf("  功能：%s\n", shell.Yellow(name))
	fmt.Println("  此功能尚未实现，敬请期待。")
	fmt.Println("━━━━━━━━━━━━━━━")
	fmt.Println("")
}

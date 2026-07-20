package system

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// ShowTimezoneStatus displays current timezone, date and NTP/chrony status.
func ShowTimezoneStatus() {
	fmt.Println()
	shell.Header("当前时间配置")
	fmt.Println()

	tz, _ := shell.Run("timedatectl", "show", "--property=Timezone", "--value")
	if tz == "" {
		tz = "未知"
	}
	fmt.Printf("  系统时区:   %s\n", shell.Green(tz))

	localTime, _ := shell.Run("date", "+%Y-%m-%d %H:%M:%S %Z")
	if localTime != "" {
		fmt.Printf("  本地时间:   %s\n", shell.Blue(localTime))
	}

	utcTime, _ := shell.Run("date", "-u", "+%Y-%m-%d %H:%M:%S UTC")
	if utcTime != "" {
		fmt.Printf("  UTC 时间:   %s\n", shell.Blue(utcTime))
	}

	fmt.Println()
	fmt.Println("  NTP 服务状态:")

	chronyd := &shell.SystemdService{Name: "chronyd", Alt: "chrony"}
	ntpd := &shell.SystemdService{Name: "ntpd"}
	timesyncd := &shell.SystemdService{Name: "systemd-timesyncd"}

	if chronyd.IsActive() {
		fmt.Printf("    %s  运行中 ✓\n", shell.Green("chronyd"))
	} else if ntpd.IsActive() {
		fmt.Printf("    %s  运行中（建议迁移到 chrony）\n", shell.Yellow("ntpd"))
	} else if timesyncd.IsActive() {
		fmt.Printf("    %s  运行中（建议迁移到 chrony）\n", shell.Yellow("systemd-timesyncd"))
	} else {
		fmt.Printf("    %s\n", shell.Red("未运行"))
	}

	if shell.Has("chronyc") {
		tracking, _ := shell.Run("chronyc", "tracking")
		for _, line := range strings.Split(tracking, "\n") {
			if strings.HasPrefix(line, "Leap status") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					fmt.Printf("    同步状态:   %s\n", shell.Green(parts[1]))
				}
				break
			}
		}
	}
	fmt.Println()
}

// SetTimezone interactively selects and sets the system timezone,
// then configures chrony time synchronization.
func SetTimezone() {
	fmt.Println()
	shell.Header("设置时区")

	if !shell.Has("timedatectl") {
		shell.Error("未找到 timedatectl，请安装 systemd。")
		return
	}

	ShowTimezoneStatus()

	if !shell.Confirm("将强制配置 Chrony 时间同步（会停止其他 NTP 服务），确认？") {
		return
	}

	method := chooseInputMethod()
	if method == "" {
		return
	}

	tz := resolveTimezone(method)
	if tz == "" {
		return
	}

	if !validateTimezone(tz) {
		return
	}

	fmt.Println()
	shell.Info("设置时区: %s", tz)
	if _, err := shell.Run("timedatectl", "set-timezone", tz); err != nil {
		shell.Error("设置时区失败")
		return
	}
	shell.Success("时区已设置为 %s", tz)

	fmt.Println()
	shell.Info("正在配置 chrony 时间同步服务...")
	if err := SetupChrony(tz); err != nil {
		shell.Warn("Chrony 配置过程有异常，请查看以上错误信息。")
	}

	fmt.Println()
	ShowTimezoneStatus()
}

// chooseInputMethod returns "list", "search", or "manual". Returns "" on b/q.
func chooseInputMethod() string {
	for {
		fmt.Println("  选择时区设置方式:")
		fmt.Println("   1) 从常用时区列表中选择")
		fmt.Println("   2) 搜索时区（输入关键词）")
		fmt.Println("   3) 手动输入时区名称")
		fmt.Println()
		fmt.Println("   b) 返回")
		fmt.Println("   q) 退出")
		fmt.Println()

		choice := strings.TrimSpace(shell.ReadInput("  请选择 [1-3]: "))
		switch choice {
		case "1":
			return "list"
		case "2":
			return "search"
		case "3":
			return "manual"
		case "b", "B":
			return ""
		case "q", "Q":
			os.Exit(0)
			return ""
		default:
			shell.Warn("无效选项")
		}
	}
}

// resolveTimezone returns a timezone string based on the chosen input method.
func resolveTimezone(method string) string {
	switch method {
	case "list":
		return selectFromContinent()
	case "search":
		return searchTimezone()
	case "manual":
		return strings.TrimSpace(shell.ReadInput("  输入时区名称 (如 Asia/Shanghai): "))
	}
	return ""
}

// selectFromContinent presents a continent menu and lets the user pick a timezone.
func selectFromContinent() string {
	fmt.Println()
	shell.Header("选择地区")
	fmt.Println("  1) 亚洲")
	fmt.Println("  2) 欧洲")
	fmt.Println("  3) 北美洲")
	fmt.Println("  4) 南美洲")
	fmt.Println("  5) 大洋洲")
	fmt.Println("  6) 非洲")
	fmt.Println()

	regionChoice := strings.TrimSpace(shell.ReadInput("  请选择地区 [1-6]: "))

	type regionEntry struct {
		name  string
		zones []string
	}
	regions := map[string]regionEntry{
		"1": {"亚洲", []string{
			"Asia/Shanghai", "Asia/Tokyo", "Asia/Singapore", "Asia/Dubai",
			"Asia/Hong_Kong", "Asia/Taipei", "Asia/Seoul", "Asia/Bangkok",
			"Asia/Jakarta", "Asia/Kolkata", "Asia/Kuala_Lumpur",
		}},
		"2": {"欧洲", []string{
			"Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Moscow",
			"Europe/Amsterdam", "Europe/Madrid", "Europe/Rome", "Europe/Stockholm",
			"Europe/Zurich", "Europe/Istanbul", "Europe/Vienna",
		}},
		"3": {"北美洲", []string{
			"America/New_York", "America/Chicago", "America/Denver",
			"America/Los_Angeles", "America/Toronto", "America/Vancouver",
			"America/Mexico_City", "America/Phoenix", "America/Halifax",
		}},
		"4": {"南美洲", []string{
			"America/Sao_Paulo", "America/Santiago", "America/Buenos_Aires",
			"America/Bogota", "America/Lima", "America/Caracas", "America/Montevideo",
		}},
		"5": {"大洋洲", []string{
			"Pacific/Auckland", "Australia/Sydney", "Pacific/Fiji",
			"Pacific/Honolulu", "Pacific/Guam", "Australia/Melbourne",
		}},
		"6": {"非洲", []string{
			"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos",
			"Africa/Nairobi", "Africa/Casablanca", "Africa/Accra",
		}},
	}

	entry, ok := regions[regionChoice]
	if !ok {
		shell.Warn("无效选择")
		return ""
	}

	fmt.Println()
	shell.Header("选择时区（" + entry.name + "）")
	for i, z := range entry.zones {
		fmt.Printf("  %2d) %s\n", i+1, z)
	}
	fmt.Println()

	idxStr := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("  请选择 [1-%d]: ", len(entry.zones))))
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 1 || idx > len(entry.zones) {
		shell.Warn("无效选择")
		return ""
	}
	return entry.zones[idx-1]
}

// searchTimezone searches timezones by keyword and returns the user's selection.
func searchTimezone() string {
	keyword := strings.TrimSpace(shell.ReadInput("  输入关键词 (如 Shanghai, Tokyo, New_York): "))
	if keyword == "" {
		shell.Warn("关键词不能为空")
		return ""
	}

	output, err := shell.Run("timedatectl", "list-timezones")
	if err != nil {
		shell.Error("获取时区列表失败")
		return ""
	}

	allZones := strings.Split(output, "\n")
	var matches []string
	for _, z := range allZones {
		z = strings.TrimSpace(z)
		if z == "" {
			continue
		}
		if strings.Contains(strings.ToLower(z), strings.ToLower(keyword)) {
			matches = append(matches, z)
			if len(matches) >= 30 {
				break
			}
		}
	}

	if len(matches) == 0 {
		shell.Warn("未找到匹配的时区")
		return ""
	}

	if len(matches) == 1 {
		fmt.Printf("  匹配: %s\n", matches[0])
		return matches[0]
	}

	fmt.Println()
	shell.Header(fmt.Sprintf("搜索结果（前 %d 条）", len(matches)))
	for i, z := range matches {
		fmt.Printf("  %2d) %s\n", i+1, z)
	}
	fmt.Println()

	selStr := strings.TrimSpace(shell.ReadInput(fmt.Sprintf("  请选择 [1-%d]: ", len(matches))))
	sel, err := strconv.Atoi(selStr)
	if err != nil || sel < 1 || sel > len(matches) {
		shell.Warn("无效选择")
		return ""
	}
	return matches[sel-1]
}

// validateTimezone checks that a timezone name is valid by querying timedatectl.
func validateTimezone(tz string) bool {
	if tz == "" {
		shell.Warn("时区名称不能为空")
		return false
	}

	output, err := shell.Run("timedatectl", "list-timezones")
	if err != nil {
		shell.Error("无法获取时区列表进行验证")
		return false
	}

	valid := false
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == tz {
			valid = true
			break
		}
	}

	if !valid {
		shell.Warn("无效的时区名称: %s", tz)
		fmt.Println("  提示: 运行 timedatectl list-timezones 查看所有可用时区")
		return false
	}

	return true
}

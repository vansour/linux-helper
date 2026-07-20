package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vansour/linux-helper/internal/shell"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装到系统",
	RunE: func(cmd *cobra.Command, args []string) error {
		src, _ := os.Executable()
		installDir := "/usr/local/linux-helper"
		binLink := "/usr/local/bin/linux-helper"

		fmt.Println("")
		shell.Header("安装 Linux Helper")

		shell.Info("安装到: %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}

		// Copy self to install dir
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("读取自身失败: %v", err)
		}
		dstPath := installDir + "/linux-helper"
		if err := os.WriteFile(dstPath, data, 0755); err != nil {
			return fmt.Errorf("复制文件失败: %v", err)
		}

		// Create symlink
		if _, err := os.Lstat(binLink); err == nil {
			shell.Warn("已存在: %s，将覆盖...", binLink)
			os.Remove(binLink)
		}
		if err := os.Symlink(dstPath, binLink); err != nil {
			return fmt.Errorf("创建符号链接失败: %v", err)
		}

		shell.Success("安装完成！")
		fmt.Println("")
		fmt.Println("  运行方式:")
		fmt.Println("    sudo linux-helper")
		fmt.Println("    或")
		fmt.Printf("    sudo %s\n", dstPath)
		fmt.Println("")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载",
	RunE: func(cmd *cobra.Command, args []string) error {
		installDir := "/usr/local/linux-helper"
		binLink := "/usr/local/bin/linux-helper"

		fmt.Println("")
		shell.Warn("将删除:")
		fmt.Printf("  - %s\n", installDir)
		fmt.Printf("  - %s\n", binLink)
		fmt.Println("")

		if !shell.Confirm("确认卸载？") {
			shell.Info("取消卸载。")
			os.Exit(0)
		}

		os.RemoveAll(installDir)
		os.Remove(binLink)

		shell.Success("卸载完成。")
		return nil
	},
}

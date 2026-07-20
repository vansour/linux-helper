package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command (interactive menu by default)
var rootCmd = &cobra.Command{
	Use:   "linux-helper",
	Short: "Linux Helper — 系统管理助手",
	Long: `Linux Helper 是一个 Linux 服务器辅助管理工具。
提供网络优化、系统配置、安全加固、系统调优等功能。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default: show interactive menu
		mainMenu.Run()
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Subcommands for direct invocation (skip menu)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

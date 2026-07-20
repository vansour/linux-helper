package main

import (
	"os"

	"github.com/vansour/linux-helper/cmd"
)

func main() {
	if os.Geteuid() != 0 {
		println("此工具需要 root 权限，请使用 sudo 运行。")
		os.Exit(1)
	}
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

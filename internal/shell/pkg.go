package shell

import (
	"fmt"
	"os/exec"
)

// Manager represents a Linux package manager.
type Manager struct {
	Name        string
	Binary      string
	UpdateArgs  []string
	InstallArgs []string
}

var managers = []Manager{
	{"apt", "apt-get", []string{"apt-get", "update", "-qq"}, []string{"DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y"}},
	{"dnf", "dnf", []string{"dnf", "check-update"}, []string{"dnf", "install", "-y"}},
	{"yum", "yum", []string{"yum", "check-update"}, []string{"yum", "install", "-y"}},
	{"zypper", "zypper", []string{"zypper", "refresh"}, []string{"zypper", "install", "-y"}},
}

// DetectPackageManager returns the first available package manager.
func DetectPackageManager() *Manager {
	for _, m := range managers {
		if _, err := exec.LookPath(m.Binary); err == nil {
			return &m
		}
	}
	return nil
}

// InstallPackage installs a package using the detected package manager.
// For apt, it runs update first and uses DEBIAN_FRONTEND=noninteractive.
func InstallPackage(pkg string) error {
	pm := DetectPackageManager()
	if pm == nil {
		return fmt.Errorf("不支持的包管理器，请手动安装: apt install %s", pkg)
	}

	if pm.Name == "apt" || pm.Name == "zypper" {
		if _, err := Run(pm.Binary, pm.UpdateArgs[1:]...); err != nil {
			// update failure is non-fatal
		}
	}

	if pm.Name == "apt" {
		args := append(pm.InstallArgs[2:], pkg)
		cmd := exec.Command(pm.InstallArgs[1], args...)
		cmd.Env = append(cmd.Environ(), pm.InstallArgs[0])
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("安装 %s 失败: %v — %s", pkg, err, string(out))
		}
		return nil
	}

	if _, err := Run(pm.InstallArgs[0], append(pm.InstallArgs[1:], pkg)...); err != nil {
		return fmt.Errorf("安装 %s 失败: %v", pkg, err)
	}
	return nil
}

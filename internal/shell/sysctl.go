package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SysctlGet reads a kernel parameter value.
func SysctlGet(key string) (string, error) {
	return Run("sysctl", "-n", key)
}

// SysctlSet writes a kernel parameter immediately.
func SysctlSet(key, value string) error {
	_, err := Run("sysctl", "-w", fmt.Sprintf("%s=%s", key, value))
	return err
}

// SysctlPersist writes a parameter to a sysctl.d drop-in file,
// cleaning any conflicting lines from existing files first.
func SysctlPersist(file, key, value string) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Read existing content
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		// Remove lines setting the same key
		if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" =") {
			continue
		}
		newLines = append(newLines, line)
	}
	newLines = append(newLines, fmt.Sprintf("%s = %s", key, value))

	return os.WriteFile(file, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

// SysctlGetBool reads a boolean kernel parameter (0/1).
func SysctlGetBool(key string) (bool, error) {
	val, err := SysctlGet(key)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(val) == "1", nil
}

// SysctlRestore restores parameters from a list of key=value pairs.
func SysctlRestore(pairs []string) error {
	for _, p := range pairs {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if err := SysctlSet(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])); err != nil {
			// non-fatal
		}
	}
	return nil
}

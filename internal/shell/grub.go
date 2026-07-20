package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const grubConfig = "/etc/default/grub"

// GrubGetParam reads a GRUB_CMDLINE_LINUX value.
func GrubGetParam() (string, error) {
	return readGrubLine(`GRUB_CMDLINE_LINUX="`)
}

// GrubSetParam updates GRUB_CMDLINE_LINUX with new parameters,
// preserving existing ones and modifying the given key=value.
func GrubSetParam(key, value string) error {
	input, err := os.ReadFile(grubConfig)
	if err != nil {
		return fmt.Errorf("读取 GRUB 配置失败: %v", err)
	}

	var output strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(input)))
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), `GRUB_CMDLINE_LINUX="`) {
			// Extract current params
			content := extractQuotedValue(line)
			params := parseParams(content)

			// Set or override the specific key
			params[key] = value

			// Rebuild the line
			newLine := fmt.Sprintf(`GRUB_CMDLINE_LINUX="%s"`, buildParams(params))
			if strings.HasPrefix(line, "\t") {
				newLine = "\t" + newLine
			}
			output.WriteString(newLine + "\n")
			found = true
		} else {
			output.WriteString(line + "\n")
		}
	}

	if !found {
		output.WriteString(fmt.Sprintf(`GRUB_CMDLINE_LINUX="%s=%s"`+"\n", key, value))
	}

	return os.WriteFile(grubConfig, []byte(output.String()), 0644)
}

// GrubRemoveParam removes a parameter from GRUB_CMDLINE_LINUX.
func GrubRemoveParam(key string) error {
	input, err := os.ReadFile(grubConfig)
	if err != nil {
		return fmt.Errorf("读取 GRUB 配置失败: %v", err)
	}

	var output strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(input)))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), `GRUB_CMDLINE_LINUX="`) {
			content := extractQuotedValue(line)
			params := parseParams(content)
			delete(params, key)

			newLine := fmt.Sprintf(`GRUB_CMDLINE_LINUX="%s"`, buildParams(params))
			if strings.HasPrefix(line, "\t") {
				newLine = "\t" + newLine
			}
			output.WriteString(newLine + "\n")
		} else {
			output.WriteString(line + "\n")
		}
	}

	return os.WriteFile(grubConfig, []byte(output.String()), 0644)
}

// GrubUpdate runs update-grub or grub-mkconfig.
func GrubUpdate() error {
	if Has("update-grub") {
		_, err := Run("update-grub")
		return err
	}
	_, err := Run("grub-mkconfig", "-o", "/boot/grub/grub.cfg")
	return err
}

func extractQuotedValue(line string) string {
	start := strings.Index(line, `"`)
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(line, `"`)
	if end <= start {
		return ""
	}
	return line[start+1 : end]
}

func parseParams(input string) map[string]string {
	params := make(map[string]string)
	fields := strings.Fields(input)
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		} else {
			params[f] = ""
		}
	}
	return params
}

func buildParams(params map[string]string) string {
	var parts []string
	for k, v := range params {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, " ")
}

func readGrubLine(prefix string) (string, error) {
	f, err := os.Open(grubConfig)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return extractQuotedValue(line), nil
		}
	}
	return "", nil
}

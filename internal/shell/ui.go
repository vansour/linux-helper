package shell

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
	colorBlue   = "\033[0;34m"
	colorCyan   = "\033[0;36m"
	ColorNC     = "\033[0m"
)

// Clear clears the terminal screen.
func Clear() {
	fmt.Print("\033[H\033[2J")
}

// Info prints a blue [INFO] message.
func Info(format string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s  %s\n", colorBlue, ColorNC, fmt.Sprintf(format, args...))
}

// Success prints a green [OK] message.
func Success(format string, args ...interface{}) {
	fmt.Printf("%s[OK]%s   %s\n", colorGreen, ColorNC, fmt.Sprintf(format, args...))
}

// Warn prints a yellow [WARN] message.
func Warn(format string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, ColorNC, fmt.Sprintf(format, args...))
}

// Error prints a red [ERROR] message.
func Error(format string, args ...interface{}) {
	fmt.Printf("%s[ERROR]%s %s\n", colorRed, ColorNC, fmt.Sprintf(format, args...))
}

func colorize(code, text string) string {
	return code + text + ColorNC
}

// Cyan wraps text in cyan color.
func Cyan(text string) string {
	return colorize(colorCyan, text)
}

// Green wraps text in green color.
func Green(text string) string {
	return colorize(colorGreen, text)
}

// Red wraps text in red color.
func Red(text string) string {
	return colorize(colorRed, text)
}

// Blue wraps text in blue color.
func Blue(text string) string {
	return colorize(colorBlue, text)
}

// Yellow wraps text in yellow color.
func Yellow(text string) string {
	return colorize(colorYellow, text)
}

// Header prints a centered section header.
func Header(title string) {
	const width = 50
	pad := (width - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Println("")
	fmt.Println(Cyan(strings.Repeat("=", width)))
	fmt.Printf("%s%s%s\n",
		Cyan(strings.Repeat(" ", pad)),
		Cyan(title),
		Cyan(strings.Repeat(" ", width-pad-len(title))),
	)
	fmt.Println(Cyan(strings.Repeat("=", width)))
	fmt.Println("")
}

// Confirm prompts the user for y/n confirmation.
// Returns true if the user answered yes.
func Confirm(prompt string) bool {
	if prompt == "" {
		prompt = "确认继续？"
	}
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// Timestamp returns a current timestamp string for use in filenames.
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}
func PressEnter() {
	fmt.Print("按回车键继续...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// ReadInput reads a line of user input with an optional prompt.
func ReadInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

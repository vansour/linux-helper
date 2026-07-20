// Package tui provides a simple interactive text-menu system for terminal UIs.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vansour/linux-helper/internal/shell"
)

// Menu represents an interactive text menu with options.
type Menu struct {
	Title   string
	Options []Option
	parent  *Menu
}

// Option represents a single menu option.
type Option struct {
	Key     string // "1", "2", "b", "q"
	Label   string // display text
	Handler func() error
	Submenu *Menu
	Back    bool // "b" — return to parent
	Quit    bool // "q" — exit program
}

// NewMenu creates a new menu. Pass a list of [key, label] pairs;
// key "" and "0" become Quit options.
func NewMenu(title string, items ...string) *Menu {
	m := &Menu{Title: title}
	for i := 0; i+1 < len(items); i += 2 {
		key := items[i]
		label := items[i+1]
		opt := Option{Key: key, Label: label}
		if key == "" || key == "0" {
			opt.Quit = true
		} else if key == "b" || key == "B" {
			opt.Back = true
		}
		m.Options = append(m.Options, opt)
	}
	return m
}

// Add adds a submenu to an option by key.
func (m *Menu) Add(key string, sub *Menu) *Menu {
	for i := range m.Options {
		if m.Options[i].Key == key {
			m.Options[i].Submenu = sub
			sub.parent = m
			break
		}
	}
	return m
}

// Handle sets the handler for an option by key.
func (m *Menu) Handle(key string, fn func() error) *Menu {
	for i := range m.Options {
		if m.Options[i].Key == key {
			m.Options[i].Handler = fn
			break
		}
	}
	return m
}

// Run starts the interactive menu loop.
func (m *Menu) Run() error {
	for {
		shell.Clear()
		shell.Header(m.Title)

		for _, opt := range m.Options {
			if opt.Key == "b" || opt.Key == "B" || opt.Key == "q" || opt.Key == "Q" || opt.Key == "0" {
				continue
			}
			fmt.Printf("  %s)  %s\n", opt.Key, opt.Label)
		}
		fmt.Println("")
		if m.parent != nil {
			fmt.Println("  b)  返回上级")
		}
		fmt.Println("  q)  退出")
		fmt.Println("")
		fmt.Print("  请选择: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		if choice == "q" || choice == "Q" {
			fmt.Println("")
			shell.Success("感谢使用，再见！")
			os.Exit(0)
		}
		if choice == "b" || choice == "B" {
			if m.parent != nil {
				return nil
			}
			continue
		}
		if choice == "0" {
			fmt.Println("")
			shell.Success("感谢使用，再见！")
			os.Exit(0)
		}

		opt := m.findOption(choice)
		if opt == nil {
			shell.Warn("无效选项，请重新选择。")
			shell.PressEnter()
			continue
		}
		if opt.Submenu != nil {
			if err := opt.Submenu.Run(); err != nil {
				return err
			}
		} else if opt.Handler != nil {
			if err := opt.Handler(); err != nil {
				shell.Warn("操作出错: %v", err)
			}
			shell.PressEnter()
		} else if opt.Back {
			return nil
		} else if opt.Quit {
			fmt.Println("")
			shell.Success("感谢使用，再见！")
			os.Exit(0)
		}
	}
}

func (m *Menu) findOption(key string) *Option {
	for i := range m.Options {
		if m.Options[i].Key == key {
			return &m.Options[i]
		}
	}
	return nil
}

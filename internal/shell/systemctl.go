package shell

import "fmt"

// SystemdService represents a systemd service unit.
type SystemdService struct {
	Name string // primary service name (e.g. "chronyd")
	Alt  string // fallback name (e.g. "chrony")
}

// IsActive checks if the service is running.
func (s *SystemdService) IsActive() bool {
	if err := RunSilent("systemctl", "is-active", "--quiet", s.Name); err == nil {
		return true
	}
	if s.Alt != "" {
		return RunSilent("systemctl", "is-active", "--quiet", s.Alt) == nil
	}
	return false
}

// IsEnabled checks if the service is enabled.
func (s *SystemdService) IsEnabled() bool {
	if err := RunSilent("systemctl", "is-enabled", "--quiet", s.Name); err == nil {
		return true
	}
	if s.Alt != "" {
		return RunSilent("systemctl", "is-enabled", "--quiet", s.Alt) == nil
	}
	return false
}

// Enable enables and starts the service.
func (s *SystemdService) Enable() error {
	if err := RunSilent("systemctl", "enable", "--now", s.Name); err == nil {
		return nil
	}
	if s.Alt != "" {
		if err := RunSilent("systemctl", "enable", "--now", s.Alt); err == nil {
			return nil
		}
	}
	return fmt.Errorf("无法启用服务 %s/%s", s.Name, s.Alt)
}

// Disable stops and disables the service.
func (s *SystemdService) Disable() error {
	if err := RunSilent("systemctl", "disable", "--now", s.Name); err == nil {
		return nil
	}
	if s.Alt != "" {
		if err := RunSilent("systemctl", "disable", "--now", s.Alt); err == nil {
			return nil
		}
	}
	return fmt.Errorf("无法禁用服务 %s/%s", s.Name, s.Alt)
}

// Restart restarts the service.
func (s *SystemdService) Restart() error {
	if err := RunSilent("systemctl", "restart", s.Name); err == nil {
		return nil
	}
	if s.Alt != "" {
		if err := RunSilent("systemctl", "restart", s.Alt); err == nil {
			return nil
		}
	}
	return fmt.Errorf("无法重启服务 %s/%s", s.Name, s.Alt)
}

// Stop stops the service.
func (s *SystemdService) Stop() error {
	if err := RunSilent("systemctl", "stop", s.Name); err == nil {
		return nil
	}
	if s.Alt != "" {
		return RunSilent("systemctl", "stop", s.Alt)
	}
	return fmt.Errorf("无法停止服务 %s", s.Name)
}

// Start starts the service.
func (s *SystemdService) Start() error {
	if err := RunSilent("systemctl", "start", s.Name); err == nil {
		return nil
	}
	if s.Alt != "" {
		return RunSilent("systemctl", "start", s.Alt)
	}
	return fmt.Errorf("无法启动服务 %s/%s", s.Name, s.Alt)
}

// HasUnitFile checks if the service unit file exists on disk.
func (s *SystemdService) HasUnitFile() bool {
	if err := RunSilent("systemctl", "list-unit-files", s.Name+".service"); err == nil {
		return true
	}
	if s.Alt != "" {
		return RunSilent("systemctl", "list-unit-files", s.Alt+".service") == nil
	}
	return false
}

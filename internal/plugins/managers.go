// Package plugins provides adapters for external tools and package managers.
package plugins

import (
	"os/exec"
	"sort"
)

// PackageManager defines the interface for system package managers.
type PackageManager interface {
	// Name returns the human-readable name (e.g., "pacman", "apt", "brew").
	Name() string
	// NeedsSudo indicates whether this manager requires sudo for installs.
	NeedsSudo() bool
	// IsAvailable checks if this manager is installed on the system.
	IsAvailable() bool
	// InstallCommand returns the command to install the given packages.
	InstallCommand(packages []string) []string
}

// Pacman adapts the Arch Linux package manager.
type Pacman struct{}

func (Pacman) Name() string           { return "pacman" }
func (Pacman) NeedsSudo() bool        { return true }
func (Pacman) IsAvailable() bool      { return exec.Command("pacman", "--version").Run() == nil }

func (Pacman) InstallCommand(packages []string) []string {
	cmd := []string{"pacman", "-S", "--noconfirm"}
	return append(cmd, packages...)
}

// Apt adapts the Debian/Ubuntu package manager.
type Apt struct{}

func (Apt) Name() string      { return "apt" }
func (Apt) NeedsSudo() bool   { return true }
func (Apt) IsAvailable() bool { return exec.Command("apt-get", "--version").Run() == nil }

func (Apt) InstallCommand(packages []string) []string {
	cmd := []string{"apt-get", "install", "-y"}
	return append(cmd, packages...)
}

// Brew adapts the Homebrew package manager (macOS/Linux).
type Brew struct{}

func (Brew) Name() string      { return "brew" }
func (Brew) NeedsSudo() bool   { return false }
func (Brew) IsAvailable() bool { return exec.Command("brew", "--version").Run() == nil }

func (Brew) InstallCommand(packages []string) []string {
	cmd := []string{"brew", "install"}
	return append(cmd, packages...)
}

// allManagers is the ordered list of supported package managers.
var allManagers = []PackageManager{
	Pacman{},
	Apt{},
	Brew{},
}

// GetPackageManager detects and returns the first available package manager.
// Returns nil if no supported manager is found.
func GetPackageManager() PackageManager {
	for _, m := range allManagers {
		if m.IsAvailable() {
			return m
		}
	}
	return nil
}

// ManagerNames returns sorted names of all supported managers (for display).
func ManagerNames() []string {
	names := make([]string, len(allManagers))
	for i, m := range allManagers {
		names[i] = m.Name()
	}
	sort.Strings(names)
	return names
}

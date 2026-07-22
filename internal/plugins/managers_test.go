package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagerNames_Sorted(t *testing.T) {
	names := ManagerNames()
	assert.Equal(t, []string{"apt", "brew", "pacman"}, names,
		"ManagerNames should return alphabetically sorted names")
}

func TestPacman(t *testing.T) {
	m := Pacman{}
	assert.Equal(t, "pacman", m.Name())
	assert.True(t, m.NeedsSudo())

	cmd := m.InstallCommand([]string{"neovim", "git"})
	assert.Equal(t, []string{"pacman", "-S", "--noconfirm", "neovim", "git"}, cmd)
}

func TestPacman_InstallCommand_Empty(t *testing.T) {
	m := Pacman{}
	cmd := m.InstallCommand(nil)
	assert.Equal(t, []string{"pacman", "-S", "--noconfirm"}, cmd)
}

func TestApt(t *testing.T) {
	m := Apt{}
	assert.Equal(t, "apt", m.Name())
	assert.True(t, m.NeedsSudo())

	cmd := m.InstallCommand([]string{"neovim", "git"})
	assert.Equal(t, []string{"apt-get", "install", "-y", "neovim", "git"}, cmd)
}

func TestApt_InstallCommand_Empty(t *testing.T) {
	m := Apt{}
	cmd := m.InstallCommand(nil)
	assert.Equal(t, []string{"apt-get", "install", "-y"}, cmd)
}

func TestBrew(t *testing.T) {
	m := Brew{}
	assert.Equal(t, "brew", m.Name())
	assert.False(t, m.NeedsSudo())

	cmd := m.InstallCommand([]string{"neovim", "git"})
	assert.Equal(t, []string{"brew", "install", "neovim", "git"}, cmd)
}

func TestBrew_InstallCommand_Empty(t *testing.T) {
	m := Brew{}
	cmd := m.InstallCommand(nil)
	assert.Equal(t, []string{"brew", "install"}, cmd)
}

func TestAllManagers_Interface(t *testing.T) {
	// Verify all managers implement PackageManager (compile-time check)
	var _ PackageManager = Pacman{}
	var _ PackageManager = Apt{}
	var _ PackageManager = Brew{}
	_ = t // unused parameter is intentional - the compile-time checks are the test
}

func TestAllManagers_DistinctNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, m := range allManagers {
		name := m.Name()
		assert.False(t, seen[name], "duplicate manager name: %s", name)
		seen[name] = true
	}
}

func TestGetPackageManager_InCI(t *testing.T) {
	// On CI or dev machines without pacman/apt/brew, this returns nil.
	// The test just verifies it doesn't panic and returns the correct type.
	mgr := GetPackageManager()
	if mgr != nil {
		assert.True(t,
			mgr.IsAvailable(),
			"returned manager %s must be available", mgr.Name())
	}
}

func TestPacman_IsAvailable(t *testing.T) {
	m := Pacman{}
	// Should not panic even if pacman is not installed
	available := m.IsAvailable()
	t.Logf("Pacman available: %v", available)
}

func TestApt_IsAvailable(t *testing.T) {
	m := Apt{}
	available := m.IsAvailable()
	t.Logf("Apt available: %v", available)
}

func TestBrew_IsAvailable(t *testing.T) {
	m := Brew{}
	available := m.IsAvailable()
	t.Logf("Brew available: %v", available)
}

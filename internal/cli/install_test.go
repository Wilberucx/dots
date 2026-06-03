package cli

import (
	"testing"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── resolveInstallDecision tests ────────────────────────────────────────────

// mockManager implements plugins.PackageManager for testing.
type mockManager struct {
	name string
}

func (m mockManager) Name() string      { return m.name }
func (m mockManager) NeedsSudo() bool   { return false }
func (m mockManager) IsAvailable() bool { return true }
func (m mockManager) InstallCommand(packages []string) []string {
	return append([]string{m.name + "-install"}, packages...)
}

func TestResolveInstallDecision_NonPackage(t *testing.T) {
	dep := yaml.Dependency{
		Name: "starship",
		Type: "binary",
		URL:  "https://example.com/starship.tar.gz",
		Dest: "/nonexistent/path/starship",
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, dep, decision.Dep, "non-package dep should be returned as-is")
	assert.Empty(t, decision.PackageName, "non-package dep should leave PackageName at dep.Name")
	assert.False(t, decision.UsesFallback, "non-package dep should not use fallback")
	assert.Empty(t, decision.SkipReason, "non-package dep with non-existent dest should not skip")
}

func TestResolveInstallDecision_PackageNoManagers(t *testing.T) {
	dep := yaml.Dependency{
		Name: "some_unique_nonexistent_pkg_12345",
		Type: "package",
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, dep, decision.Dep)
	assert.Equal(t, "some_unique_nonexistent_pkg_12345", decision.PackageName, "should use dep.Name when no managers map")
	assert.False(t, decision.UsesFallback)
	// The package name is unlikely to be a real binary, so skip reason should be empty
	// (binary not found via exec.LookPath = install ok)
	assert.Empty(t, decision.SkipReason)
}

func TestResolveInstallDecision_PackageWithManager(t *testing.T) {
	dep := yaml.Dependency{
		Name: "some_unique_nonexistent_pkg_12345",
		Type: "package",
		Managers: map[string]string{
			"pacman": "starship",
			"apt":    "starship",
		},
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, dep, decision.Dep)
	assert.Equal(t, "starship", decision.PackageName, "should use manager-specific name")
	assert.False(t, decision.UsesFallback)
	assert.Empty(t, decision.SkipReason)
}

func TestResolveInstallDecision_PackageWithManagerDifferentName(t *testing.T) {
	dep := yaml.Dependency{
		Name: "some_unique_nonexistent_pkg_12345",
		Type: "package",
		Managers: map[string]string{
			"brew": "starship",
		},
	}
	manager := mockManager{name: "brew"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, dep, decision.Dep)
	assert.Equal(t, "starship", decision.PackageName, "should use the brew-specific name")
	assert.False(t, decision.UsesFallback)
	assert.Empty(t, decision.SkipReason)
}

func TestResolveInstallDecision_PackageFallback(t *testing.T) {
	fallback := yaml.Dependency{
		Name: "starship",
		Type: "binary",
		URL:  "https://example.com/starship.tar.gz",
		Dest: "/nonexistent/fallback/starship",
	}
	dep := yaml.Dependency{
		Name:     "starship",
		Type:     "package",
		Managers: map[string]string{"apt": "starship"},
		Fallback: &fallback,
	}
	manager := mockManager{name: "pacman"} // not in managers map

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, fallback, decision.Dep, "should return the fallback dep")
	assert.Empty(t, decision.PackageName, "fallback is binary, not package")
	assert.True(t, decision.UsesFallback)
	assert.Empty(t, decision.SkipReason)
}

func TestResolveInstallDecision_PackageSkip(t *testing.T) {
	dep := yaml.Dependency{
		Name:     "starship",
		Type:     "package",
		Managers: map[string]string{"apt": "starship"},
		// No Fallback
	}
	manager := mockManager{name: "pacman"} // not in managers map

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, dep, decision.Dep)
	assert.Empty(t, decision.PackageName)
	assert.False(t, decision.UsesFallback)
	assert.Equal(t, "not available for pacman", decision.SkipReason)
}

func TestResolveInstallDecision_PackageAlreadyInstalled(t *testing.T) {
	// "ls" is guaranteed to be in PATH on any Unix system
	dep := yaml.Dependency{
		Name: "ls",
		Type: "package",
		Bin:  "ls",
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, "already installed", decision.SkipReason)
}

func TestResolveInstallDecision_GitDestExists(t *testing.T) {
	tmpDir := t.TempDir()
	dep := yaml.Dependency{
		Name: "myrepo",
		Type: "git",
		URL:  "https://example.com/repo.git",
		Dest: tmpDir,
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Contains(t, decision.SkipReason, "already exists")
}

func TestResolveInstallDecision_GitMissingFields(t *testing.T) {
	dep := yaml.Dependency{
		Name: "myrepo",
		Type: "git",
		// Missing URL and Dest
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, "missing source or target", decision.SkipReason)
}

func TestResolveInstallDecision_BinaryDestExists(t *testing.T) {
	tmpDir := t.TempDir()
	dep := yaml.Dependency{
		Name: "mybinary",
		Type: "binary",
		URL:  "https://example.com/binary.tar.gz",
		Dest: tmpDir,
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Contains(t, decision.SkipReason, "already exists")
}

func TestResolveInstallDecision_BinaryMissingFields(t *testing.T) {
	dep := yaml.Dependency{
		Name: "mybinary",
		Type: "binary",
		// Missing URL and Dest
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Equal(t, "missing source or target", decision.SkipReason)
}

func TestResolveInstallDecision_UnknownType(t *testing.T) {
	dep := yaml.Dependency{
		Name: "weird",
		Type: "unknown",
	}
	manager := mockManager{name: "pacman"}

	decision := resolveInstallDecision(dep, manager)

	assert.Contains(t, decision.SkipReason, "unknown type")
}

func TestResolveInstallDecision_PackageFallbackWithExistingDest(t *testing.T) {
	// Package with unsupported manager and a binary fallback whose Dest exists.
	// Preview should show the fallback chain, then resolveInstallDecision on
	// the fallback should produce a skip.
	tmpDir := t.TempDir()
	fallback := yaml.Dependency{
		Name: "starship",
		Type: "binary",
		URL:  "https://example.com/starship.tar.gz",
		Dest: tmpDir,
	}
	dep := yaml.Dependency{
		Name:     "starship",
		Type:     "package",
		Managers: map[string]string{"apt": "starship"},
		Fallback: &fallback,
	}
	manager := mockManager{name: "pacman"}

	// First level: package → fallback
	decision := resolveInstallDecision(dep, manager)
	assert.True(t, decision.UsesFallback, "should use fallback")
	assert.Equal(t, fallback, decision.Dep)
	assert.Empty(t, decision.SkipReason)

	// Second level: fallback binary → skip because dest exists
	fallbackDecision := resolveInstallDecision(fallback, manager)
	assert.Contains(t, fallbackDecision.SkipReason, "already exists")
}

// ─── buildVariantSwapMap purity test ─────────────────────────────────────────

func TestBuildVariantSwapMap_NilOnNoVariant(t *testing.T) {
	// When variant is empty, should return nil
	m := buildVariantSwapMap(nil, []string{"Zsh"}, "", false)
	assert.Nil(t, m, "no variant should return nil")
}

func TestBuildVariantSwapMap_NilOnForce(t *testing.T) {
	// When force is true, should return nil even with variant
	m := buildVariantSwapMap(nil, []string{"Zsh"}, "work", true)
	assert.Nil(t, m, "force should return nil")
}

func TestBuildVariantSwapMap_NilOnNoModules(t *testing.T) {
	// When no modules selected, should return nil
	m := buildVariantSwapMap(nil, nil, "work", false)
	assert.Nil(t, m, "no modules should return nil")
}

func TestBuildVariantSwapMap_NoSwapNeeded(t *testing.T) {
	// With empty config (no repo root), GetActiveVariant returns empty string,
	// so no swap should be detected.
	cfg := &config.DotsConfig{}
	m := buildVariantSwapMap(cfg, []string{"Zsh"}, "work", false)
	assert.Nil(t, m, "no active variant means no swap needed")
}

// ─── sortedSwapModules test ──────────────────────────────────────────────────

func TestSortedSwapModules(t *testing.T) {
	m := map[string]bool{
		"Zsh":       true,
		"Nvim":      true,
		"Alacritty": true,
	}
	result := sortedSwapModules(m)
	require.Len(t, result, 3)
	assert.Equal(t, "Alacritty", result[0])
	assert.Equal(t, "Nvim", result[1])
	assert.Equal(t, "Zsh", result[2])
}

func TestSortedSwapModules_Empty(t *testing.T) {
	assert.Empty(t, sortedSwapModules(nil))
	assert.Empty(t, sortedSwapModules(map[string]bool{}))
}

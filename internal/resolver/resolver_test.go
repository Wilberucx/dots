package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cantoarch/dots/internal/config"
	"github.com/cantoarch/dots/internal/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary dotfiles repo with the given module structure.
func setupTestRepo(t *testing.T) *config.DotsConfig {
	t.Helper()
	repoDir := t.TempDir()

	// Create marker
	dotsDir := filepath.Join(repoDir, ".dots")
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, "config.yaml"), []byte("version: 1"), 0644)
	require.NoError(t, err)

	// Create a home dir and set HOME so IsSafePath works
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create DotsConfig
	cfg := &config.DotsConfig{
		RepoRoot:  repoDir,
		CurrentOS: "linux",
		HomeDir:   homeDir,
		CLIDir:    filepath.Join(repoDir, "cli"),
	}

	return cfg
}

// createModule creates a module with a path.yaml and optional source files.
func createModule(t *testing.T, cfg *config.DotsConfig, name, yamlContent string, sourceFiles map[string]string) {
	t.Helper()
	modDir := filepath.Join(cfg.RepoRoot, name)
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	for srcPath, content := range sourceFiles {
		fullPath := filepath.Join(modDir, srcPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
}

// createSymlink creates a symlink from target to linkPath.
func createSymlink(t *testing.T, target, linkPath string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(linkPath), 0755)
	require.NoError(t, err)
	err = os.Symlink(target, linkPath)
	require.NoError(t, err)
}

func TestExpandPath(t *testing.T) {
	// Test with absolute path
	result := ExpandPath("/some/absolute/path")
	assert.Equal(t, "/some/absolute/path", result)

	// Test with no tilde (relative)
	result = ExpandPath("relative/path")
	assert.Equal(t, "relative/path", result)
}

func TestResolveModules_NoModules(t *testing.T) {
	cfg := setupTestRepo(t)
	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestResolveModules_SimpleModule(t *testing.T) {
	cfg := setupTestRepo(t)

	// Create a simple module
	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	require.Len(t, results["Zsh"], 1)
	assert.Equal(t, StatePending, results["Zsh"][0].State)
	assert.Equal(t, "will create", results["Zsh"][0].Detail)
}

func TestResolveModules_LinkedModule(t *testing.T) {
	cfg := setupTestRepo(t)

	// Create a module with a source file
	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	// Create the symlink
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", ".zshrc"), filepath.Join(cfg.HomeDir, ".zshrc"))

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	require.Len(t, results["Zsh"], 1)
	assert.Equal(t, StateLinked, results["Zsh"][0].State)
}

func TestResolveModules_ConflictModule(t *testing.T) {
	cfg := setupTestRepo(t)

	// Create a module with a source file
	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	// Create a symlink pointing elsewhere
	createSymlink(t, "/some/other/target", filepath.Join(cfg.HomeDir, ".zshrc"))

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	require.Len(t, results["Zsh"], 1)
	assert.Equal(t, StateConflict, results["Zsh"][0].State)
	assert.Contains(t, results["Zsh"][0].Detail, "points to")
}

func TestResolveModules_WithBackup(t *testing.T) {
	cfg := setupTestRepo(t)

	// Create a module with a source file
	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	// Create a real file at destination (not a symlink)
	zshrcDest := filepath.Join(cfg.HomeDir, ".zshrc")
	err := os.MkdirAll(filepath.Dir(zshrcDest), 0755)
	require.NoError(t, err)
	err = os.WriteFile(zshrcDest, []byte("existing content"), 0644)
	require.NoError(t, err)

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	require.Len(t, results["Zsh"], 1)
	assert.Equal(t, StatePending, results["Zsh"][0].State)
	assert.Equal(t, "backup needed", results["Zsh"][0].Detail)
}

func TestResolveModules_FilterByModule(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	createModule(t, cfg, "Nvim", `
files:
  - source: init.lua
    destination: `+filepath.Join(cfg.HomeDir, ".config", "nvim", "init.lua")+`
`, map[string]string{"init.lua": "vim.cmd('colorscheme slate')"})

	// Filter to only Zsh
	results, err := ResolveModules(cfg, []string{"Zsh"}, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	assert.NotContains(t, results, "Nvim")
}

func TestGetModuleVariantInfo_NoVariants(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	info, err := GetModuleVariantInfo(cfg, "Zsh")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.False(t, info.HasVariants)
}

func TestGetModuleVariantInfo_WithVariants(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Zsh", `
files:
  - source: wilber/.zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
  - source: canto/.zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{
		"wilber/.zshrc": "config 1",
		"canto/.zshrc":  "config 2",
	})

	info, err := GetModuleVariantInfo(cfg, "Zsh")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.True(t, info.HasVariants)
	assert.Contains(t, info.Variants, "wilber/.zshrc")
	assert.Contains(t, info.Variants, "canto/.zshrc")
}

func TestGetActiveVariant_NoVariants(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Zsh", `
files:
  - source: .zshrc
    destination: `+filepath.Join(cfg.HomeDir, ".zshrc")+`
`, map[string]string{".zshrc": "export FOO=bar"})

	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Empty(t, active)
}

func TestResolveModules_GlobSource(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Scripts", `
files:
  - source: scripts/*
    destination: `+filepath.Join(cfg.HomeDir, "scripts")+`
`, map[string]string{
		"scripts/foo.sh": "#!/bin/bash\necho foo",
		"scripts/bar.sh": "#!/bin/bash\necho bar",
	})

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Scripts")
	require.Len(t, results["Scripts"], 2)
}

func TestResolveModules_PerOSMapping(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Alacritty", `
files:
  - source: alacritty.yml
    per-os:
      linux: `+filepath.Join(cfg.HomeDir, ".config", "alacritty", "alacritty.yml")+`
      mac: /Users/test/Library/Application Support/alacritty/alacritty.yml
`, map[string]string{"alacritty.yml": "colors: *default"})

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Alacritty")
	require.Len(t, results["Alacritty"], 1)
	// Should use the linux destination since our test OS is linux
	assert.Contains(t, results["Alacritty"][0].Destination, ".config/alacritty")
}

func TestResolveModules_WithVariantFilter(t *testing.T) {
	cfg := setupTestRepo(t)

	createModule(t, cfg, "Hypr", `
files:
  - source: wilber/hyprland.conf
    destination: `+filepath.Join(cfg.HomeDir, ".config", "hypr", "hyprland.conf")+`
  - source: canto/hyprland.conf
    destination: `+filepath.Join(cfg.HomeDir, ".config", "hypr", "hyprland.conf")+`
`, map[string]string{
		"wilber/hyprland.conf": "config 1",
		"canto/hyprland.conf":  "config 2",
	})

	// Test with explicit variant
	results, err := ResolveModules(cfg, []string{"Hypr"}, nil, "wilber/hyprland.conf")
	require.NoError(t, err)
	require.Contains(t, results, "Hypr")
	require.Len(t, results["Hypr"], 1)
	assert.Contains(t, results["Hypr"][0].Source, "wilber")
}

func TestExpandPathSafety(t *testing.T) {
	t.Run("home prefix", func(t *testing.T) {
		homeDir := filepath.Join("/home", "testuser")
		path := filepath.Join(homeDir, ".config", "nvim")
		result := shortPath(path, homeDir)
		assert.Equal(t, "~/.config/nvim", result)
	})

	t.Run("no prefix", func(t *testing.T) {
		path := "/opt/bin/something"
		result := shortPath(path, "/home/testuser")
		assert.Equal(t, "/opt/bin/something", result)
	})
}

// Test that DotFileMapping and VariantInfo types are accessible
func TestTypesAccessible(t *testing.T) {
	assert.NotNil(t, yaml.DetectVariants)
	assert.NotNil(t, yaml.FilterByVariant)
}

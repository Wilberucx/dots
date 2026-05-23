package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDotfilesRepo_NewFormat(t *testing.T) {
	dir := t.TempDir()

	// Create .dots/config.yaml
	dotsDir := filepath.Join(dir, MarkerDir)
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, MarkerConfig), []byte("version: 1"), 0644)
	require.NoError(t, err)

	assert.True(t, IsDotfilesRepo(dir))
}

func TestIsDotfilesRepo_LegacyFormat(t *testing.T) {
	dir := t.TempDir()

	// Create dots.toml
	err := os.WriteFile(filepath.Join(dir, LegacyMarker), []byte("[dots]\nversion = \"1\""), 0644)
	require.NoError(t, err)

	assert.True(t, IsDotfilesRepo(dir))
}

func TestIsDotfilesRepo_NotARepo(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsDotfilesRepo(dir))
}

func TestIsDotfilesRepo_EmptyDir(t *testing.T) {
	assert.False(t, IsDotfilesRepo("/nonexistent"))
}

func TestLoad_WithEnvVar(t *testing.T) {
	dir := t.TempDir()

	// Create .dots/config.yaml
	dotsDir := filepath.Join(dir, MarkerDir)
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, MarkerConfig), []byte("version: 1"), 0644)
	require.NoError(t, err)

	// Set env var
	t.Setenv("DOTS_REPO", dir)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, dir, cfg.RepoRoot)
	assert.NotEmpty(t, cfg.CurrentOS)
	assert.NotEmpty(t, cfg.HomeDir)
}

func TestLoad_NoRepo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOTS_REPO", dir)

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a dotfiles repository")
}

func TestCreate(t *testing.T) {
	dir := "/tmp/test-dots-repo"
	cfg := create(dir)

	assert.Equal(t, dir, cfg.RepoRoot)
	assert.NotEmpty(t, cfg.CurrentOS)
	assert.NotEmpty(t, cfg.HomeDir)
	assert.Equal(t, filepath.Join(dir, "cli"), cfg.CLIDir)
}

func TestGetModuleDirs_Empty(t *testing.T) {
	dir := t.TempDir()

	// Create .dots/config.yaml
	dotsDir := filepath.Join(dir, MarkerDir)
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, MarkerConfig), []byte("version: 1"), 0644)
	require.NoError(t, err)

	cfg := create(dir)
	mods, err := cfg.GetModuleDirs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, mods)
}

func TestGetModuleDirs_WithModules(t *testing.T) {
	dir := t.TempDir()

	// Create .dots/config.yaml
	dotsDir := filepath.Join(dir, MarkerDir)
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, MarkerConfig), []byte("version: 1"), 0644)
	require.NoError(t, err)

	// Create a module with path.yaml
	modDir := filepath.Join(dir, "Zsh")
	err = os.MkdirAll(modDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files:\n  - source: .zshrc\n    destination: ~/.zshrc"), 0644)
	require.NoError(t, err)

	cfg := create(dir)
	mods, err := cfg.GetModuleDirs(nil, nil)
	require.NoError(t, err)
	require.Len(t, mods, 1)
	assert.Equal(t, "Zsh", mods[0].Name)
}

func TestGetModuleDirs_FilterByName(t *testing.T) {
	dir := t.TempDir()

	// Create .dots/config.yaml
	dotsDir := filepath.Join(dir, MarkerDir)
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dotsDir, MarkerConfig), []byte("version: 1"), 0644)
	require.NoError(t, err)

	// Create modules
	for _, name := range []string{"Zsh", "Nvim", "Alacritty"} {
		modDir := filepath.Join(dir, name)
		err := os.MkdirAll(modDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files: []"), 0644)
		require.NoError(t, err)
	}

	cfg := create(dir)
	mods, err := cfg.GetModuleDirs([]string{"Zsh", "Nvim"}, nil)
	require.NoError(t, err)
	require.Len(t, mods, 2)
	assert.Equal(t, "Zsh", mods[0].Name)
	assert.Equal(t, "Nvim", mods[1].Name)
}

func TestParseModuleMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "path.yaml")

	err := os.WriteFile(path, []byte("type: minimal\nfiles: []"), 0644)
	require.NoError(t, err)

	meta, err := ParseModuleMeta(path)
	require.NoError(t, err)
	assert.Equal(t, "minimal", meta["type"])
}

func TestParseModuleMeta_NoType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "path.yaml")

	err := os.WriteFile(path, []byte("files: []"), 0644)
	require.NoError(t, err)

	meta, err := ParseModuleMeta(path)
	require.NoError(t, err)
	assert.NotContains(t, meta, "type")
}

func TestParseModuleMeta_NotExists(t *testing.T) {
	meta, err := ParseModuleMeta("/nonexistent/path.yaml")
	require.NoError(t, err)
	assert.Empty(t, meta)
}

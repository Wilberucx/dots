package yaml

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "path.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestParsePathYAML_Simple(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
`)

	mappings, err := ParsePathYAML(path, "linux")
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	assert.Equal(t, "init.lua", mappings[0].Source)
	assert.Equal(t, "~/.config/nvim/init.lua", mappings[0].Destination)
}

func TestParsePathYAML_PerOS(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
files:
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty/alacritty.yml
      mac: ~/Library/Application Support/alacritty/alacritty.yml
`)

	mappings, err := ParsePathYAML(path, "linux")
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	assert.Equal(t, "~/.config/alacritty/alacritty.yml", mappings[0].Destination)

	mappingsMac, err := ParsePathYAML(path, "mac")
	require.NoError(t, err)
	require.Len(t, mappingsMac, 1)
	assert.Contains(t, mappingsMac[0].Destination, "Library")
}

func TestParsePathYAML_OSFilter(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
files:
  - source: gnome.conf
    destination: ~/.config/gnome/settings.conf
    os:
      - linux
`)

	// Should be included on linux
	mappings, err := ParsePathYAML(path, "linux")
	require.NoError(t, err)
	require.Len(t, mappings, 1)

	// Should be excluded on mac
	mappingsMac, err := ParsePathYAML(path, "mac")
	require.NoError(t, err)
	require.Len(t, mappingsMac, 0)
}

func TestParsePathYAML_NoFile(t *testing.T) {
	mappings, err := ParsePathYAML("/nonexistent/path.yaml", "linux")
	require.NoError(t, err)
	require.Nil(t, mappings)
}

func TestParsePathYAML_V2Schema(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
files:
  - source: file.conf
    destination-linux: ~/.config/file.conf  # v2 field
`)

	mappings, err := ParsePathYAML(path, "linux")
	require.NoError(t, err)
	require.Nil(t, mappings) // V2 schema should return nil
}

func TestParseDependencies_Simple(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
dependencies:
  - neovim
  - git
  - zsh
`)

	deps, err := ParseDependencies(path)
	require.NoError(t, err)
	require.Len(t, deps, 3)
	assert.Equal(t, "neovim", deps[0].Name)
	assert.Equal(t, "package", deps[0].Type)
}

func TestParseDependencies_Complex(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
dependencies:
  - name: fd
    type: binary
    url: https://github.com/sharkdp/fd/releases/download/{{version}}/fd-{{version}}-{{arch}}-unknown-linux-musl.tar.gz
    dest: ~/.local/bin/fd
    version: v8.7.0
    extract: fd-{{version}}-{{arch}}-unknown-linux-musl/fd
`)

	deps, err := ParseDependencies(path)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "fd", deps[0].Name)
	assert.Equal(t, "binary", deps[0].Type)
	assert.Equal(t, "v8.7.0", deps[0].Version)
}

func TestParseDependencies_WithManagers(t *testing.T) {
	dir := t.TempDir()
	path := writeTestYAML(t, dir, `
dependencies:
  - name: neovim
    type: package
    managers:
      pacman: neovim
      apt: neovim
      brew: neovim
`)

	deps, err := ParseDependencies(path)
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "neovim", deps[0].Name)
	assert.Equal(t, "neovim", deps[0].Managers["pacman"])
	assert.Equal(t, "neovim", deps[0].Managers["brew"])
}

func TestDetectVariants_NoVariants(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "init.lua", Destination: "~/.config/nvim/init.lua"},
		{Source: "coc-settings.json", Destination: "~/.config/nvim/coc-settings.json"},
	}

	info := DetectVariants(mappings)
	assert.False(t, info.HasVariants)
	assert.Empty(t, info.Variants)
}

func TestDetectVariants_WithVariants(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "nvim", Destination: "~/.config/nvim"},
		{Source: "notevim", Destination: "~/.config/nvim"},
	}

	info := DetectVariants(mappings)
	assert.True(t, info.HasVariants)
	assert.Equal(t, []string{"nvim", "notevim"}, info.Variants)
	assert.Equal(t, "notevim", info.DefaultVariant) // last is default
}

func TestDetectVariants_WithGlobSources(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "wilber/*", Destination: "~/.config/hypr/hyprland.conf"},
		{Source: "canto/*", Destination: "~/.config/hypr/hyprland.conf"},
	}

	info := DetectVariants(mappings)
	assert.True(t, info.HasVariants)
	assert.Contains(t, info.Variants, "wilber")
	assert.Contains(t, info.Variants, "canto")
}

func TestFilterByVariant(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "nvim", Destination: "~/.config/nvim"},
		{Source: "notevim", Destination: "~/.config/nvim"},
	}

	filtered := FilterByVariant(mappings, "nvim")
	require.Len(t, filtered, 1)
	assert.Equal(t, "nvim", filtered[0].Source)

	filtered2 := FilterByVariant(mappings, "notevim")
	require.Len(t, filtered2, 1)
	assert.Equal(t, "notevim", filtered2[0].Source)
}

func TestFilterByVariant_Empty(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "file.conf", Destination: "~/.config/file.conf"},
	}

	filtered := FilterByVariant(mappings, "")
	assert.Equal(t, len(mappings), len(filtered))
}

func TestFilterByVariant_GlobNormalized(t *testing.T) {
	mappings := []DotFileMapping{
		{Source: "wilber/*", Destination: "~/.config/hypr/hyprland.conf"},
		{Source: "canto/*", Destination: "~/.config/hypr/hyprland.conf"},
	}

	filtered := FilterByVariant(mappings, "wilber")
	require.Len(t, filtered, 1)
	assert.Equal(t, "wilber/*", filtered[0].Source)
}

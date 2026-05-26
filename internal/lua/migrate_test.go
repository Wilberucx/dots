package lua

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── InitLuaTemplate ────────────────────────────────────────────────────────

func TestInitLuaTemplate_WithName(t *testing.T) {
	result := InitLuaTemplate("cantoarch/dotfiles")
	assert.Contains(t, result, `name = "cantoarch/dotfiles"`)
	assert.Contains(t, result, "return {")
	assert.Contains(t, result, "module_paths")
	assert.Contains(t, result, "plugins")
}

func TestInitLuaTemplate_EmptyName(t *testing.T) {
	result := InitLuaTemplate("")
	assert.Contains(t, result, `name = "dotfiles"`)
}

func TestInitLuaTemplateMinimal(t *testing.T) {
	assert.Contains(t, InitLuaTemplateMinimal, `name = "dotfiles"`)
	assert.Contains(t, InitLuaTemplateMinimal, "return {")
}

// ─── MigrateModule ──────────────────────────────────────────────────────────

func TestMigrateModule_SimpleFile(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Nvim")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)
	assert.NotEmpty(t, luaContent)

	assert.Contains(t, luaContent, `file("init.lua", "~/.config/nvim/init.lua")`)
	assert.Contains(t, luaContent, "return {")
	assert.Contains(t, luaContent, "files = {")
}

func TestMigrateModule_DirInto(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Scripts")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	// Destination with /* should generate dir():into() (expand contents)
	yamlContent := `files:
  - source: scripts
    destination: ~/.local/bin/*
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `dir("scripts"):into("~/.local/bin")`)
}

func TestMigrateModule_DirTo(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Config")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	// Directory source without /* should generate dir():to() (symlink entire dir)
	yamlContent := `files:
  - source: config
    destination: ~/.config/tool
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `dir("config"):to("~/.config/tool")`)
}

func TestMigrateModule_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "EmptyFiles")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `files: []
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	// Should not have a "files = {" block when files are empty
	assert.NotContains(t, luaContent, "files = {")
}

func TestMigrateModule_EmptyDeps(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "EmptyDeps")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `dependencies: []
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	// Should not have a "dependencies = {" block when deps are empty
	assert.NotContains(t, luaContent, "dependencies = {")
}

func TestMigrateModule_WithType(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Zsh")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `type: minimal
files:
  - source: .zshrc
    destination: ~/.zshrc
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `type = "minimal"`)
	assert.Contains(t, luaContent, `file(".zshrc", "~/.zshrc")`)
}

func TestMigrateModule_PkgDeps(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Tools")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `dependencies:
  - neovim
  - ripgrep
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `pkg "neovim"`)
	assert.Contains(t, luaContent, `pkg "ripgrep"`)
	assert.Contains(t, luaContent, "dependencies = {")
}

func TestMigrateModule_BinaryDep(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Fd")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `dependencies:
  - name: fd
    type: binary
    url: https://github.com/sharkdp/fd/releases/download/v8.7.0/fd.tar.gz
    dest: ~/.local/bin/fd
    extract: fd/fd
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `curl("https://github.com/sharkdp/fd/releases/download/v8.7.0/fd.tar.gz")`)
	assert.Contains(t, luaContent, `:extract("fd/fd")`)
	assert.Contains(t, luaContent, `:to("~/.local/bin/fd")`)
}

func TestMigrateModule_GitDep(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "P10k")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `dependencies:
  - name: powerlevel10k
    type: git
    url: https://github.com/romkatv/powerlevel10k.git
    dest: ~/.local/share/zsh/plugins/p10k
    ref: v1.19.0
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `git("https://github.com/romkatv/powerlevel10k.git")`)
	assert.Contains(t, luaContent, `:to("~/.local/share/zsh/plugins/p10k")`)
	assert.Contains(t, luaContent, `:at("v1.19.0")`)
}

func TestMigrateModule_WithManagers(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Neovim")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `dependencies:
  - name: neovim
    type: package
    managers:
      pacman: neovim
      apt: neovim
      brew: neovim
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `pkg("neovim")`)
	assert.Contains(t, luaContent, `:on({`)
	assert.Contains(t, luaContent, `pacman = "neovim"`)
	assert.Contains(t, luaContent, `brew = "neovim"`)
}

func TestMigrateModule_PerOS(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Alacritty")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := `files:
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty/alacritty.yml
      mac: ~/Library/Application Support/alacritty/alacritty.yml
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `file("alacritty.yml"`)
	assert.Contains(t, luaContent, `:per_os({`)
	assert.Contains(t, luaContent, `linux = "~/.config/alacritty/alacritty.yml"`)
}

func TestMigrateModule_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Exists")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	// dots.lua already exists
	err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	// path.yaml also exists
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files: []"), 0644)
	require.NoError(t, err)

	_, err = MigrateModule(modDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMigrateModule_DirIntoWithPerOS(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Scripts")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	// per-os + dest with /* — should generate dir():into() with first OS dest + comment
	yamlContent := `files:
  - source: scripts
    destination: ~/.local/bin/*
    per-os:
      linux: ~/.local/bin/*
      mac: ~/Library/bin/*
`
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	assert.Contains(t, luaContent, `TODO: per-OS dir expansion`, "should warn about unsupported per-OS dir")
	assert.Contains(t, luaContent, `dir("scripts"):into("~/.local/bin")`, "should use first OS dest")
}

func TestMigrateModule_NoPathYAML(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "NoYAML")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	_, err = MigrateModule(modDir)
	assert.Error(t, err)
}

func TestMigrateModule_EmptyYAML(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "Empty")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	_, err = MigrateModule(modDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// ─── generateFileEntry ──────────────────────────────────────────────────────

func TestGenerateFileEntry_Simple(t *testing.T) {
	f := map[string]interface{}{
		"source":      "init.lua",
		"destination": "~/.config/nvim/init.lua",
	}
	result := generateFileEntry(f)
	assert.Equal(t, `file("init.lua", "~/.config/nvim/init.lua")`, result)
}

func TestGenerateFileEntry_DirInto(t *testing.T) {
	f := map[string]interface{}{
		"source":      "scripts",
		"destination": "~/.local/bin/*",
	}
	result := generateFileEntry(f)
	assert.Contains(t, result, `dir("scripts"):into("~/.local/bin")`)
}

func TestGenerateFileEntry_PerOS(t *testing.T) {
	f := map[string]interface{}{
		"source": "alacritty.yml",
		"per-os": map[string]interface{}{
			"linux": "~/.config/alacritty.yml",
			"mac":   "~/Library/alacritty.yml",
		},
	}
	result := generateFileEntry(f)
	assert.Contains(t, result, `per_os({`)
	assert.Contains(t, result, `linux = "~/.config/alacritty.yml"`)
}

func TestGenerateFileEntry_OSFilter(t *testing.T) {
	f := map[string]interface{}{
		"source":      "linux-only.conf",
		"destination": "~/.config/linux.conf",
		"os":          []interface{}{"linux"},
	}
	result := generateFileEntry(f)
	assert.Equal(t, `file("linux-only.conf", "~/.config/linux.conf"):when("linux")`, result)
}

// ─── generateDepEntry ───────────────────────────────────────────────────────

func TestGenerateDepEntry_PkgSimple(t *testing.T) {
	d := map[string]interface{}{
		"name": "ripgrep",
		"type": "package",
	}
	result := generateDepEntry(d)
	assert.Equal(t, `pkg "ripgrep"`, result)
}

func TestGenerateDepEntry_PkgWithManagers(t *testing.T) {
	d := map[string]interface{}{
		"name": "fd",
		"type": "package",
		"managers": map[string]interface{}{
			"pacman": "fd",
			"apt":    "fd-find",
		},
	}
	result := generateDepEntry(d)
	assert.Contains(t, result, `pkg("fd")`)
	assert.Contains(t, result, `:on({`)
	assert.Contains(t, result, `apt = "fd-find"`)
	assert.Contains(t, result, `pacman = "fd"`)
}

func TestGenerateDepEntry_Binary(t *testing.T) {
	d := map[string]interface{}{
		"name":    "eza",
		"type":    "binary",
		"url":     "https://example.com/eza.tar.gz",
		"dest":    "~/.local/bin/eza",
		"extract": "eza",
	}
	result := generateDepEntry(d)
	assert.Contains(t, result, `curl("https://example.com/eza.tar.gz")`)
	assert.Contains(t, result, `:extract("eza")`)
	assert.Contains(t, result, `:to("~/.local/bin/eza")`)
}

func TestGenerateDepEntry_Git(t *testing.T) {
	d := map[string]interface{}{
		"name": "p10k",
		"type": "git",
		"url":  "https://github.com/user/repo.git",
		"dest": "~/.local/share/plugins/p10k",
		"ref":  "v1.0.0",
	}
	result := generateDepEntry(d)
	assert.Contains(t, result, `git("https://github.com/user/repo.git")`)
	assert.Contains(t, result, `:to("~/.local/share/plugins/p10k")`)
	assert.Contains(t, result, `:at("v1.0.0")`)
}

func TestGenerateDepEntry_BinaryWithArch(t *testing.T) {
	d := map[string]interface{}{
		"name": "fd",
		"type": "binary",
		"url":  "https://example.com/fd.tar.gz",
		"arch": map[string]interface{}{
			"x86_64":  "amd64",
			"aarch64": "arm64",
		},
	}
	result := generateDepEntry(d)
	assert.Contains(t, result, `curl("https://example.com/fd.tar.gz")`)
	// Use separate assertions per key-value pair to avoid map ordering brittleness
	assert.Contains(t, result, `aarch64 = "arm64"`)
	assert.Contains(t, result, `x86_64 = "amd64"`)
	assert.Contains(t, result, `:arch({`)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func TestIsDirLike(t *testing.T) {
	tests := []struct {
		source string
		dir    bool
	}{
		{source: "config", dir: true},
		{source: "scripts", dir: true},
		{source: "scripts/", dir: true},
		{source: ".zshrc", dir: false},
		{source: "init.lua", dir: false},
		{source: "config.toml", dir: false},
		{source: "*.toml", dir: false},
		{source: "file?name", dir: false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := isDirLike(tt.source)
			assert.Equal(t, tt.dir, result, "isDirLike(%q)", tt.source)
		})
	}
}

// ─── WriteLuaModule / WriteInitLua ──────────────────────────────────────────

func TestWriteLuaModule(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "TestMod")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	err = WriteLuaModule(modDir, "return { type = \"test\" }")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(modDir, "dots.lua"))
	require.NoError(t, err)
	assert.Equal(t, "return { type = \"test\" }", string(content))
}

func TestWriteInitLua(t *testing.T) {
	dir := t.TempDir()
	err := WriteInitLua(dir, "return { name = \"test\" }")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "init.lua"))
	require.NoError(t, err)
	assert.Equal(t, "return { name = \"test\" }", string(content))
}

// ─── Integration: Full migration round-trip ─────────────────────────────────

func TestIntegration_FullModuleMigration(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "FullMod")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	yamlContent := strings.TrimSpace(`
type: full
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty.yml
      mac: ~/Library/alacritty.yml
  - source: scripts
    destination: ~/.local/bin/*
dependencies:
  - neovim
  - name: fd
    type: binary
    url: https://example.com/fd.tar.gz
    dest: ~/.local/bin/fd
    extract: fd/fd
  - name: p10k
    type: git
    url: https://github.com/user/p10k.git
    dest: ~/p10k
    ref: v1.0
`)

	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte(yamlContent), 0644)
	require.NoError(t, err)

	luaContent, err := MigrateModule(modDir)
	require.NoError(t, err)

	// Verify all expected elements are present
	assert.Contains(t, luaContent, `type = "full"`)
	assert.Contains(t, luaContent, `file("init.lua", "~/.config/nvim/init.lua")`)
	assert.Contains(t, luaContent, `per_os({`)
	assert.Contains(t, luaContent, `dir("scripts"):into("~/.local/bin")`)
	assert.Contains(t, luaContent, `pkg "neovim"`)
	assert.Contains(t, luaContent, `curl("https://example.com/fd.tar.gz")`)
	assert.Contains(t, luaContent, `:bin("fd")`) // name preserved via :bin()
	assert.Contains(t, luaContent, `git("https://github.com/user/p10k.git")`)
	assert.Contains(t, luaContent, `:bin("p10k")`) // name preserved via :bin()

	// Verify the generated Lua parses back correctly
	writeLuaFile(t, modDir, "dots.lua", luaContent)
	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(filepath.Join(modDir, "dots.lua"))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "full", cfg.Type)
	assert.Len(t, cfg.Files, 3)
	assert.Len(t, cfg.Dependencies, 3)

	// Verify file ops
	assert.Equal(t, FileOpFile, cfg.Files[0].Type)
	assert.Equal(t, FileOpFile, cfg.Files[1].Type)
	assert.Equal(t, FileOpDirInto, cfg.Files[2].Type)

	// Verify deps — curl/git preserve name via :bin(), so Name = "fd" / "p10k"
	assert.Equal(t, "neovim", cfg.Dependencies[0].Name)
	assert.Equal(t, "fd", cfg.Dependencies[1].Bin, "binary dep name stored in Bin field")
	assert.Equal(t, "binary", cfg.Dependencies[1].Type)
	assert.Equal(t, "https://example.com/fd.tar.gz", cfg.Dependencies[1].URL)
	assert.Equal(t, "fd/fd", cfg.Dependencies[1].Extract)
	assert.Equal(t, "~/.local/bin/fd", cfg.Dependencies[1].Destination)

	assert.Equal(t, "p10k", cfg.Dependencies[2].Bin, "git dep name stored in Bin field")
	assert.Equal(t, "git", cfg.Dependencies[2].Type)
	assert.Equal(t, "https://github.com/user/p10k.git", cfg.Dependencies[2].URL)
	assert.Equal(t, "v1.0", cfg.Dependencies[2].Ref)
}

package lua

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Types helpers ──────────────────────────────────────────────────────────

func TestLuaValToString(t *testing.T) {
	tests := []struct {
		name  string
		input lua.LValue
		want  string
	}{
		{name: "string", input: lua.LString("hello"), want: "hello"},
		{name: "number", input: lua.LNumber(42), want: "42"},
		{name: "bool", input: lua.LBool(true), want: "true"},
		{name: "nil", input: lua.LNil, want: "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LuaValToString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLuaTableToStringMap(t *testing.T) {
	tbl := &lua.LTable{}
	tbl.RawSetString("linux", lua.LString("~/.config/linux.conf"))
	tbl.RawSetString("mac", lua.LString("~/Library/linux.conf"))

	result := LuaTableToStringMap(tbl)
	assert.Equal(t, "~/.config/linux.conf", result["linux"])
	assert.Equal(t, "~/Library/linux.conf", result["mac"])
}

func TestLuaTableToStringSlice(t *testing.T) {
	tbl := &lua.LTable{}
	tbl.Append(lua.LString("one"))
	tbl.Append(lua.LString("two"))
	tbl.Append(lua.LString("three"))

	result := LuaTableToStringSlice(tbl)
	assert.Equal(t, []string{"one", "two", "three"}, result)
}

// ─── NewLuaVM ───────────────────────────────────────────────────────────────

func TestNewLuaVM(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	// Verify all global functions are registered
	for _, name := range []string{"file", "dir", "glob", "pkg", "curl", "git"} {
		val := vm.L.GetGlobal(name)
		assert.Equal(t, lua.LTFunction, val.Type(), "expected %s to be a function", name)
	}
}

// ─── LoadModuleConfig ───────────────────────────────────────────────────────

func writeLuaFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestLoadModuleConfig_SimpleFile(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  type = "minimal",
  files = {
    file(".zshrc", "~/.zshrc"),
    file(".zshenv", "~/.zshenv"):when("linux"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "minimal", cfg.Type)
	require.Len(t, cfg.Files, 2)

	// First file
	assert.Equal(t, FileOpFile, cfg.Files[0].Type)
	assert.Equal(t, ".zshrc", cfg.Files[0].Source)
	assert.Equal(t, "~/.zshrc", cfg.Files[0].Destination)
	assert.Empty(t, cfg.Files[0].OSFilter)

	// Second file with OS filter
	assert.Equal(t, FileOpFile, cfg.Files[1].Type)
	assert.Equal(t, ".zshenv", cfg.Files[1].Source)
	assert.Equal(t, "~/.zshenv", cfg.Files[1].Destination)
	assert.Equal(t, "linux", cfg.Files[1].OSFilter)
}

func TestLoadModuleConfig_PerOS(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
      mac   = "~/Library/Application Support/alacritty.toml",
      linux = "~/.config/alacritty.toml",
    }),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 1)

	f := cfg.Files[0]
	assert.Equal(t, FileOpFile, f.Type)
	assert.Equal(t, "alacritty.toml", f.Source)
	assert.Equal(t, "~/.config/alacritty/alacritty.toml", f.Destination)
	assert.Equal(t, "~/.config/alacritty.toml", f.PerOS["linux"])
	assert.Contains(t, f.PerOS["mac"], "Library")
}

func TestLoadModuleConfig_DirTo(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    dir("config"):to("~/.config/tool"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 1)

	f := cfg.Files[0]
	assert.Equal(t, FileOpDirTo, f.Type)
	assert.Equal(t, "config", f.Source)
	assert.Equal(t, "~/.config/tool", f.Destination)
}

func TestLoadModuleConfig_DirInto(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    dir("scripts"):into("~/.local/bin"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 1)

	f := cfg.Files[0]
	assert.Equal(t, FileOpDirInto, f.Type)
	assert.Equal(t, "scripts", f.Source)
	assert.Equal(t, "~/.local/bin", f.Destination)
}

func TestLoadModuleConfig_Glob(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    glob("*.toml"):into("~/.config/"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 1)

	f := cfg.Files[0]
	assert.Equal(t, FileOpGlob, f.Type)
	assert.Equal(t, "*.toml", f.Pattern)
	assert.Equal(t, "~/.config/", f.Destination)
}

func TestLoadModuleConfig_PkgDeps(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  dependencies = {
    pkg "ripgrep",
    pkg("fd"):on({ pacman = "fd", apt = "fd-find", brew = "fd" }),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Dependencies, 2)

	// First dep: simple pkg shorthand
	d0 := cfg.Dependencies[0]
	assert.Equal(t, "package", d0.Type)
	assert.Equal(t, "ripgrep", d0.Name)

	// Second dep: pkg with managers
	d1 := cfg.Dependencies[1]
	assert.Equal(t, "package", d1.Type)
	assert.Equal(t, "fd", d1.Name)
	assert.Equal(t, "fd", d1.Managers["pacman"])
	assert.Equal(t, "fd-find", d1.Managers["apt"])
	assert.Equal(t, "fd", d1.Managers["brew"])
}

func TestLoadModuleConfig_CurlDep(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  dependencies = {
    curl("https://example.com/eza.tar.gz"):extract("eza"):to("~/.local/bin/eza"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Dependencies, 1)

	d := cfg.Dependencies[0]
	assert.Equal(t, "binary", d.Type)
	assert.Equal(t, "https://example.com/eza.tar.gz", d.URL)
	assert.Equal(t, "eza", d.Extract)
	assert.Equal(t, "~/.local/bin/eza", d.Destination)
}

func TestLoadModuleConfig_GitDep(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  dependencies = {
    git("https://github.com/romkatv/powerlevel10k.git"):to("~/.local/share/zsh/plugins/p10k"):at("v1.19.0"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Dependencies, 1)

	d := cfg.Dependencies[0]
	assert.Equal(t, "git", d.Type)
	assert.Equal(t, "https://github.com/romkatv/powerlevel10k.git", d.URL)
	assert.Equal(t, "~/.local/share/zsh/plugins/p10k", d.Destination)
	assert.Equal(t, "v1.19.0", d.Ref)
}

func TestLoadModuleConfig_DepWithFallback(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  dependencies = {
    pkg("starship"):on({ pacman = "starship", brew = "starship" })
      :fallback(curl("https://github.com/starship.tgz"):extract("starship")),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Dependencies, 1)

	d := cfg.Dependencies[0]
	assert.Equal(t, "package", d.Type)
	assert.Equal(t, "starship", d.Name)
	assert.Equal(t, "starship", d.Managers["pacman"])

	require.NotNil(t, d.Fallback)
	assert.Equal(t, "binary", d.Fallback.Type)
	assert.Equal(t, "https://github.com/starship.tgz", d.Fallback.URL)
	assert.Equal(t, "starship", d.Fallback.Extract)
}

func TestLoadModuleConfig_AllFeatures(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  type = "full",
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("alacritty.toml", "~/.config/alacritty.toml"):per_os({
      linux = "~/.config/alacritty.toml",
      mac = "~/Library/alacritty.toml",
    }),
    file("linux-only.conf", "/dev/null"):when("linux"),
    dir("config"):to("~/.config/myapp"),
    dir("scripts"):into("~/.local/bin"),
    glob("*.theme"):into("~/.themes/"),
  },
  dependencies = {
    pkg "neovim",
    pkg("fd"):on({ apt = "fd-find" }),
    curl("https://example.com/tool.tar.gz"):extract("tool"):to("~/.local/bin/tool"),
    git("https://github.com/user/repo.git"):to("~/repo"):at("main"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "full", cfg.Type)
	assert.Len(t, cfg.Files, 6)
	assert.Len(t, cfg.Dependencies, 4)
}

// ─── Error cases ────────────────────────────────────────────────────────────

func TestLoadModuleConfig_NotExists(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	_, err := vm.LoadModuleConfig("/nonexistent/dots.lua")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadModuleConfig_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  invalid syntax here
}`)

	vm := NewLuaVM()
	defer vm.Close()

	_, err := vm.LoadModuleConfig(luaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestLoadModuleConfig_NotATable(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return "not a table"`)

	vm := NewLuaVM()
	defer vm.Close()

	_, err := vm.LoadModuleConfig(luaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return a table")
}

// ─── LoadRootConfig ─────────────────────────────────────────────────────────

func TestLoadRootConfig_Simple(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return {
  name = "cantoarch/dotfiles",
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "cantoarch/dotfiles", cfg.Name)
	assert.Empty(t, cfg.ModulePaths)
	assert.Empty(t, cfg.Plugins)
}

func TestLoadRootConfig_WithModulePaths(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return {
  name = "dotfiles",
  module_paths = "modules/",
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "dotfiles", cfg.Name)
	assert.Equal(t, []string{"modules/"}, cfg.ModulePaths)
	assert.Empty(t, cfg.Plugins)
}

func TestLoadRootConfig_MultipleModulePaths(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return {
  name = "dotfiles",
  module_paths = { "packages/", "custom/" },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, []string{"packages/", "custom/"}, cfg.ModulePaths)
}

func TestLoadRootConfig_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return {
  name = "dotfiles",
  plugins = { "dots.http", "dots.archive", "dots.git" },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, []string{"dots.http", "dots.archive", "dots.git"}, cfg.Plugins)
}

func TestLoadRootConfig_NotExists(t *testing.T) {
	dir := t.TempDir()

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(filepath.Join(dir, "init.lua"))
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadRootConfig_DefaultName(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return {}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Should get default name
	assert.Equal(t, "dotfiles", cfg.Name)
}

func TestLoadRootConfig_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "init.lua", `return { broken`)

	vm := NewLuaVM()
	defer vm.Close()

	_, err := vm.LoadRootConfig(luaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

// ─── CheckSyntax ────────────────────────────────────────────────────────────

func TestCheckSyntax_Valid(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = { file("a", "~/.a") },
  dependencies = { pkg "x" },
}`)

	err := CheckSyntax(luaPath)
	assert.NoError(t, err)
}

func TestCheckSyntax_Invalid(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return { broken syntax`)

	err := CheckSyntax(luaPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestCheckSyntax_NotExists(t *testing.T) {
	err := CheckSyntax("/nonexistent/dots.lua")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── parseModuleConfig ──────────────────────────────────────────────────────

func TestParseModuleConfig_Empty(t *testing.T) {
	tbl := &lua.LTable{}
	cfg, err := parseModuleConfig(tbl)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.Type)
	assert.Empty(t, cfg.Files)
	assert.Empty(t, cfg.Dependencies)
}

func TestParseModuleConfig_TypeOnly(t *testing.T) {
	tbl := &lua.LTable{}
	tbl.RawSetString("type", lua.LString("minimal"))

	cfg, err := parseModuleConfig(tbl)
	require.NoError(t, err)
	assert.Equal(t, "minimal", cfg.Type)
}

// ─── parseRootConfig ────────────────────────────────────────────────────────

func TestParseRootConfig_Empty(t *testing.T) {
	tbl := &lua.LTable{}
	cfg, err := parseRootConfig(tbl)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "dotfiles", cfg.Name) // default name
}

func TestParseRootConfig_Full(t *testing.T) {
	tbl := &lua.LTable{}
	tbl.RawSetString("name", lua.LString("my/dotfiles"))

	// module_paths table
	mpTbl := &lua.LTable{}
	mpTbl.Append(lua.LString("pkgs/"))
	mpTbl.Append(lua.LString("cfg/"))
	tbl.RawSetString("module_paths", mpTbl)

	// plugins table
	plTbl := &lua.LTable{}
	plTbl.Append(lua.LString("dots.http"))
	tbl.RawSetString("plugins", plTbl)

	cfg, err := parseRootConfig(tbl)
	require.NoError(t, err)
	assert.Equal(t, "my/dotfiles", cfg.Name)
	assert.Equal(t, []string{"pkgs/", "cfg/"}, cfg.ModulePaths)
	assert.Equal(t, []string{"dots.http"}, cfg.Plugins)
}

// ─── Helper functions ───────────────────────────────────────────────────────

func TestFileOpTypeString(t *testing.T) {
	tests := []struct {
		opType FileOpType
		want   string
	}{
		{FileOpFile, "file"},
		{FileOpDirTo, "dir:to"},
		{FileOpDirInto, "dir:into"},
		{FileOpGlob, "glob"},
		{FileOpType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, fileOpTypeString(tt.opType))
		})
	}
}

func TestDetectDepTypeFromTable(t *testing.T) {
	tbl := &lua.LTable{}
	assert.Equal(t, "package", detectDepTypeFromTable(tbl))

	tbl2 := &lua.LTable{}
	tbl2.RawSetString("url", lua.LString("https://example.com/file.tar.gz"))
	assert.Equal(t, "binary", detectDepTypeFromTable(tbl2))

	tbl3 := &lua.LTable{}
	tbl3.RawSetString("url", lua.LString("https://github.com/user/repo.git"))
	tbl3.RawSetString("ref", lua.LString("v1.0"))
	assert.Equal(t, "git", detectDepTypeFromTable(tbl3))
}

// ─── Integration: Full round-trip ───────────────────────────────────────────

// ─── Variant tests ────────────────────────────────────────────────────────────

func TestLoadModuleConfig_FileWithVariant(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    file("common/gitconfig", "~/.gitconfig"),
    file("work/gitconfig", "~/.gitconfig"):variant("work"),
    file("personal/gitconfig", "~/.gitconfig"):variant("personal"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 3)

	// Base file: no variant
	assert.Equal(t, "common/gitconfig", cfg.Files[0].Source)
	assert.Empty(t, cfg.Files[0].VariantName)

	// Work variant
	assert.Equal(t, "work/gitconfig", cfg.Files[1].Source)
	assert.Equal(t, "work", cfg.Files[1].VariantName)

	// Personal variant
	assert.Equal(t, "personal/gitconfig", cfg.Files[2].Source)
	assert.Equal(t, "personal", cfg.Files[2].VariantName)
}

func TestLoadModuleConfig_DirWithVariant(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    dir("config"):to("~/.config/app"):variant("work"),
    dir("config-default"):to("~/.config/app"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 2)

	assert.Equal(t, "work", cfg.Files[0].VariantName)
	assert.Equal(t, FileOpDirTo, cfg.Files[0].Type)
	assert.Equal(t, "~/.config/app", cfg.Files[0].Destination)

	assert.Empty(t, cfg.Files[1].VariantName)
}

func TestLoadModuleConfig_GlobWithVariant(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    glob("*.toml"):into("~/.config/"):variant("work"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 1)

	assert.Equal(t, "work", cfg.Files[0].VariantName)
	assert.Equal(t, FileOpGlob, cfg.Files[0].Type)
	assert.Equal(t, "*.toml", cfg.Files[0].Pattern)
}

func TestLoadModuleConfig_VariantWithMethods(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "dots.lua", `return {
  files = {
    file("work/config", "~/.config/app"):when("linux"):variant("work"),
    file("personal/config", "~/.config/app"):per_os({
      linux = "~/.config/app-linux",
      mac = "~/Library/app",
    }):variant("personal"),
  },
}`)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.Len(t, cfg.Files, 2)

	// First: when + variant
	assert.Equal(t, "work", cfg.Files[0].VariantName)
	assert.Equal(t, "linux", cfg.Files[0].OSFilter)

	// Second: per_os + variant
	assert.Equal(t, "personal", cfg.Files[1].VariantName)
	assert.Equal(t, "~/.config/app-linux", cfg.Files[1].PerOS["linux"])
}

// ─── Integration: Full round-trip ───────────────────────────────────────────

func TestIntegration_FileDepRoundTrip(t *testing.T) {
	dir := t.TempDir()
	content := `return {
  type = "full",
  files = {
    file(".zshrc", "~/.zshrc"):when("linux"),
    dir("config"):to("~/.config/app"),
    dir("scripts"):into("~/.local/bin"),
  },
  dependencies = {
    pkg "ripgrep",
    git("https://github.com/user/repo.git"):to("~/repo"):at("main"),
  },
}`
	luaPath := writeLuaFile(t, dir, "dots.lua", content)

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "full", cfg.Type)
	assert.Len(t, cfg.Files, 3)
	assert.Len(t, cfg.Dependencies, 2)

	// Verify file ordering
	assert.Equal(t, ".zshrc", cfg.Files[0].Source)
	assert.Equal(t, "linux", cfg.Files[0].OSFilter)
	assert.Equal(t, "config", cfg.Files[1].Source)
	assert.Equal(t, FileOpDirTo, cfg.Files[1].Type)
	assert.Equal(t, "scripts", cfg.Files[2].Source)
	assert.Equal(t, FileOpDirInto, cfg.Files[2].Type)

	// Verify deps
	assert.Equal(t, "ripgrep", cfg.Dependencies[0].Name)
	assert.Equal(t, "git", cfg.Dependencies[1].Type)
	assert.Equal(t, "main", cfg.Dependencies[1].Ref)
}

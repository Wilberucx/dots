package lua

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── FindModules ────────────────────────────────────────────────────────────

func TestFindModules_EmptyRepo(t *testing.T) {
	dir := t.TempDir()

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	assert.Empty(t, modules)
}

func TestFindModules_LuaModules(t *testing.T) {
	dir := t.TempDir()

	// Create modules with dots.lua
	for _, name := range []string{"Zsh", "Nvim", "Kitty"} {
		modDir := filepath.Join(dir, name)
		err := os.MkdirAll(modDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte("return {}"), 0644)
		require.NoError(t, err)
	}

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	require.Len(t, modules, 3)

	// Should be sorted by name
	assert.Equal(t, "Kitty", modules[0].Name)
	assert.Equal(t, "Nvim", modules[1].Name)
	assert.Equal(t, "Zsh", modules[2].Name)

	// All should be Lua type
	for _, m := range modules {
		assert.Equal(t, ModuleTypeLua, m.Type)
	}
}

func TestFindModules_YAMLModules(t *testing.T) {
	dir := t.TempDir()

	// Create modules with path.yaml
	for _, name := range []string{"Alacritty", "Tmux"} {
		modDir := filepath.Join(dir, name)
		err := os.MkdirAll(modDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files: []"), 0644)
		require.NoError(t, err)
	}

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	require.Len(t, modules, 2)
	assert.Equal(t, ModuleTypeYAML, modules[0].Type)
}

func TestFindModules_BothFormats_LuaWins(t *testing.T) {
	dir := t.TempDir()

	modDir := filepath.Join(dir, "Hyprland")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	// Create both dots.lua and path.yaml
	err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files:\n  - source: x\n    destination: ~/x"), 0644)
	require.NoError(t, err)

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	// Lua should take priority
	assert.Equal(t, ModuleTypeLua, modules[0].Type)
}

func TestFindModules_WithModulePaths(t *testing.T) {
	dir := t.TempDir()

	// Create modules in packages/ and custom/
	for _, sub := range []string{"packages", "custom"} {
		subDir := filepath.Join(dir, sub)
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		modName := sub + "-mod"
		modDir := filepath.Join(subDir, modName)
		err = os.MkdirAll(modDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte("return {}"), 0644)
		require.NoError(t, err)
	}

	// Create a module in root (should NOT be found when module_paths restricts)
	rootMod := filepath.Join(dir, "RootMod")
	err := os.MkdirAll(rootMod, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(rootMod, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	initCfg := &RootConfig{
		Name:        "test",
		ModulePaths: []string{"packages/", "custom/"},
	}

	modules, err := FindModules(dir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 2)

	// Should only find modules in packages/ and custom/
	names := make(map[string]bool)
	for _, m := range modules {
		names[m.Name] = true
	}
	assert.True(t, names["packages-mod"], "should find module in packages/")
	assert.True(t, names["custom-mod"], "should find module in custom/")
	assert.False(t, names["RootMod"], "should NOT find module in root")
}

func TestFindModules_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	// Create modules in hidden dirs that should be skipped
	for _, hidden := range []string{".git", ".dots", "node_modules"} {
		subDir := filepath.Join(dir, hidden)
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(subDir, "dots.lua"), []byte("return {}"), 0644)
		require.NoError(t, err)
	}

	// Create a real module
	modDir := filepath.Join(dir, "RealMod")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	assert.Equal(t, "RealMod", modules[0].Name)
}

func TestFindModules_SkipsNoConfig(t *testing.T) {
	dir := t.TempDir()

	// Create directories without any config file
	for _, name := range []string{"EmptyDir", "JustFiles"} {
		subDir := filepath.Join(dir, name)
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)
	}
	// Put a non-config file in JustFiles
	err := os.WriteFile(filepath.Join(dir, "JustFiles", "readme.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	modules, err := FindModules(dir, nil)
	require.NoError(t, err)
	assert.Empty(t, modules)
}

func TestFindModules_DeduplicatesByName(t *testing.T) {
	dir := t.TempDir()

	// Two directories with same name in different scan paths
	pkg1 := filepath.Join(dir, "pkgs1", "SharedMod")
	err := os.MkdirAll(pkg1, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkg1, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	pkg2 := filepath.Join(dir, "pkgs2", "SharedMod")
	err = os.MkdirAll(pkg2, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkg2, "dots.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	initCfg := &RootConfig{
		Name:        "test",
		ModulePaths: []string{"pkgs1/", "pkgs2/"},
	}

	modules, err := FindModules(dir, initCfg)
	require.NoError(t, err)
	// Should only appear once
	require.Len(t, modules, 1)
	assert.Equal(t, "SharedMod", modules[0].Name)
}

// ─── IsLuaRepo ──────────────────────────────────────────────────────────────

func TestIsLuaRepo_InitLua(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte("return { name = \"test\" }"), 0644)
	require.NoError(t, err)

	assert.True(t, IsLuaRepo(dir))
}

func TestIsLuaRepo_ConfigLua(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "config.lua"), []byte("return { name = \"test\" }"), 0644)
	require.NoError(t, err)

	assert.True(t, IsLuaRepo(dir))
}

func TestIsLuaRepo_NotFound(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsLuaRepo(dir))
}

func TestIsLuaRepo_EmptyDir(t *testing.T) {
	assert.False(t, IsLuaRepo("/nonexistent"))
}

func TestIsLuaRepo_InitLuaPriority(t *testing.T) {
	dir := t.TempDir()
	// Both exist, init.lua should be detected
	err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "config.lua"), []byte("return {}"), 0644)
	require.NoError(t, err)

	assert.True(t, IsLuaRepo(dir))
}

// ─── LoadInitConfig ─────────────────────────────────────────────────────────

func TestLoadInitConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`return {
  name = "test/dotfiles",
  module_paths = "modules/",
  plugins = { "dots.http" },
}`), 0644)
	require.NoError(t, err)

	cfg, err := LoadInitConfig(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "test/dotfiles", cfg.Name)
	assert.Equal(t, []string{"modules/"}, cfg.ModulePaths)
	assert.Equal(t, []string{"dots.http"}, cfg.Plugins)
}

func TestLoadInitConfig_NotExists(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadInitConfig(dir)
	require.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestLoadInitConfig_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte("return { broken syntax"), 0644)
	require.NoError(t, err)

	_, err = LoadInitConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

// ─── LoadModuleConfigForModule ──────────────────────────────────────────────

func TestLoadModuleConfigForModule_Lua(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "TestMod")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte(`return {
  type = "test",
  files = { file("a", "~/.a") },
}`), 0644)
	require.NoError(t, err)

	module := ModuleDir{
		Name: "TestMod",
		Path: modDir,
		Type: ModuleTypeLua,
	}

	cfg, err := LoadModuleConfigForModule(module)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "test", cfg.Type)
	assert.Len(t, cfg.Files, 1)
}

func TestLoadModuleConfigForModule_YAML(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "YAMLMod")
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(modDir, "path.yaml"), []byte("files: []"), 0644)
	require.NoError(t, err)

	module := ModuleDir{
		Name: "YAMLMod",
		Path: modDir,
		Type: ModuleTypeYAML,
	}

	cfg, err := LoadModuleConfigForModule(module)
	require.NoError(t, err)
	assert.Nil(t, cfg) // YAML modules return nil from the Lua loader
}

func TestLoadModuleConfigForModule_NotExists(t *testing.T) {
	dir := t.TempDir()
	module := ModuleDir{
		Name: "NonExistent",
		Path: filepath.Join(dir, "NonExistent"),
		Type: ModuleTypeLua,
	}

	_, err := LoadModuleConfigForModule(module)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── ConvertYAMLDepsToDepOps ────────────────────────────────────────────────

func TestConvertYAMLDepsToDepOps_Empty(t *testing.T) {
	result := ConvertYAMLDepsToDepOps(nil)
	assert.Empty(t, result)
}

func TestConvertYAMLDepsToDepOps_Package(t *testing.T) {
	yamlDeps := []interface{}{
		map[string]interface{}{
			"name": "neovim",
			"type": "package",
		},
	}

	result := ConvertYAMLDepsToDepOps(yamlDeps)
	require.Len(t, result, 1)
	assert.Equal(t, "neovim", result[0].Name)
	assert.Equal(t, "package", result[0].Type)
}

func TestConvertYAMLDepsToDepOps_Full(t *testing.T) {
	yamlDeps := []interface{}{
		map[string]interface{}{
			"name":         "fd",
			"type":         "binary",
			"url":          "https://example.com/fd.tar.gz",
			"dest":         "~/.local/bin/fd",
			"extract":      "fd-bin",
			"version":      "v8.0.0",
			"arch": map[string]interface{}{
				"x86_64": "amd64",
				"aarch64": "arm64",
			},
		},
	}

	result := ConvertYAMLDepsToDepOps(yamlDeps)
	require.Len(t, result, 1)
	assert.Equal(t, "fd", result[0].Name)
	assert.Equal(t, "binary", result[0].Type)
	assert.Equal(t, "https://example.com/fd.tar.gz", result[0].URL)
	assert.Equal(t, "~/.local/bin/fd", result[0].Destination)
	assert.Equal(t, "fd-bin", result[0].Extract)
	assert.Equal(t, "v8.0.0", result[0].Version)
	assert.Equal(t, "amd64", result[0].Arch["x86_64"])
}

func TestConvertYAMLDepsToDepOps_WithFallback(t *testing.T) {
	yamlDeps := []interface{}{
		map[string]interface{}{
			"name": "starship",
			"type": "package",
			"fallback": map[string]interface{}{
				"name":    "starship",
				"type":    "binary",
				"url":     "https://example.com/starship.tar.gz",
				"extract": "starship",
				"dest":    "~/.local/bin/starship",
			},
		},
	}

	result := ConvertYAMLDepsToDepOps(yamlDeps)
	require.Len(t, result, 1)
	require.NotNil(t, result[0].Fallback)
	assert.Equal(t, "binary", result[0].Fallback.Type)
	assert.Equal(t, "https://example.com/starship.tar.gz", result[0].Fallback.URL)
}

// ─── IsSubdirOf ─────────────────────────────────────────────────────────────

func TestIsSubdirOf(t *testing.T) {
	assert.True(t, isSubdirOf("/a/b/c", "/a", "/a"))
	assert.True(t, isSubdirOf("/a/b", "/a", "/a"))
	assert.True(t, isSubdirOf("/a", "/a", "/a"))
	assert.False(t, isSubdirOf("/other", "/a", "/a"))
	assert.False(t, isSubdirOf("/a/../other", "/a", "/a"))
}

// ─── SortModules ────────────────────────────────────────────────────────────

func TestSortModules(t *testing.T) {
	modules := []ModuleDir{
		{Name: "Zsh", Path: "/zsh"},
		{Name: "Alacritty", Path: "/alacritty"},
		{Name: "Nvim", Path: "/nvim"},
	}

	sortModules(modules)
	assert.Equal(t, "Alacritty", modules[0].Name)
	assert.Equal(t, "Nvim", modules[1].Name)
	assert.Equal(t, "Zsh", modules[2].Name)
}

func TestSortModules_Empty(t *testing.T) {
	modules := []ModuleDir{}
	sortModules(modules) // should not panic
	assert.Empty(t, modules)
}

func TestSortModules_Single(t *testing.T) {
	modules := []ModuleDir{{Name: "OnlyOne", Path: "/only"}}
	sortModules(modules)
	assert.Len(t, modules, 1)
}

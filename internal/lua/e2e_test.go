// Package lua_test provides end-to-end integration tests for the Lua config pipeline.
// This is an external test package to avoid circular imports with internal/resolver.
package lua_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wilberucx/dots/internal/config"
	luacfg "github.com/Wilberucx/dots/internal/lua"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test helpers ───────────────────────────────────────────────────────────

// createLuaModule creates a module directory with dots.lua and optional source files.
func createLuaModule(t *testing.T, repoRoot, relPath, luaContent string, sourceFiles map[string]string) {
	t.Helper()
	modDir := filepath.Join(repoRoot, relPath)
	err := os.MkdirAll(modDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(modDir, "dots.lua"), []byte(luaContent), 0644)
	require.NoError(t, err)

	for srcPath, content := range sourceFiles {
		fullPath := filepath.Join(modDir, srcPath)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
}

// createSymlink creates a symbolic link ensuring parent directories exist.
func createSymlink(t *testing.T, target, linkPath string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(linkPath), 0755)
	require.NoError(t, err)
	err = os.Symlink(target, linkPath)
	require.NoError(t, err)
}

// findModule finds a luacfg.ModuleDir by name in a slice.
func findModule(t *testing.T, modules []luacfg.ModuleDir, name string) luacfg.ModuleDir {
	t.Helper()
	for _, m := range modules {
		if m.Name == name {
			return m
		}
	}
	require.FailNowf(t, "module not found", "module %q not found in %d modules", name, len(modules))
	return luacfg.ModuleDir{}
}

// findStatus finds a resolver.LinkStatus by source filename in a slice.
func findStatus(t *testing.T, statuses []resolver.LinkStatus, srcFile string) resolver.LinkStatus {
	t.Helper()
	for _, st := range statuses {
		if filepath.Base(st.Source) == srcFile {
			return st
		}
	}
	require.FailNowf(t, "status not found", "status for %q not found in %d statuses", srcFile, len(statuses))
	return resolver.LinkStatus{}
}

// makeDotsCfg creates a properly initialized config.DotsConfig for a Lua repo.
func makeDotsCfg(t *testing.T, repoDir, homeDir string, modules []luacfg.ModuleDir, initCfg *luacfg.RootConfig) *config.DotsConfig {
	t.Helper()

	dotsCfg := &config.DotsConfig{
		RepoRoot:  repoDir,
		CurrentOS: "linux",
		HomeDir:   homeDir,
		CLIDir:    filepath.Join(repoDir, "cli"),
		IsLuaRepo: true,
	}

	if initCfg != nil {
		internalCfg := &config.RootConfig{
			Name:        initCfg.Name,
			ModulePaths: initCfg.ModulePaths,
			Plugins:     initCfg.Plugins,
		}
		dotsCfg.SetInitConfig(internalCfg)
	}

	modDirs := make([]config.ModuleDir, len(modules))
	for i, m := range modules {
		modDirs[i] = config.ModuleDir{
			Name: m.Name,
			Path: m.Path,
			Type: int(m.Type),
		}
	}
	dotsCfg.SetCachedModuleDirs(modDirs)

	return dotsCfg
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Full Lua Pipeline
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_FullLuaPipeline(t *testing.T) {
	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 1: Create init.lua with module_paths
	// ═══════════════════════════════════════════════════════════════════════

	initContent := `return {
  name = "test/dotfiles",
  module_paths = { "packages/", "configs/" },
  plugins = { "dots.http" },
}`
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(initContent), 0644)
	require.NoError(t, err)

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 2: Create modules with various file/dep operations
	// ═══════════════════════════════════════════════════════════════════════

	// Zsh: simple file + OS-filtered files + deps
	createLuaModule(t, repoDir, "packages/Zsh", `return {
  type = "minimal",
  files = {
    file(".zshrc", "~/.zshrc"),
    file(".zshenv", "~/.zshenv"):when("linux"),
    file(".mac-only", "~/.mac"):when("mac"),
  },
  dependencies = {
    pkg "zsh",
    pkg "starship",
  },
}`, map[string]string{
		".zshrc":  "export FOO=bar",
		".zshenv": "export PATH=$PATH:~/.local/bin",
	})

	// Nvim: per_os destinations + files with no filter
	createLuaModule(t, repoDir, "configs/Nvim", `return {
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("lazy-lock.json", "~/.config/nvim/lazy-lock.json"):when("linux"),
    file("nvim.conf", "~/.config/nvim/nvim.conf"):per_os({
      linux = "~/.config/nvim/linux.conf",
      mac   = "~/Library/nvim/mac.conf",
    }),
  },
}`, map[string]string{
		"init.lua":       "vim.g.mapleader = ' '",
		"lazy-lock.json": "{}",
		"nvim.conf":      "{}",
	})

	// Kitty: dependencies only (no files)
	createLuaModule(t, repoDir, "configs/Kitty", `return {
  dependencies = {
    pkg "kitty",
    curl("https://example.com/kitty-themes.tar.gz"):extract("themes"):to("~/.config/kitty/themes"),
  },
}`, nil)

	// Scripts: dir():into() — expand directory contents
	createLuaModule(t, repoDir, "packages/Scripts", `return {
  files = {
    dir("scripts"):into("~/.local/bin"),
  },
}`, map[string]string{
		"scripts/foo.sh": "#!/bin/bash\necho foo",
		"scripts/bar.sh": "#!/bin/bash\necho bar",
	})

	// Alacritty: dir():to() — symlink entire directory
	createLuaModule(t, repoDir, "configs/Alacritty", `return {
  files = {
    dir("config"):to("~/.config/alacritty"),
  },
}`, map[string]string{
		"config/alacritty.toml": "colorscheme = \"catppuccin\"",
		"config/fonts.toml":    "font = { size = 12 }",
	})

	// Root-level module (NOT under module_paths → should NOT be found)
	createLuaModule(t, repoDir, "HiddenMod", `return {
  files = { file("secret", "~/.secret") },
}`, map[string]string{
		"secret": "shh",
	})

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 3: Load init.lua and discover modules
	// ═══════════════════════════════════════════════════════════════════════

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)
	require.NotNil(t, initCfg)
	assert.Equal(t, "test/dotfiles", initCfg.Name)
	assert.Equal(t, []string{"packages/", "configs/"}, initCfg.ModulePaths)
	assert.Equal(t, []string{"dots.http"}, initCfg.Plugins)

	// Verify IsLuaRepo detection
	assert.True(t, luacfg.IsLuaRepo(repoDir), "repo should be detected as Lua repo")

	// Find modules — module_paths restricts search
	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)

	// Should find 5 modules (Zsh + Scripts in packages/, Nvim + Kitty + Alacritty in configs/)
	// HiddenMod at root should NOT be found
	require.Len(t, modules, 5, "should find 5 modules under package_paths, not HiddenMod")

	// Verify module types are Lua
	for _, m := range modules {
		assert.Equal(t, luacfg.ModuleTypeLua, m.Type, "module %s should be Lua type", m.Name)
	}

	// Verify names are sorted
	moduleNames := make([]string, len(modules))
	for i, m := range modules {
		moduleNames[i] = m.Name
	}
	assert.Equal(t, []string{"Alacritty", "Kitty", "Nvim", "Scripts", "Zsh"}, moduleNames)

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 4: Load individual module configs and verify contents
	// ═══════════════════════════════════════════════════════════════════════

	zshMod := findModule(t, modules, "Zsh")
	zshCfg, err := luacfg.LoadModuleConfigForModule(zshMod)
	require.NoError(t, err)
	require.NotNil(t, zshCfg)
	assert.Equal(t, "minimal", zshCfg.Type)
	assert.Len(t, zshCfg.Files, 3, "Zsh: 3 files")
	assert.Len(t, zshCfg.Dependencies, 2, "Zsh: 2 deps")

	assert.Equal(t, ".zshrc", zshCfg.Files[0].Source)
	assert.Equal(t, "~/.zshrc", zshCfg.Files[0].Destination)
	assert.Empty(t, zshCfg.Files[0].OSFilter)

	assert.Equal(t, ".zshenv", zshCfg.Files[1].Source)
	assert.Equal(t, "linux", zshCfg.Files[1].OSFilter)

	assert.Equal(t, ".mac-only", zshCfg.Files[2].Source)
	assert.Equal(t, "mac", zshCfg.Files[2].OSFilter)

	nvimMod := findModule(t, modules, "Nvim")
	nvimCfg, err := luacfg.LoadModuleConfigForModule(nvimMod)
	require.NoError(t, err)
	require.NotNil(t, nvimCfg)
	assert.Len(t, nvimCfg.Files, 3)

	// Verify per_os resolution
	assert.Equal(t, "~/.config/nvim/linux.conf", nvimCfg.Files[2].PerOS["linux"])
	assert.Equal(t, "~/Library/nvim/mac.conf", nvimCfg.Files[2].PerOS["mac"])

	kittyMod := findModule(t, modules, "Kitty")
	kittyCfg, err := luacfg.LoadModuleConfigForModule(kittyMod)
	require.NoError(t, err)
	assert.Len(t, kittyCfg.Files, 0, "Kitty: no files")
	assert.Len(t, kittyCfg.Dependencies, 2, "Kitty: 2 deps")
	assert.Equal(t, "kitty", kittyCfg.Dependencies[0].Name)
	assert.Equal(t, "package", kittyCfg.Dependencies[0].Type)

	scriptsMod := findModule(t, modules, "Scripts")
	scriptsCfg, err := luacfg.LoadModuleConfigForModule(scriptsMod)
	require.NoError(t, err)
	assert.Len(t, scriptsCfg.Files, 1)
	assert.Equal(t, luacfg.FileOpDirInto, scriptsCfg.Files[0].Type)
	assert.Equal(t, "scripts", scriptsCfg.Files[0].Source)

	alacrittyMod := findModule(t, modules, "Alacritty")
	alacrittyCfg, err := luacfg.LoadModuleConfigForModule(alacrittyMod)
	require.NoError(t, err)
	assert.Len(t, alacrittyCfg.Files, 1)
	assert.Equal(t, luacfg.FileOpDirTo, alacrittyCfg.Files[0].Type)
	assert.Equal(t, "config", alacrittyCfg.Files[0].Source)

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 5: Set up DotsConfig (like root.go does) and resolve modules
	// ═══════════════════════════════════════════════════════════════════════

	dotsCfg := makeDotsCfg(t, repoDir, homeDir, modules, initCfg)

	// Resolve all modules (no symlinks exist yet → all pending for OS-matching files)
	results, err := resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	// Zsh: 2 files (linux matches for .zshenv, .mac-only filtered by OS)
	require.Contains(t, results, "Zsh", "Zsh should be in results")
	assert.Len(t, results["Zsh"], 2, "Zsh: 2 files visible (linux OS)")
	// Verify .zshrc and .zshenv are present, .mac-only is NOT
	zshrc := findStatus(t, results["Zsh"], ".zshrc")
	assert.Equal(t, resolver.StatePending, zshrc.State, ".zshrc should be pending")
	zshenv := findStatus(t, results["Zsh"], ".zshenv")
	assert.Equal(t, resolver.StatePending, zshenv.State, ".zshenv should be pending")
	// .mac-only should NOT be present (filtered by OS)
	for _, st := range results["Zsh"] {
		assert.NotEqual(t, ".mac-only", filepath.Base(st.Source), ".mac-only should be filtered out")
	}

	// Nvim: 3 files (init.lua no filter, lazy-lock.json linux, nvim.conf per_os linux)
	require.Contains(t, results, "Nvim", "Nvim should be in results")
	assert.Len(t, results["Nvim"], 3, "Nvim: 3 files visible (linux OS)")
	// Verify per_os resolution chose linux destination
	nvimConf := findStatus(t, results["Nvim"], "nvim.conf")
	assert.Contains(t, nvimConf.Destination, "linux.conf", "nvim.conf should use linux destination")

	// Kitty: no files (deps only) → not in results
	_, kittyInResults := results["Kitty"]
	assert.False(t, kittyInResults, "Kitty (deps-only) should not appear in resolution results")

	// Scripts: 2 files from dir expansion (foo.sh, bar.sh)
	require.Contains(t, results, "Scripts", "Scripts should be in results")
	assert.Len(t, results["Scripts"], 2, "Scripts: 2 files from dir expansion")
	for _, st := range results["Scripts"] {
		assert.Equal(t, resolver.StatePending, st.State)
	}

	// Alacritty: 1 file (dir symlink)
	require.Contains(t, results, "Alacritty", "Alacritty should be in results")
	assert.Len(t, results["Alacritty"], 1, "Alacritty: 1 dir symlink")
	assert.Equal(t, resolver.StatePending, results["Alacritty"][0].State)

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 6: Create symlinks and verify linked detection
	// ═══════════════════════════════════════════════════════════════════════

	// Link .zshrc correctly
	createSymlink(t, filepath.Join(zshMod.Path, ".zshrc"), filepath.Join(homeDir, ".zshrc"))

	// Re-resolve just Zsh
	results, err = resolver.ResolveModules(dotsCfg, []string{"Zsh"}, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	assert.Len(t, results["Zsh"], 2)

	// .zshrc should now be linked
	zshrc = findStatus(t, results["Zsh"], ".zshrc")
	assert.Equal(t, resolver.StateLinked, zshrc.State, ".zshrc should be linked")

	// .zshenv should still be pending
	zshenv = findStatus(t, results["Zsh"], ".zshenv")
	assert.Equal(t, resolver.StatePending, zshenv.State, ".zshenv should still be pending")

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 7: Conflict detection — wrong symlink target
	// ═══════════════════════════════════════════════════════════════════════

	// Create a symlink pointing to the WRONG target
	createSymlink(t, "/wrong/target/init.lua", filepath.Join(homeDir, ".config", "nvim", "init.lua"))

	results, err = resolver.ResolveModules(dotsCfg, []string{"Nvim"}, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Nvim")

	nvimInit := findStatus(t, results["Nvim"], "init.lua")
	assert.Equal(t, resolver.StateConflict, nvimInit.State, "Nvim init.lua should be in conflict (wrong target)")

	// lazy-lock.json should be pending (doesn't exist yet)
	lazyLock := findStatus(t, results["Nvim"], "lazy-lock.json")
	assert.Equal(t, resolver.StatePending, lazyLock.State, "lazy-lock.json should still be pending")

	// ═══════════════════════════════════════════════════════════════════════
	// Phase 8: Link all → all should be linked
	// ═══════════════════════════════════════════════════════════════════════

	// Fix the Nvim conflict by removing the wrong symlink and creating the correct one
	os.Remove(filepath.Join(homeDir, ".config", "nvim", "init.lua"))
	createSymlink(t, filepath.Join(nvimMod.Path, "init.lua"), filepath.Join(homeDir, ".config", "nvim", "init.lua"))

	// Link the remaining files
	createSymlink(t, filepath.Join(nvimMod.Path, "lazy-lock.json"), filepath.Join(homeDir, ".config", "nvim", "lazy-lock.json"))
	createSymlink(t, filepath.Join(nvimMod.Path, "nvim.conf"), filepath.Join(homeDir, ".config", "nvim", "linux.conf"))
	createSymlink(t, filepath.Join(zshMod.Path, ".zshenv"), filepath.Join(homeDir, ".zshenv"))
	createSymlink(t, filepath.Join(scriptsMod.Path, "scripts", "foo.sh"), filepath.Join(homeDir, ".local", "bin", "foo.sh"))
	createSymlink(t, filepath.Join(scriptsMod.Path, "scripts", "bar.sh"), filepath.Join(homeDir, ".local", "bin", "bar.sh"))
	createSymlink(t, filepath.Join(alacrittyMod.Path, "config"), filepath.Join(homeDir, ".config", "alacritty"))

	// Re-resolve all
	results, err = resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	// All modules should have all files linked
	for modName, statuses := range results {
		for _, st := range statuses {
			assert.Equal(t, resolver.StateLinked, st.State,
				"%s/%s should be linked", modName, filepath.Base(st.Source))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Root-level modules (no module_paths)
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_RootLevelModules(t *testing.T) {
	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create init.lua WITHOUT module_paths → search repo root
	initContent := `return {
  name = "test/dotfiles",
}`
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(initContent), 0644)
	require.NoError(t, err)

	// Create modules at repo root
	createLuaModule(t, repoDir, "Zsh", `return {
  files = { file(".zshrc", "~/.zshrc") },
}`, map[string]string{".zshrc": "export FOO=bar"})

	createLuaModule(t, repoDir, "Nvim", `return {
  files = { file("init.lua", "~/.config/nvim/init.lua") },
}`, map[string]string{"init.lua": "vim.g.mapleader = ' '"})

	// Module with subdirectory
	createLuaModule(t, repoDir, "Scripts", `return {
  files = { dir("scripts"):into("~/.local/bin") },
}`, map[string]string{"scripts/go.sh": "#!/bin/bash\necho go"})

	// Nested module inside another module's directory — fully recursive discovery
	createLuaModule(t, repoDir, "Zsh/plugins", `return {
  files = { file("plugin.lua", "~/.zsh/plugin.lua") },
}`, map[string]string{"plugin.lua": "return {}"})

	// Discover modules
	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)
	require.NotNil(t, initCfg)
	assert.Empty(t, initCfg.ModulePaths, "no module_paths specified")

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	// FindModules walks recursively, so nested modules ARE discovered
	require.Len(t, modules, 4, "should find 4 modules (3 root-level + 1 nested)")

	// Verify names
	names := make(map[string]bool)
	for _, m := range modules {
		names[m.Name] = true
	}
	assert.True(t, names["Zsh"], "Zsh should be found")
	assert.True(t, names["Nvim"], "Nvim should be found")
	assert.True(t, names["Scripts"], "Scripts should be found")
	assert.True(t, names["plugins"], "nested 'plugins' should be found (recursive walk)")

	// Resolve and verify
	dotsCfg := makeDotsCfg(t, repoDir, homeDir, modules, initCfg)

	results, err := resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	assert.Contains(t, results, "Zsh")
	assert.Len(t, results["Zsh"], 1)

	assert.Contains(t, results, "Nvim")
	assert.Len(t, results["Nvim"], 1)

	assert.Contains(t, results, "Scripts")
	assert.Len(t, results["Scripts"], 1) // 1 file in scripts/
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Mixed format detection (both dots.lua and path.yaml)
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_MixedFormats(t *testing.T) {
	repoDir := t.TempDir()

	// Create init.lua at root
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	// Create both dots.lua and path.yaml → Lua wins
	createLuaModule(t, repoDir, "Zsh", `return {
  files = { file(".zshrc", "~/.zshrc") },
}`, map[string]string{
		".zshrc": "export FOO=bar",
	})
	// Also write path.yaml
	err = os.WriteFile(filepath.Join(repoDir, "Zsh", "path.yaml"), []byte(`files:
  - source: .zshrc
    destination: ~/.zshrc`), 0644)
	require.NoError(t, err)

	// Create a pure YAML module (only path.yaml)
	yamlModDir := filepath.Join(repoDir, "YAMLOnly")
	err = os.MkdirAll(yamlModDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(yamlModDir, "path.yaml"), []byte(`files:
  - source: config
    destination: ~/.config`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(yamlModDir, "config"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)

	// Find modules
	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 2, "should find 2 modules")

	zshFound := false
	yamlFound := false
	for _, m := range modules {
		switch m.Name {
		case "Zsh":
			zshFound = true
			assert.Equal(t, luacfg.ModuleTypeLua, m.Type, "Zsh should be Lua type (both formats, Lua wins)")
		case "YAMLOnly":
			yamlFound = true
			assert.Equal(t, luacfg.ModuleTypeYAML, m.Type, "YAMLOnly should be YAML type")
		}
	}
	assert.True(t, zshFound, "Zsh module should be found")
	assert.True(t, yamlFound, "YAMLOnly module should be found")

	// Can load Lua module config
	zshMod := findModule(t, modules, "Zsh")
	zshCfg, err := luacfg.LoadModuleConfigForModule(zshMod)
	require.NoError(t, err)
	require.NotNil(t, zshCfg)
	assert.Len(t, zshCfg.Files, 1)
	assert.Equal(t, ".zshrc", zshCfg.Files[0].Source)
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Config loading error propagation
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_LoadErrors(t *testing.T) {
	repoDir := t.TempDir()

	// init.lua with syntax error
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte("return { broken"), 0644)
	require.NoError(t, err)

	// IsLuaRepo should still detect the file
	assert.True(t, luacfg.IsLuaRepo(repoDir))

	// Loading should fail with syntax error
	_, err = luacfg.LoadInitConfig(repoDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")

	// Create a valid init.lua but a module with syntax error
	createLuaModule(t, repoDir, "BrokenMod", `return {
  files = { file("bad" "syntax") }
}`, map[string]string{"bad": "content"})

	err = os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	// Loading the broken module should fail
	_, err = luacfg.LoadModuleConfigForModule(modules[0])
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Module with no dots.lua or path.yaml should be ignored
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_NoConfigDirIgnored(t *testing.T) {
	repoDir := t.TempDir()

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	// Create a directory without any config file
	err = os.MkdirAll(filepath.Join(repoDir, "NoConfig"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "NoConfig", "readme.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	// Create a real module
	createLuaModule(t, repoDir, "RealMod", `return {
  files = { file("f", "~/.f") },
}`, map[string]string{"f": "content"})

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1, "should find only RealMod, ignoring NoConfig")
	assert.Equal(t, "RealMod", modules[0].Name)
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Per-OS destination resolution
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_PerOSResolution(t *testing.T) {
	repoDir := t.TempDir()

	createLuaModule(t, repoDir, "Alacritty", `return {
  files = {
    file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
      linux = "~/.config/alacritty/linux.toml",
      mac   = "~/Library/Application Support/alacritty/mac.toml",
      windows = "~/AppData/alacritty/win.toml",
    }),
  },
}`, map[string]string{"alacritty.toml": "colorscheme = \"catppuccin\""})

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	dotsCfg := makeDotsCfg(t, repoDir, t.TempDir(), modules, initCfg)

	results, err := resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	require.Contains(t, results, "Alacritty")
	require.Len(t, results["Alacritty"], 1)

	// Current OS is linux, so should use linux destination
	assert.Contains(t, results["Alacritty"][0].Destination, "linux.toml",
		"should use linux-specific destination on linux OS")
	assert.NotContains(t, results["Alacritty"][0].Destination, "mac.toml",
		"should NOT use mac destination on linux OS")

	// Now test with mac OS
	dotsCfg.CurrentOS = "mac"
	results, err = resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	require.Contains(t, results, "Alacritty")
	require.Len(t, results["Alacritty"], 1)
	assert.Contains(t, results["Alacritty"][0].Destination, "mac.toml",
		"should use mac-specific destination on mac OS")

	// Test with windows OS
	dotsCfg.CurrentOS = "windows"
	results, err = resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	require.Contains(t, results, "Alacritty")
	require.Len(t, results["Alacritty"], 1)
	assert.Contains(t, results["Alacritty"][0].Destination, "win.toml",
		"should use windows-specific destination on windows OS")
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Dependency loading from Lua modules
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_LuaDependencies(t *testing.T) {
	repoDir := t.TempDir()

	createLuaModule(t, repoDir, "Zsh", `return {
  dependencies = {
    pkg "zsh",
    pkg("starship"):on({ pacman = "starship", apt = "starship", brew = "starship" })
      :fallback(curl("https://github.com/starship/starship/releases/latest.tar.gz"):extract("starship"):to("~/.local/bin/starship")),
    curl("https://github.com/eza-community/eza/releases/latest/eza.tar.gz"):extract("eza"):to("~/.local/bin/eza"),
    git("https://github.com/romkatv/powerlevel10k.git"):to("~/.local/share/zsh/plugins/p10k"):at("v1.19.0"),
  },
}`, nil)

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	assert.Equal(t, "Zsh", modules[0].Name)

	// Load the module config and verify dependencies
	modCfg, err := luacfg.LoadModuleConfigForModule(modules[0])
	require.NoError(t, err)
	require.NotNil(t, modCfg)
	require.Len(t, modCfg.Dependencies, 4)

	// Dep 0: simple pkg
	assert.Equal(t, "package", modCfg.Dependencies[0].Type)
	assert.Equal(t, "zsh", modCfg.Dependencies[0].Name)

	// Dep 1: pkg with managers + fallback
	assert.Equal(t, "package", modCfg.Dependencies[1].Type)
	assert.Equal(t, "starship", modCfg.Dependencies[1].Name)
	assert.Equal(t, "starship", modCfg.Dependencies[1].Managers["pacman"])
	assert.NotNil(t, modCfg.Dependencies[1].Fallback)
	assert.Equal(t, "binary", modCfg.Dependencies[1].Fallback.Type)

	// Dep 2: binary via curl
	assert.Equal(t, "binary", modCfg.Dependencies[2].Type)
	assert.Equal(t, "https://github.com/eza-community/eza/releases/latest/eza.tar.gz", modCfg.Dependencies[2].URL)
	assert.Equal(t, "eza", modCfg.Dependencies[2].Extract)
	assert.Equal(t, "~/.local/bin/eza", modCfg.Dependencies[2].Destination)

	// Dep 3: git
	assert.Equal(t, "git", modCfg.Dependencies[3].Type)
	assert.Equal(t, "https://github.com/romkatv/powerlevel10k.git", modCfg.Dependencies[3].URL)
	assert.Equal(t, "~/.local/share/zsh/plugins/p10k", modCfg.Dependencies[3].Destination)
	assert.Equal(t, "v1.19.0", modCfg.Dependencies[3].Ref)
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Glob pattern matching
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_GlobPatterns(t *testing.T) {
	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	createLuaModule(t, repoDir, "Configs", `return {
  files = {
    glob("*.toml"):into("~/.config/"),
  },
}`, map[string]string{
		"alacritty.toml": "[general]",
		"kitty.toml":     "[font]",
		"starship.toml":  "[format]",
		"readme.txt":     "not a toml", // should not match
	})

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)
	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	dotsCfg := makeDotsCfg(t, repoDir, homeDir, modules, initCfg)

	results, err := resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)

	require.Contains(t, results, "Configs")
	// Should have 3 files matching *.toml (readme.txt excluded)
	var matchedFiles []string
	for _, st := range results["Configs"] {
		matchedFiles = append(matchedFiles, filepath.Base(st.Source))
	}
	assert.Len(t, matchedFiles, 3, "should match 3 .toml files, not readme.txt")
	assert.Contains(t, matchedFiles, "alacritty.toml")
	assert.Contains(t, matchedFiles, "kitty.toml")
	assert.Contains(t, matchedFiles, "starship.toml")

	// Verify all are pending (no symlinks exist)
	for _, st := range results["Configs"] {
		assert.Equal(t, resolver.StatePending, st.State)
	}

	// Create a symlink for one file and verify linked detection
	modPath := findModule(t, modules, "Configs").Path
	createSymlink(t, filepath.Join(modPath, "alacritty.toml"), filepath.Join(homeDir, ".config", "alacritty.toml"))

	results, err = resolver.ResolveModules(dotsCfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Configs")

	alacrittyStatus := findStatus(t, results["Configs"], "alacritty.toml")
	assert.Equal(t, resolver.StateLinked, alacrittyStatus.State, "alacritty.toml should be linked")
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Sorting verification — modules sorted by name
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_ModuleSorting(t *testing.T) {
	repoDir := t.TempDir()

	// No module_paths → scans root
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	// Create modules in unsorted order
	for _, name := range []string{"Zsh", "Alacritty", "Nvim", "Kitty"} {
		createLuaModule(t, repoDir, name, `return {}`, nil)
	}

	modules, err := luacfg.FindModules(repoDir, &luacfg.RootConfig{Name: "test"})
	require.NoError(t, err)
	require.Len(t, modules, 4)

	// Verify alphabetical order
	assert.Equal(t, "Alacritty", modules[0].Name)
	assert.Equal(t, "Kitty", modules[1].Name)
	assert.Equal(t, "Nvim", modules[2].Name)
	assert.Equal(t, "Zsh", modules[3].Name)
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Module deduplication across module_paths
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_DeduplicationAcrossPaths(t *testing.T) {
	repoDir := t.TempDir()

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return {
  name = "test",
  module_paths = { "pkgs1/", "pkgs2/" },
}`), 0644)
	require.NoError(t, err)

	// Create the same module name in both paths
	createLuaModule(t, repoDir, "pkgs1/SharedMod", `return {
  files = { file("v1.conf", "~/.config/v1.conf") },
}`, map[string]string{"v1.conf": "version 1"})

	createLuaModule(t, repoDir, "pkgs2/SharedMod", `return {
  files = { file("v2.conf", "~/.config/v2.conf") },
}`, map[string]string{"v2.conf": "version 2"})

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)
	require.NotNil(t, initCfg)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)

	// Should only appear once (first path wins)
	require.Len(t, modules, 1, "SharedMod should be deduplicated")
	assert.Equal(t, "SharedMod", modules[0].Name)

	// The first path's module should be used
	cfg, err := luacfg.LoadModuleConfigForModule(modules[0])
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Files, 1)
	assert.Equal(t, "v1.conf", cfg.Files[0].Source,
		"first path's config should win on dedup")
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: Empty return from dots.lua (minimal config)
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_EmptyModuleConfig(t *testing.T) {
	repoDir := t.TempDir()

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	// dots.lua that returns {} (no files, no deps)
	createLuaModule(t, repoDir, "Empty", `return {}`, nil)

	initCfg, err := luacfg.LoadInitConfig(repoDir)
	require.NoError(t, err)

	modules, err := luacfg.FindModules(repoDir, initCfg)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	assert.Equal(t, "Empty", modules[0].Name)

	cfg, err := luacfg.LoadModuleConfigForModule(modules[0])
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.Type)
	assert.Empty(t, cfg.Files)
	assert.Empty(t, cfg.Dependencies)
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: modules at repo root respect hidden/special directory exclusion
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_HiddenDirExclusion(t *testing.T) {
	repoDir := t.TempDir()

	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)

	// Create config in hidden dirs that should be excluded
	for _, hidden := range []string{".git/modules", ".dots", "node_modules/pkg"} {
		createLuaModule(t, repoDir, hidden, `return {
      files = { file("x", "~/.x") },
    }`, map[string]string{"x": "content"})
	}

	// Create a real module
	createLuaModule(t, repoDir, "RealMod", `return {
    files = { file("y", "~/.y") },
  }`, map[string]string{"y": "content"})

	// Also create a .hidden dir module (should be excluded too)
	createLuaModule(t, repoDir, ".secret-mod", `return {}`, nil)

	modules, err := luacfg.FindModules(repoDir, &luacfg.RootConfig{Name: "test"})
	require.NoError(t, err)

	require.Len(t, modules, 1, "should find only RealMod")
	assert.Equal(t, "RealMod", modules[0].Name)
}

// ─────────────────────────────────────────────────────────────────────────────
// Verify the config.IsDotfilesRepo detects init.lua repos
// ─────────────────────────────────────────────────────────────────────────────

func TestE2E_IsDotfilesRepoDetection(t *testing.T) {
	repoDir := t.TempDir()

	// Without any marker, should not be detected
	assert.False(t, config.IsDotfilesRepo(repoDir), "empty dir is not a repo")

	// With init.lua, should be detected
	err := os.WriteFile(filepath.Join(repoDir, "init.lua"), []byte(`return { name = "test" }`), 0644)
	require.NoError(t, err)
	assert.True(t, config.IsDotfilesRepo(repoDir), "init.lua should be detected as repo marker")

	// With init.lua + config.lua fallback
	assert.True(t, luacfg.IsLuaRepo(repoDir), "IsLuaRepo should detect init.lua")
}

package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wilberucx/dots/internal/config"
	luacfg "github.com/Wilberucx/dots/internal/lua"
	"github.com/Wilberucx/dots/internal/yaml"
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

// createLuaModule creates a module directory with dots.lua content and source files.
func createLuaModule(t *testing.T, cfg *config.DotsConfig, name, luaContent string, sourceFiles map[string]string) {
	t.Helper()
	modDir := filepath.Join(cfg.RepoRoot, name)
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

// ─── luaFileIsLinked unit tests ─────────────────────────────────────────────

func TestLuaFileIsLinked_FileOpFile_Linked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, ".zshrc"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)
	createSymlink(t, filepath.Join(srcDir, ".zshrc"), filepath.Join(destDir, ".zshrc"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpFile,
		Source:      ".zshrc",
		Destination: filepath.Join(destDir, ".zshrc"),
	}

	assert.True(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_FileOpFile_NotLinked(t *testing.T) {
	srcDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, ".zshrc"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)

	f := luacfg.FileOp{
		Type:        luacfg.FileOpFile,
		Source:      ".zshrc",
		Destination: "/nonexistent/path/.zshrc",
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirTo_Linked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "config"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "config", "file.toml"), []byte("setting=true"), 0644)
	require.NoError(t, err)

	// dir():to(): dest itself is a symlink to the source directory
	createSymlink(t, filepath.Join(srcDir, "config"), filepath.Join(destDir, "config"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirTo,
		Source:      "config",
		Destination: filepath.Join(destDir, "config"),
	}

	assert.True(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirTo_NotLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "config"), 0755)
	require.NoError(t, err)

	// Create real directory at dest, not a symlink
	err = os.MkdirAll(filepath.Join(destDir, "config"), 0755)
	require.NoError(t, err)

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirTo,
		Source:      "config",
		Destination: filepath.Join(destDir, "config"),
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirInto_AllLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "arch"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshrc"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshenv"), []byte("export PATH=$PATH"), 0644)
	require.NoError(t, err)

	// All children correctly symlinked
	createSymlink(t, filepath.Join(srcDir, "arch", ".zshrc"), filepath.Join(destDir, ".zshrc"))
	createSymlink(t, filepath.Join(srcDir, "arch", ".zshenv"), filepath.Join(destDir, ".zshenv"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirInto,
		Source:      "arch",
		Destination: destDir,
	}

	assert.True(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirInto_PartialLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "arch"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshrc"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshenv"), []byte("export PATH=$PATH"), 0644)
	require.NoError(t, err)

	// Only ONE child symlinked
	createSymlink(t, filepath.Join(srcDir, "arch", ".zshrc"), filepath.Join(destDir, ".zshrc"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirInto,
		Source:      "arch",
		Destination: destDir,
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirInto_NoneLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "arch"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshrc"), []byte("export FOO=bar"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "arch", ".zshenv"), []byte("export PATH=$PATH"), 0644)
	require.NoError(t, err)

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirInto,
		Source:      "arch",
		Destination: destDir,
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirInto_EmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.MkdirAll(filepath.Join(srcDir, "arch"), 0755)
	require.NoError(t, err)

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirInto,
		Source:      "arch",
		Destination: destDir,
	}

	// Empty directory is trivially linked
	assert.True(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_DirInto_SourceNotDir(t *testing.T) {
	srcDir := t.TempDir()

	// Source is a file, not a directory
	err := os.WriteFile(filepath.Join(srcDir, "arch"), []byte("not a dir"), 0644)
	require.NoError(t, err)

	f := luacfg.FileOp{
		Type:        luacfg.FileOpDirInto,
		Source:      "arch",
		Destination: t.TempDir(),
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_Glob_AllLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, "alacritty.toml"), []byte("[general]"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "kitty.toml"), []byte("[font]"), 0644)
	require.NoError(t, err)

	createSymlink(t, filepath.Join(srcDir, "alacritty.toml"), filepath.Join(destDir, "alacritty.toml"))
	createSymlink(t, filepath.Join(srcDir, "kitty.toml"), filepath.Join(destDir, "kitty.toml"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpGlob,
		Pattern:     "*.toml",
		Destination: destDir,
	}

	assert.True(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_Glob_PartialLinked(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, "alacritty.toml"), []byte("[general]"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "kitty.toml"), []byte("[font]"), 0644)
	require.NoError(t, err)

	createSymlink(t, filepath.Join(srcDir, "alacritty.toml"), filepath.Join(destDir, "alacritty.toml"))

	f := luacfg.FileOp{
		Type:        luacfg.FileOpGlob,
		Pattern:     "*.toml",
		Destination: destDir,
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

func TestLuaFileIsLinked_Glob_NoMatches(t *testing.T) {
	srcDir := t.TempDir()

	f := luacfg.FileOp{
		Type:        luacfg.FileOpGlob,
		Pattern:     "*.toml",
		Destination: t.TempDir(),
	}

	assert.False(t, luaFileIsLinked(f, srcDir, "linux"))
}

// ─── Integration: GetActiveVariant with DirInto variants ────────────────────

func TestGetActiveVariant_LuaDirIntoVariant_AllLinked(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Zsh", `return {
  type = "minimal",
  files = {
    dir("arch"):into("~"):variant("arch"),
    dir("termux"):into("~"):variant("termux"),
  },
}`, map[string]string{
		"arch/.zshrc":   "export FOO=bar",
		"arch/.zshenv":  "export PATH=$PATH",
		"termux/.zshrc": "export TERMUX=true",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Zsh", Path: filepath.Join(cfg.RepoRoot, "Zsh"), Type: 1},
	})

	// Create all symlinks for "arch" variant
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", "arch", ".zshrc"), filepath.Join(cfg.HomeDir, ".zshrc"))
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", "arch", ".zshenv"), filepath.Join(cfg.HomeDir, ".zshenv"))

	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Equal(t, "arch", active)
}

func TestGetActiveVariant_LuaDirIntoVariant_PartialLinked(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Zsh", `return {
  type = "minimal",
  files = {
    dir("arch"):into("~"):variant("arch"),
    dir("termux"):into("~"):variant("termux"),
  },
}`, map[string]string{
		"arch/.zshrc":   "export FOO=bar",
		"arch/.zshenv":  "export PATH=$PATH",
		"termux/.zshrc": "export TERMUX=true",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Zsh", Path: filepath.Join(cfg.RepoRoot, "Zsh"), Type: 1},
	})

	// Only ONE file linked for "arch"
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", "arch", ".zshrc"), filepath.Join(cfg.HomeDir, ".zshrc"))

	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Empty(t, active)
}

func TestGetActiveVariant_LuaDirIntoVariant_NoLinks(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Zsh", `return {
  type = "minimal",
  files = {
    dir("arch"):into("~"):variant("arch"),
    dir("termux"):into("~"):variant("termux"),
  },
}`, map[string]string{
		"arch/.zshrc":   "export FOO=bar",
		"arch/.zshenv":  "export PATH=$PATH",
		"termux/.zshrc": "export TERMUX=true",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Zsh", Path: filepath.Join(cfg.RepoRoot, "Zsh"), Type: 1},
	})

	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Empty(t, active)
}

func TestGetActiveVariant_LuaDirToVariant_AllLinked(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Alacritty", `return {
  files = {
    dir("personal"):to("~/.config/alacritty"):variant("personal"),
    dir("work"):to("~/.config/alacritty"):variant("work"),
  },
}`, map[string]string{
		"personal/alacritty.toml": "colorscheme = \"catppuccin\"",
		"work/alacritty.toml":     "colorscheme = \"solarized\"",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Alacritty", Path: filepath.Join(cfg.RepoRoot, "Alacritty"), Type: 1},
	})

	// Symlink the entire directory for "personal"
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Alacritty", "personal"), filepath.Join(cfg.HomeDir, ".config", "alacritty"))

	active, err := GetActiveVariant(cfg, "Alacritty")
	require.NoError(t, err)
	assert.Equal(t, "personal", active)
}

func TestGetActiveVariant_LuaGlobVariant_AllLinked(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Configs", `return {
  files = {
    glob("*.toml"):into("~/.config/"):variant("work"),
  },
}`, map[string]string{
		"alacritty.toml": "[general]",
		"kitty.toml":     "[font]",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Configs", Path: filepath.Join(cfg.RepoRoot, "Configs"), Type: 1},
	})

	// Create all symlinks for the glob matches
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Configs", "alacritty.toml"), filepath.Join(cfg.HomeDir, ".config", "alacritty.toml"))
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Configs", "kitty.toml"), filepath.Join(cfg.HomeDir, ".config", "kitty.toml"))

	active, err := GetActiveVariant(cfg, "Configs")
	require.NoError(t, err)
	assert.Equal(t, "work", active)
}

// ─── Integration: ResolveModules with DirInto variants ──────────────────────

func TestResolveModules_LuaDirIntoVariant_AllLinked_Default(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Zsh", `return {
  type = "minimal",
  files = {
    dir("arch"):into("~"):variant("arch"),
    dir("termux"):into("~"):variant("termux"),
  },
}`, map[string]string{
		"arch/.zshrc":   "export FOO=bar",
		"arch/.zshenv":  "export PATH=$PATH",
		"termux/.zshrc": "export TERMUX=true",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Zsh", Path: filepath.Join(cfg.RepoRoot, "Zsh"), Type: 1},
	})

	// Create all symlinks for "arch" (the default variant = last declared)
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", "arch", ".zshrc"), filepath.Join(cfg.HomeDir, ".zshrc"))
	createSymlink(t, filepath.Join(cfg.RepoRoot, "Zsh", "arch", ".zshenv"), filepath.Join(cfg.HomeDir, ".zshenv"))

	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	require.Len(t, results["Zsh"], 2)

	for _, st := range results["Zsh"] {
		assert.Equal(t, StateLinked, st.State)
	}

	// Verify active variant is detected
	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Equal(t, "arch", active)
}

func TestResolveModules_LuaDirIntoVariant_NoneLinked_ShowsVariants(t *testing.T) {
	cfg := setupTestRepo(t)
	cfg.IsLuaRepo = true

	createLuaModule(t, cfg, "Zsh", `return {
  type = "minimal",
  files = {
    dir("arch"):into("~"):variant("arch"),
    dir("termux"):into("~"):variant("termux"),
  },
}`, map[string]string{
		"arch/.zshrc":    "export FOO=bar",
		"arch/.zshenv":   "export PATH=$PATH",
		"termux/.zshrc":  "export TERMUX=true",
		"termux/.zshenv": "export TERMUX_PATH=$PATH",
	})

	cfg.SetCachedModuleDirs([]config.ModuleDir{
		{Name: "Zsh", Path: filepath.Join(cfg.RepoRoot, "Zsh"), Type: 1},
	})

	// No symlinks created — none linked
	results, err := ResolveModules(cfg, nil, nil, "")
	require.NoError(t, err)
	require.Contains(t, results, "Zsh")
	// Should resolve with default variant "termux" (last declared) and show pending
	require.Len(t, results["Zsh"], 2)
	for _, st := range results["Zsh"] {
		assert.Equal(t, StatePending, st.State)
	}

	// No active variant detected (none linked)
	active, err := GetActiveVariant(cfg, "Zsh")
	require.NoError(t, err)
	assert.Empty(t, active)
}

// ─── OLD TESTS (below) ──────────────────────────────────────────────────────

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
	assert.Contains(t, info.Variants, "wilber")
	assert.Contains(t, info.Variants, "canto")
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
	results, err := ResolveModules(cfg, []string{"Hypr"}, nil, "wilber")
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

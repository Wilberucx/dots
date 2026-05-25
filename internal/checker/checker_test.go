package checker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wilberucx/dots/internal/config"
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

	// Create a home dir
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

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

func TestRunSyntaxCheck_NoModules(t *testing.T) {
	cfg := setupTestRepo(t)
	result := RunSyntaxCheck(cfg)
	assert.Empty(t, result.Issues)
}

func TestRunSyntaxCheck_ValidModule(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Nvim", `
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
dependencies:
  - name: neovim
    type: package
`, map[string]string{"init.lua": "vim.cmd('colorscheme slate')"})

	result := RunSyntaxCheck(cfg)
	assert.Empty(t, result.Issues)
}

func TestRunSyntaxCheck_V2DepFields(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
dependencies:
  - name: fd
    type: binary
    source: https://example.com/fd.tar.gz
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Field == "source" {
			found = true
			assert.Contains(t, issue.Message, "v2 field")
			assert.Contains(t, issue.Message, "source")
			assert.Contains(t, issue.Message, "url")
			break
		}
	}
	assert.True(t, found, "expected v2 field error for 'source'")
}

func TestRunSyntaxCheck_V2FileFields(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
    destination-linux: ~/.config/config.conf
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Field == "destination-linux" {
			found = true
			assert.Contains(t, issue.Message, "v2 field")
			assert.Contains(t, issue.Message, "destination-linux")
			assert.Contains(t, issue.Message, "per-os")
			break
		}
	}
	assert.True(t, found, "expected v2 field error for 'destination-linux'")
}

func TestRunSyntaxCheck_MissingSourceFile(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Nvim", `
files:
  - source: nonexistent.lua
    destination: ~/.config/nvim/init.lua
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && issue.Field == "" {
			if assert.Contains(t, issue.Message, "source path not found") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected warning for missing source file")
}

func TestRunSyntaxCheck_MissingRequiredField(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
dependencies:
  - name: fd
    type: binary
    # missing url and dest
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	urlFound := false
	destFound := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			if issue.Field == "url" {
				urlFound = true
			}
			if issue.Field == "dest" {
				destFound = true
			}
		}
	}
	assert.True(t, urlFound, "expected error for missing 'url'")
	assert.True(t, destFound, "expected error for missing 'dest'")
}

func TestRunSyntaxCheck_UnknownDepType(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
dependencies:
  - name: foo
    type: invalid-type
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Field == "type" {
			found = true
			assert.Contains(t, issue.Message, "unknown type")
			break
		}
	}
	assert.True(t, found, "expected error for unknown type")
}

func TestRunSyntaxCheck_NoDestination(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Message != "" {
			if assert.Contains(t, issue.Message, "nowhere to link") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected error for missing destination")
}

func TestRunSyntaxCheck_PerOSValid(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Alacritty", `
files:
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty/alacritty.yml
      mac: ~/Library/Application Support/alacritty/alacritty.yml
`, map[string]string{"alacritty.yml": "colors: *default"})

	result := RunSyntaxCheck(cfg)
	assert.Empty(t, result.Issues)
}

func TestRunSyntaxCheck_InvalidPerOSKey(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
    per-os:
      linux: ~/.config/linux.conf
      solaris: ~/.config/solaris.conf
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && issue.Message != "" {
			if assert.Contains(t, issue.Message, "unknown OS") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected warning for unknown OS in per-os")
}

func TestRunSyntaxCheck_InvalidOSFilter(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
    destination: ~/.config/config.conf
    os:
      - linux
      - solaris
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && issue.Message != "" {
			if assert.Contains(t, issue.Message, "unknown OS") {
				assert.Contains(t, issue.Message, "solaris")
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected warning for unknown OS in filter")
}

func TestRunSyntaxCheck_DuplicateSource(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
    destination: ~/.config/config1.conf
  - source: config.conf
    destination: ~/.config/config2.conf
`, map[string]string{"config.conf": "some content"})

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && strings.Contains(issue.Message, "duplicate source") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning for duplicate source")
}

func TestRunSyntaxCheck_UnknownField(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
files:
  - source: config.conf
    destination: ~/.config/config.conf
    typo_field: true
dependencies:
  - name: test
    type: package
    typo_field: true
`, map[string]string{"config.conf": "some content"})

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	warnCount := 0
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && strings.Contains(issue.Message, "unknown field") {
			warnCount++
		}
	}
	assert.Equal(t, 2, warnCount, "expected 2 unknown field warnings")
}

func TestRunSyntaxCheck_EmptyFile(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Empty", ``, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && issue.Message != "" {
			if assert.Contains(t, issue.Message, "empty") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected warning for empty file")
}

func TestRunSyntaxCheck_NoSections(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Empty", `
type: module
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning && issue.Message != "" {
			if assert.Contains(t, issue.Message, "no 'dependencies' or 'files'") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected warning for no sections")
}

func TestRunSyntaxCheck_InvalidYAML(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Broken", `invalid: [yaml: broken`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			if assert.Contains(t, issue.Message, "invalid YAML") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected error for invalid YAML")
}

func TestRunSyntaxCheck_MultipleModules(t *testing.T) {
	cfg := setupTestRepo(t)

	// Valid module
	createModule(t, cfg, "Nvim", `
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
dependencies:
  - name: neovim
    type: package
`, map[string]string{"init.lua": "vim.cmd('colorscheme slate')"})

	// Module with v2 deps
	createModule(t, cfg, "Fd", `
dependencies:
  - name: fd
    type: binary
    source: https://example.com/fd.tar.gz
    target: ~/.local/bin/fd
`, nil)

	// Empty module
	createModule(t, cfg, "Empty", ``, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	// Should have issues for Fd (v2 fields) and Empty (empty file)
	moduleSet := make(map[string]bool)
	for _, issue := range result.Issues {
		moduleSet[issue.Module] = true
	}

	assert.True(t, moduleSet["Fd"], "expected issues for Fd module")
	assert.True(t, moduleSet["Empty"], "expected issues for Empty module")
	assert.False(t, moduleSet["Nvim"], "expected no issues for Nvim module")
}

func TestRunSyntaxCheck_GlobSourceSkipped(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Scripts", `
files:
  - source: scripts/*
    destination: ~/scripts
`, nil)

	result := RunSyntaxCheck(cfg)
	// Glob sources should not trigger "source not found" warning
	for _, issue := range result.Issues {
		if issue.Severity == SeverityWarning {
			assert.NotContains(t, issue.Message, "source path not found",
				"glob sources should not trigger source-not-found warnings")
		}
	}
}

func TestRunSyntaxCheck_ArchNotDict(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
dependencies:
  - name: fd
    type: binary
    url: https://example.com/fd.tar.gz
    dest: ~/.local/bin/fd
    arch: x86_64
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Field == "arch" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error for 'arch' not being a dict")
}

func TestRunSyntaxCheck_ManagersNotDict(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Test", `
dependencies:
  - name: neovim
    type: package
    managers: pacman
`, nil)

	result := RunSyntaxCheck(cfg)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && issue.Field == "managers" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error for 'managers' not being a dict")
}

func TestHasErrors(t *testing.T) {
	r := &Result{Issues: []Issue{
		{Severity: SeverityWarning, Message: "warning"},
	}}
	assert.False(t, r.HasErrors())

	r.Issues = append(r.Issues, Issue{Severity: SeverityError, Message: "error"})
	assert.True(t, r.HasErrors())
}

func TestHasWarnings(t *testing.T) {
	r := &Result{Issues: []Issue{
		{Severity: SeverityError, Message: "error"},
	}}
	assert.False(t, r.HasWarnings())

	r.Issues = append(r.Issues, Issue{Severity: SeverityWarning, Message: "warning"})
	assert.True(t, r.HasWarnings())
}

func TestFormatIssue(t *testing.T) {
	issue := Issue{
		Module:   "Test",
		File:     "Test/path.yaml",
		Severity: SeverityError,
		Message:  "something is wrong",
	}
	formatted := FormatIssue(issue)
	assert.Contains(t, formatted, "✘")
	assert.Contains(t, formatted, "Test/path.yaml")
	assert.Contains(t, formatted, "something is wrong")

	issue.Severity = SeverityWarning
	formatted = FormatIssue(issue)
	assert.Contains(t, formatted, "⚠")
}

func TestRunSyntaxCheck_BrokenLink(t *testing.T) {
	cfg := setupTestRepo(t)
	createModule(t, cfg, "Nvim", `
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
`, map[string]string{"init.lua": "vim.cmd('colorscheme slate')"})

	// Create a conflicting symlink at the destination
	destPath := filepath.Join(cfg.HomeDir, ".config", "nvim", "init.lua")
	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	require.NoError(t, err)
	err = os.Symlink("/some/wrong/target", destPath)
	require.NoError(t, err)

	result := RunSyntaxCheck(cfg)
	CheckBrokenLinks(cfg, result)
	require.NotEmpty(t, result.Issues)

	found := false
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError && strings.Contains(issue.Message, "broken link") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected broken link error in issues")
}


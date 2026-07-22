package writer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDestinationStr(t *testing.T) {
	home := "/home/user"

	tests := []struct {
		name    string
		absPath string
		homeDir string
		want    string
	}{
		{"inside home", "/home/user/.zshrc", home, "~/.zshrc"},
		{"nested inside home", "/home/user/.config/nvim/init.lua", home, "~/.config/nvim/init.lua"},
		{"home itself", home, home, "~"},
		{"outside home", "/etc/passwd", home, filepath.Join("..", "etc", "passwd")},
		{"sibling path", "/home/other/file", home, "other/file"},
		{"empty home", "/tmp/file", "", "/tmp/file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DestinationStr(tt.absPath, tt.homeDir)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadModuleData(t *testing.T) {
	t.Run("file doesn't exist", func(t *testing.T) {
		data := LoadModuleData("/nonexistent/path.yaml")
		assert.Equal(t, map[string]interface{}{"files": []interface{}{}}, data)
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")
		require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

		data := LoadModuleData(path)
		// Empty YAML unmarshals to nil, so we get empty files
		assert.Equal(t, map[string]interface{}{"files": []interface{}{}}, data)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")
		require.NoError(t, os.WriteFile(path, []byte("invalid: [yaml: broken"), 0o644))

		data := LoadModuleData(path)
		assert.Equal(t, map[string]interface{}{"files": []interface{}{}}, data)
	})

	t.Run("valid yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")
		content := `files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		data := LoadModuleData(path)
		files, ok := data["files"].([]interface{})
		require.True(t, ok)
		require.Len(t, files, 1)

		entry, ok := files[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "init.lua", entry["source"])
		assert.Equal(t, "~/.config/nvim/init.lua", entry["destination"])
	})

	t.Run("with type metadata", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")
		content := `type: shell
files:
  - source: script.sh
    destination: ~/bin/script.sh
`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		data := LoadModuleData(path)
		assert.Equal(t, "shell", data["type"])
	})
}

func TestIsDestinationDeclared(t *testing.T) {
	data := map[string]interface{}{
		"files": []interface{}{
			map[string]interface{}{
				"source":      "init.lua",
				"destination": "~/.config/nvim/init.lua",
			},
			map[string]interface{}{
				"source": "alacritty.yml",
				"per-os": map[string]interface{}{
					"linux": "~/.config/alacritty/alacritty.yml",
					"mac":   "~/Library/alacritty.yml",
				},
			},
			map[string]interface{}{
				"source": "other",
				// no destination at all
			},
		},
	}

	t.Run("found in destination", func(t *testing.T) {
		assert.True(t, IsDestinationDeclared(data, "~/.config/nvim/init.lua"))
	})

	t.Run("found in per-os", func(t *testing.T) {
		assert.True(t, IsDestinationDeclared(data, "~/.config/alacritty/alacritty.yml"))
		assert.True(t, IsDestinationDeclared(data, "~/Library/alacritty.yml"))
	})

	t.Run("not found", func(t *testing.T) {
		assert.False(t, IsDestinationDeclared(data, "~/.config/nonexistent"))
	})

	t.Run("empty data", func(t *testing.T) {
		assert.False(t, IsDestinationDeclared(nil, "~/.config/file"))
		assert.False(t, IsDestinationDeclared(map[string]interface{}{}, "~/.config/file"))
	})

	t.Run("files is not a slice", func(t *testing.T) {
		data := map[string]interface{}{"files": "invalid"}
		assert.False(t, IsDestinationDeclared(data, "~/.config/file"))
	})

	t.Run("entry is not a map", func(t *testing.T) {
		data := map[string]interface{}{
			"files": []interface{}{"string entry"},
		}
		assert.False(t, IsDestinationDeclared(data, "~/.config/file"))
	})

	t.Run("per-os is not a map", func(t *testing.T) {
		data := map[string]interface{}{
			"files": []interface{}{
				map[string]interface{}{
					"per-os": "invalid_string",
				},
			},
		}
		assert.False(t, IsDestinationDeclared(data, "~/.config/file"))
	})
}

func TestAppendFileEntry(t *testing.T) {
	t.Run("new file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")

		entry := map[string]interface{}{
			"source":      "init.lua",
			"destination": "~/.config/nvim/init.lua",
		}
		err := AppendFileEntry(path, entry)
		require.NoError(t, err)

		// Verify the file was created
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = yaml.Unmarshal(data, &parsed)
		require.NoError(t, err)

		files, ok := parsed["files"].([]interface{})
		require.True(t, ok)
		require.Len(t, files, 1)
	})

	t.Run("append to existing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")

		// First entry
		err := AppendFileEntry(path, map[string]interface{}{
			"source":      "first.lua",
			"destination": "~/.config/first.lua",
		})
		require.NoError(t, err)

		// Second entry
		err = AppendFileEntry(path, map[string]interface{}{
			"source":      "second.lua",
			"destination": "~/.config/second.lua",
		})
		require.NoError(t, err)

		// Verify both entries exist
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = yaml.Unmarshal(data, &parsed)
		require.NoError(t, err)

		files, ok := parsed["files"].([]interface{})
		require.True(t, ok)
		assert.Len(t, files, 2)
	})

	t.Run("corrupt file is overwritten", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "path.yaml")

		// Write corrupt content first
		require.NoError(t, os.WriteFile(path, []byte("invalid: [yaml: broken"), 0o644))

		err := AppendFileEntry(path, map[string]interface{}{
			"source":      "config.conf",
			"destination": "~/.config/config.conf",
		})
		require.NoError(t, err)

		// Should have created a valid yaml file
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = yaml.Unmarshal(data, &parsed)
		require.NoError(t, err)

		files, ok := parsed["files"].([]interface{})
		require.True(t, ok)
		assert.Len(t, files, 1)
	})
}

func TestWriteConfigYAML(t *testing.T) {
	t.Run("creates file and dirs", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".dots", "config.yaml")

		err := WriteConfigYAML(path, "version: 1\nrepo: test\n")
		require.NoError(t, err)

		// Verify file exists and has content
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "version: 1")
		assert.Contains(t, string(data), "repo: test")
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".dots", "config.yaml")

		// Write initial content
		err := WriteConfigYAML(path, "version: 1\n")
		require.NoError(t, err)

		// Overwrite
		err = WriteConfigYAML(path, "version: 2\nrepo: updated\n")
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "version: 2")
		assert.Contains(t, string(data), "repo: updated")
	})
}

func TestAppendToFile(t *testing.T) {
	t.Run("creates new file and appends", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")

		err := AppendToFile(path, "line 1\n")
		require.NoError(t, err)

		err = AppendToFile(path, "line 2\n")
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "line 1\nline 2\n", string(data))
	})

	t.Run("empty content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")

		err := AppendToFile(path, "")
		require.NoError(t, err)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Empty(t, data)
	})
}

func TestDetectShell(t *testing.T) {
	t.Run("zsh from SHELL env", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/zsh")
		t.Setenv("HOME", "/home/user")

		shell, cfg := DetectShell()
		assert.Equal(t, "zsh", shell)
		assert.Equal(t, "/home/user/.zshrc", cfg)
	})

	t.Run("bash from SHELL env", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/bash")
		t.Setenv("HOME", "/home/user")

		shell, cfg := DetectShell()
		assert.Equal(t, "bash", shell)
		assert.Equal(t, "/home/user/.bashrc", cfg)
	})

	t.Run("fish from SHELL env", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/fish")
		t.Setenv("HOME", "/home/user")

		shell, cfg := DetectShell()
		assert.Equal(t, "fish", shell)
		assert.Equal(t, "/home/user/.config/fish/config.fish", cfg)
	})

	t.Run("unknown SHELL falls back to zshrc", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/tcsh")
		t.Setenv("HOME", "/tmp")

		shell, cfg := DetectShell()
		assert.Equal(t, "unknown", shell)
		assert.Equal(t, "/tmp/.zshrc", cfg)
	})

	t.Run("empty SHELL falls back", func(t *testing.T) {
		t.Setenv("SHELL", "")
		home := t.TempDir()
		t.Setenv("HOME", home)

		// Create .bashrc so it's detected
		err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte(""), 0o644)
		require.NoError(t, err)

		shell, cfg := DetectShell()
		assert.Equal(t, "bash", shell)
		assert.Equal(t, filepath.Join(home, ".bashrc"), cfg)
	})

	t.Run("fallback: zshrc exists", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/unknown")
		home := t.TempDir()
		t.Setenv("HOME", home)

		err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(""), 0o644)
		require.NoError(t, err)

		shell, cfg := DetectShell()
		assert.Equal(t, "zsh", shell)
		assert.Equal(t, filepath.Join(home, ".zshrc"), cfg)
	})

	t.Run("fallback: fish config exists", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/unknown")
		home := t.TempDir()
		t.Setenv("HOME", home)

		fishDir := filepath.Join(home, ".config", "fish")
		err := os.MkdirAll(fishDir, 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(fishDir, "config.fish"), []byte(""), 0o644)
		require.NoError(t, err)

		shell, cfg := DetectShell()
		assert.Equal(t, "fish", shell)
		assert.Equal(t, filepath.Join(fishDir, "config.fish"), cfg)
	})
}

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectOS(t *testing.T) {
	os := DetectOS()
	assert.Contains(t, []string{"linux", "mac", "windows", "unknown"}, os)
}

func TestHomeDir(t *testing.T) {
	home := HomeDir()
	assert.NotEmpty(t, home)
	assert.DirExists(t, home)
}

func TestIsSafePath(t *testing.T) {
	home := HomeDir()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"home directory", home, false},
		{"inside home", filepath.Join(home, ".zshrc"), true},
		{"subdir inside home", filepath.Join(home, ".config", "nvim"), true},
		{"outside home", "/etc/passwd", false},
		{"root", "/", false},
		{"tmp", "/tmp", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSafePath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandPath(t *testing.T) {
	home := HomeDir()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tilde home", "~/.zshrc", filepath.Join(home, ".zshrc")},
		{"tilde alone", "~", home},
		{"no expansion", "/etc/passwd", "/etc/passwd"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandPath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandPathWithEnv(t *testing.T) {
	os.Setenv("DOTS_TEST_ENV", "/custom/path")
	defer os.Unsetenv("DOTS_TEST_ENV")

	got := ExpandPath("$DOTS_TEST_ENV/file")
	assert.Equal(t, "/custom/path/file", got)
}

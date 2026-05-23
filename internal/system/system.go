package system

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectOS returns the current operating system: "linux", "mac", "windows", or "unknown".
func DetectOS() string {
	switch runtime.GOOS {
	case "linux":
		return "linux"
	case "darwin":
		return "mac"
	case "windows":
		return "windows"
	default:
		return "unknown"
	}
}

// IsSafePath checks if a path is safe to modify (inside HOME and not HOME itself).
func IsSafePath(path string) bool {
	home := getHomeDir()
	if home == "" {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absHome, err := filepath.Abs(home)
	if err != nil {
		return false
	}

	// Must be inside home
	if !strings.HasPrefix(absPath, absHome) {
		return false
	}

	// Must not be home itself
	if absPath == absHome {
		return false
	}

	return true
}

// ExpandPath expands ~ and environment variables in a path.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err == nil {
			path = usr.HomeDir + path[1:]
		}
	}
	return os.ExpandEnv(path)
}

// getHomeDir returns the user's home directory.
// It checks $HOME first for testability, then falls back to user.Current().
func getHomeDir() string {
	// $HOME is checked first so tests can override it with t.Setenv
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	usr, err := user.Current()
	if err == nil {
		return usr.HomeDir
	}
	return "/"
}

// HomeDir returns the user's home directory.
func HomeDir() string {
	return getHomeDir()
}

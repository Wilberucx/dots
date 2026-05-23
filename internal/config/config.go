package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/system"

	"gopkg.in/yaml.v3"
)

const (
	// MarkerDir is the directory used as a repo marker.
	MarkerDir = ".dots"
	// MarkerConfig is the config file inside MarkerDir.
	MarkerConfig = "config.yaml"
	// LegacyMarker is the legacy marker file (dots.toml).
	LegacyMarker = "dots.toml"
)

// DotsConfig represents immutable runtime configuration.
type DotsConfig struct {
	RepoRoot  string
	CurrentOS string
	HomeDir   string
	CLIDir    string
}

// IsDotfilesRepo checks if a path is a dotfiles repository.
func IsDotfilesRepo(path string) bool {
	// New format: .dots/config.yaml
	if _, err := os.Stat(filepath.Join(path, MarkerDir, MarkerConfig)); err == nil {
		return true
	}
	// Legacy format: dots.toml
	if _, err := os.Stat(filepath.Join(path, LegacyMarker)); err == nil {
		return true
	}
	return false
}

// Load finds the dotfiles repository and returns a DotsConfig.
// Search order:
//  1. DOTS_REPO environment variable (override)
//  2. Walk up from current working directory
//  3. Common locations in user's home (~/Dot.files, ~/.dotfiles, ~/dotfiles)
func Load() (*DotsConfig, error) {
	home := system.HomeDir()

	// 1. Environment variable override
	if repo := os.Getenv("DOTS_REPO"); repo != "" {
		absRepo, err := filepath.Abs(repo)
		if err != nil {
			return nil, fmt.Errorf("invalid DOTS_REPO path: %w", err)
		}
		if !IsDotfilesRepo(absRepo) {
			return nil, fmt.Errorf("DOTS_REPO %q is not a dotfiles repository", absRepo)
		}
		return create(absRepo), nil
	}

	// 2. Walk up from CWD
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for {
			if IsDotfilesRepo(dir) {
				return create(dir), nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 3. Common fallback locations
	for _, name := range []string{"Dot.files", ".dotfiles", "dotfiles"} {
		potential := filepath.Join(home, name)
		if IsDotfilesRepo(potential) {
			return create(potential), nil
		}
	}

	return nil, fmt.Errorf(
		"could not find a dotfiles repository.\n"+
			"No '%s/%s' or '%s' found in current directory tree or common locations.\n\n"+
			"To fix this, you have 3 options:\n"+
			"  1. dots --path ~/your-dotfiles <command> — specify path directly\n"+
			"  2. export DOTS_REPO=~/your-dotfiles — set environment variable\n"+
			"  3. cd ~/your-dotfiles && dots init — initialize if not done yet",
		MarkerDir, MarkerConfig, LegacyMarker,
	)
}

func create(repoRoot string) *DotsConfig {
	return &DotsConfig{
		RepoRoot:  repoRoot,
		CurrentOS: system.DetectOS(),
		HomeDir:   system.HomeDir(),
		CLIDir:    filepath.Join(repoRoot, "cli"),
	}
}

// GetModuleDirs returns sorted module directories, optionally filtered by name or type.
func (c *DotsConfig) GetModuleDirs(modules, types []string) ([]ModuleDir, error) {
	entries, err := os.ReadDir(c.RepoRoot)
	if err != nil {
		return nil, err
	}

	var allDirs []ModuleDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pathYAML := filepath.Join(c.RepoRoot, entry.Name(), "path.yaml")
		if _, err := os.Stat(pathYAML); os.IsNotExist(err) {
			continue
		}
		allDirs = append(allDirs, ModuleDir{
			Name: entry.Name(),
			Path: filepath.Join(c.RepoRoot, entry.Name()),
		})
	}

	if len(modules) > 0 {
		dirMap := make(map[string]ModuleDir)
		for _, d := range allDirs {
			dirMap[d.Name] = d
		}
		var filtered []ModuleDir
		for _, name := range modules {
			if d, ok := dirMap[name]; ok {
				filtered = append(filtered, d)
			}
		}
		allDirs = filtered
	}

	if len(types) > 0 {
		var filtered []ModuleDir
		for _, d := range allDirs {
			meta, err := ParseModuleMeta(filepath.Join(d.Path, "path.yaml"))
			if err != nil {
				continue
			}
			moduleType, _ := meta["type"].(string)
			for _, t := range types {
				if moduleType == t {
					filtered = append(filtered, d)
					break
				}
			}
		}
		allDirs = filtered
	}

	return allDirs, nil
}

// ModuleDir represents a module directory in the dotfiles repo.
type ModuleDir struct {
	Name string
	Path string
}

// ParseModuleMeta reads type metadata from a path.yaml file.
func ParseModuleMeta(yamlPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return make(map[string]interface{}), nil
	}

	meta := make(map[string]interface{})
	if t, ok := raw["type"].(string); ok {
		meta["type"] = t
	}
	return meta, nil
}

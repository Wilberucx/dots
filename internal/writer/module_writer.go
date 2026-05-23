package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DestinationStr returns the destination string in ~/... format for path.yaml.
func DestinationStr(absPath, homeDir string) string {
	rel, err := filepath.Rel(homeDir, absPath)
	if err != nil {
		return absPath
	}
	return filepath.Join("~", rel)
}

// LoadModuleData reads a path.yaml file and returns the raw data.
// Returns an empty structure if the file doesn't exist or is corrupt.
func LoadModuleData(yamlPath string) map[string]interface{} {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"files": []interface{}{}}
		}
		return map[string]interface{}{"files": []interface{}{}}
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return map[string]interface{}{"files": []interface{}{}}
	}
	if parsed == nil {
		return map[string]interface{}{"files": []interface{}{}}
	}
	return parsed
}

// IsDestinationDeclared returns true if the destination is already declared in the files list.
// Compatible with v3 schema (per-os) and generic destination.
func IsDestinationDeclared(data map[string]interface{}, destination string) bool {
	filesRaw, ok := data["files"].([]interface{})
	if !ok {
		return false
	}

	for _, f := range filesRaw {
		entry, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		// Schema v3: generic destination
		if entry["destination"] == destination {
			return true
		}

		// Schema v3: per-os destinations
		if perOS, ok := entry["per-os"].(map[string]interface{}); ok {
			for _, v := range perOS {
				if v == destination {
					return true
				}
			}
		}
	}

	return false
}

// AppendFileEntry appends an entry to the files list in a path.yaml file.
// Creates the file if it doesn't exist.
func AppendFileEntry(yamlPath string, entry map[string]interface{}) error {
	data := LoadModuleData(yamlPath)

	// Ensure files is a slice
	filesRaw, ok := data["files"].([]interface{})
	if !ok {
		filesRaw = []interface{}{}
	}

	filesRaw = append(filesRaw, entry)
	data["files"] = filesRaw

	// Marshal with sort_keys=false, similar to Python's yaml.dump
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling yaml: %w", err)
	}

	if err := os.WriteFile(yamlPath, out, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", yamlPath, err)
	}

	return nil
}

// WriteConfigYAML creates or overwrites a .dots/config.yaml marker file with the given content.
func WriteConfigYAML(path, content string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// AppendToFile appends a line to a file, creating it if it doesn't exist.
func AppendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("writing to %s: %w", path, err)
	}
	return nil
}

// DetectShell detects the user's shell and its config file.
func DetectShell() (string, string) {
	shell := os.Getenv("SHELL")
	home := os.Getenv("HOME")

	switch {
	case strings.Contains(shell, "zsh"):
		return "zsh", filepath.Join(home, ".zshrc")
	case strings.Contains(shell, "bash"):
		return "bash", filepath.Join(home, ".bashrc")
	case strings.Contains(shell, "fish"):
		return "fish", filepath.Join(home, ".config", "fish", "config.fish")
	}

	// Fallback: check which config files exist
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
		return "zsh", filepath.Join(home, ".zshrc")
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		return "bash", filepath.Join(home, ".bashrc")
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "fish", "config.fish")); err == nil {
		return "fish", filepath.Join(home, ".config", "fish", "config.fish")
	}

	return "unknown", filepath.Join(home, ".zshrc")
}

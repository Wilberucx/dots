package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Wilberucx/dots/internal/ui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// v2→v3 field mappings for dependencies.
var depFieldMap = map[string]string{
	"source":           "url",
	"target":           "dest",
	"extract-path":     "extract",
	"arch_map":         "arch",
	"package-managers": "managers",
}

// v2 file fields to detect (destination-linux/mac → per-os, destination-override → per-os).
var fileFieldMap = map[string]bool{
	"destination-linux":    true,
	"destination-mac":      true,
	"destination-override": true,
}

func init() {
	migrateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMigrate(cmd)
	}
}

// ─── CLI entry point ────────────────────────────────────────────────────────

func runMigrate(cmd *cobra.Command) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	repoRoot := cfg.RepoRoot
	ui.PrintHeader("Migrating path.yaml")
	fmt.Printf("  Scanning %s for path.yaml files...\n", repoRoot)

	pathYAMLFiles := findPathYAMLFiles(repoRoot)

	if len(pathYAMLFiles) == 0 {
		ui.PrintInfo("No path.yaml files found.")
		return nil
	}
	fmt.Printf("  Found %d path.yaml file(s).\n\n", len(pathYAMLFiles))

	// Preview phase
	var filesToMigrate []string
	for _, fp := range pathYAMLFiles {
		rel, _ := filepath.Rel(repoRoot, fp)
		modified, _ := migrateFile(fp, true) // dry-run = true
		if modified {
			fmt.Printf("  → %s (needs migration)\n", rel)
			filesToMigrate = append(filesToMigrate, fp)
		} else {
			fmt.Printf("    %s (already v3)\n", rel)
		}
	}

	if len(filesToMigrate) == 0 {
		fmt.Println()
		ui.PrintSuccess("All files already at v3 format.")
		return nil
	}

	if dryRun {
		fmt.Println()
		ui.PrintWarning(fmt.Sprintf("--dry-run: no files were modified."))
		return nil
	}

	// Confirmation prompt
	if !yes {
		fmt.Println()
		ui.PrintInfo(fmt.Sprintf("Migrate %d file(s)?", len(filesToMigrate)))
		confirmed := ui.RunConfirm("Proceed with migration?", true)
		if !confirmed {
			ui.PrintInfo("Aborted.")
			return nil
		}
	}

	// Apply migration
	fmt.Println()
	for _, fp := range filesToMigrate {
		rel, _ := filepath.Rel(repoRoot, fp)
		modified, err := migrateFile(fp, false) // dry-run = false
		if err != nil {
			ui.PrintWarning(fmt.Sprintf("  ✗ %s: %v", rel, err))
		} else {
			fmt.Printf("  ✓ %s\n", rel)
		}
		_ = modified // we know it was modified because preview found it
	}

	fmt.Println()
	ui.PrintSuccess(fmt.Sprintf("Migration complete: %d file(s) updated.", len(filesToMigrate)))
	return nil
}

// ─── File discovery ─────────────────────────────────────────────────────────

// findPathYAMLFiles recursively finds all path.yaml files in repoRoot.
func findPathYAMLFiles(repoRoot string) []string {
	var files []string
	filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if info.IsDir() {
			// Skip .git, .dots, and node_modules to avoid scanning irrelevant dirs
			if info.Name() == ".git" || info.Name() == ".dots" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "path.yaml" {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// ─── File migration ─────────────────────────────────────────────────────────

// migrateFile migrates a single path.yaml from v2 to v3.
// Returns true if the file was modified (or would be modified in dry-run).
// Returns an error only for write failures (not for dry-run or no-op).
func migrateFile(filePath string, dryRun bool) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, nil
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, nil
	}

	if raw == nil {
		return false, nil
	}

	modified := false

	// Migrate dependencies
	if deps, ok := raw["dependencies"].([]interface{}); ok {
		var newDeps []interface{}
		for _, d := range deps {
			if depMap, ok := d.(map[string]interface{}); ok {
				migrated := migrateDependency(depMap)
				if !reflect.DeepEqual(depMap, migrated) {
					modified = true
				}
				newDeps = append(newDeps, migrated)
			} else {
				newDeps = append(newDeps, d)
			}
		}
		if modified {
			raw["dependencies"] = newDeps
		}
	}

	// Migrate files
	if files, ok := raw["files"].([]interface{}); ok {
		var newFiles []interface{}
		for _, f := range files {
			if fMap, ok := f.(map[string]interface{}); ok {
				migrated := migrateFileEntry(fMap)
				if !reflect.DeepEqual(fMap, migrated) {
					modified = true
				}
				newFiles = append(newFiles, migrated)
			} else {
				newFiles = append(newFiles, f)
			}
		}
		if modified {
			raw["files"] = newFiles
		}
	}

	if !modified {
		return false, nil
	}

	if dryRun {
		return true, nil
	}

	// Write back
	out, err := yaml.Marshal(raw)
	if err != nil {
		return false, nil
	}

	if err := os.WriteFile(filePath, out, 0644); err != nil {
		return false, fmt.Errorf("writing %s: %w", filePath, err)
	}

	return true, nil
}

// ─── Dependency migration ───────────────────────────────────────────────────

// migrateDependency migrates a single dependency from v2 to v3.
func migrateDependency(dep map[string]interface{}) map[string]interface{} {
	result := copyMap(dep)

	// Rename fields: v2 → v3
	for oldField, newField := range depFieldMap {
		if v, exists := result[oldField]; exists {
			if _, existsV3 := result[newField]; !existsV3 {
				result[newField] = v
			}
			delete(result, oldField)
		}
	}

	// type: system → type: package
	if t, ok := result["type"].(string); ok && t == "system" {
		result["type"] = "package"
	}

	return result
}

// ─── File entry migration ───────────────────────────────────────────────────

// migrateFileEntry migrates a single file entry from v2 to v3.
func migrateFileEntry(entry map[string]interface{}) map[string]interface{} {
	result := copyMap(entry)

	// Migrate destination-linux / destination-mac → per-os
	destLinux := popField(result, "destination-linux")
	destMac := popField(result, "destination-mac")

	if destLinux != nil || destMac != nil {
		perOS := make(map[string]interface{})
		// Preserve existing per-os if present
		if existing, ok := result["per-os"].(map[string]interface{}); ok {
			for k, v := range existing {
				perOS[k] = v
			}
		}
		if destLinux != nil {
			perOS["linux"] = destLinux
		}
		if destMac != nil {
			perOS["mac"] = destMac
		}
		result["per-os"] = perOS
	}

	// Migrate destination-override → per-os
	destOverride := popField(result, "destination-override")
	if destOverride != nil {
		perOS := make(map[string]interface{})
		if existing, ok := result["per-os"].(map[string]interface{}); ok {
			for k, v := range existing {
				perOS[k] = v
			}
		}

		switch v := destOverride.(type) {
		case map[string]interface{}:
			// Dict: {linux: /path, mac: /path}
			for osName, dest := range v {
				perOS[osName] = dest
			}
		default:
			// String value: applies to both linux and mac
			perOS["linux"] = destOverride
			perOS["mac"] = destOverride
		}

		result["per-os"] = perOS
	}

	return result
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// copyMap returns a shallow copy of a map.
func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// popField removes a key from a map and returns its value.
// Returns nil if the key doesn't exist.
func popField(m map[string]interface{}, key string) interface{} {
	if v, ok := m[key]; ok {
		delete(m, key)
		return v
	}
	return nil
}



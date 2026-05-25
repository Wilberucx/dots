// Package resolver scans dotfile modules and resolves symlink states.
package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/system"
	"github.com/Wilberucx/dots/internal/yaml"
)

// LinkState represents the state of a single source→destination mapping.
type LinkState string

const (
	StateLinked   LinkState = "linked"
	StateConflict LinkState = "conflict"
	StatePending  LinkState = "pending"
	StateMissing  LinkState = "missing"
	StateUnsafe   LinkState = "unsafe"
)

// LinkStatus holds the resolved status of a single file mapping.
type LinkStatus struct {
	Source      string
	Destination string
	State       LinkState
	Detail      string
	BackupPath  string
}

// ExpandPath expands ~ and converts to an absolute path.
func ExpandPath(pathStr string) string {
	return system.ExpandPath(pathStr)
}

// GetModuleVariantInfo returns variant information for a specific module.
func GetModuleVariantInfo(cfg *config.DotsConfig, moduleName string) (*yaml.VariantInfo, error) {
	yamlPath := filepath.Join(cfg.RepoRoot, moduleName, "path.yaml")

	mappings, err := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
	if err != nil {
		return nil, err
	}
	if mappings == nil {
		return nil, nil
	}

	info := yaml.DetectVariants(mappings)
	return &info, nil
}

// GetActiveVariant determines which variant is currently active for a module
// by inspecting existing symlinks.
func GetActiveVariant(cfg *config.DotsConfig, moduleName string) (string, error) {
	yamlPath := filepath.Join(cfg.RepoRoot, moduleName, "path.yaml")

	mappings, err := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
	if err != nil {
		return "", err
	}
	if mappings == nil {
		return "", nil
	}

	variantInfo := yaml.DetectVariants(mappings)
	if !variantInfo.HasVariants {
		return "", nil
	}

	moduleDir := filepath.Join(cfg.RepoRoot, moduleName)

	// For each variant, check if ALL its source files are symlinked correctly
	for _, variantName := range variantInfo.Variants {
		allLinked := true
		for _, m := range mappings {
			if yaml.VariantKey(m.Source) != variantName {
				continue
			}
			destPath := strings.TrimRight(system.ExpandPath(m.Destination), "/")
			srcPath := filepath.Join(moduleDir, strings.TrimLeft(m.Source, "/"))

			if fi, err := os.Lstat(destPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				linkTarget, err := os.Readlink(destPath)
				if err != nil {
					allLinked = false
					break
				}

				// Resolve relative symlinks
				if !filepath.IsAbs(linkTarget) {
					linkTarget = filepath.Join(filepath.Dir(destPath), linkTarget)
				}
				linkTarget, err = filepath.EvalSymlinks(linkTarget)
				if err != nil {
					allLinked = false
					break
				}

				srcResolved, err := filepath.EvalSymlinks(srcPath)
				if err != nil {
					allLinked = false
					break
				}

				if linkTarget != srcResolved {
					allLinked = false
					break
				}
			} else {
				allLinked = false
				break
			}
		}
		if allLinked {
			return variantName, nil
		}
	}

	return "", nil
}

// ResolveModules scans all configuration modules and returns link status for each.
// modules and types are optional filters; variant can force a specific variant.
func ResolveModules(
	cfg *config.DotsConfig,
	modules []string,
	types []string,
	variant string,
) (map[string][]LinkStatus, error) {
	results := make(map[string][]LinkStatus)

	modDirs, err := cfg.GetModuleDirs(modules, types)
	if err != nil {
		return nil, fmt.Errorf("listing module dirs: %w", err)
	}

	for _, mod := range modDirs {
		yamlPath := filepath.Join(mod.Path, "path.yaml")

		mappings, err := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", yamlPath, err)
		}
		if mappings == nil {
			continue
		}

		// Detect variants
		variantInfo := yaml.DetectVariants(mappings)

		// Apply variant filtering
		if variant != "" {
			// User specified a variant
			mappings = yaml.FilterByVariant(mappings, variant)
		} else if variantInfo.HasVariants {
			// Use active variant if linked, else cascade to default
			active, _ := GetActiveVariant(cfg, mod.Name)
			effective := active
			if effective == "" {
				effective = variantInfo.DefaultVariant
			}
			mappings = yaml.FilterByVariant(mappings, effective)
		}

		statuses := resolveModuleMappings(cfg, mod.Path, mappings, cfg.CurrentOS)
		if len(statuses) > 0 {
			results[mod.Name] = statuses
		}
	}

	return results, nil
}

// resolveModuleMappings resolves the state for each mapping in a module.
func resolveModuleMappings(cfg *config.DotsConfig, modulePath string, mappings []yaml.DotFileMapping, currentOS string) []LinkStatus {
	var statuses []LinkStatus
	homeDir := system.HomeDir()

	for _, m := range mappings {
		// Clean source path
		cleanSource := strings.TrimLeft(m.Source, "/")
		if cleanSource == "" {
			cleanSource = "."
		}

		sourcePath := filepath.Join(modulePath, strings.TrimLeft(m.Source, "/"))

		// Handle globs
		var sources []string
		if strings.Contains(cleanSource, "*") {
			matches, _ := filepath.Glob(sourcePath)
			sources = matches
		} else {
			if _, err := os.Stat(sourcePath); err == nil {
				sources = []string{sourcePath}
			}
		}

		// Check if destination expresses "expand into" intent (ends with /*)
		destIsContainer := strings.HasSuffix(m.Destination, "/*")
		destStr := strings.TrimRight(m.Destination, "*")
		if destIsContainer {
			destStr = strings.TrimSuffix(destStr, "/")
		}
		isGlobSource := strings.Contains(cleanSource, "*")

		for _, src := range sources {
			srcName := filepath.Base(src)
			// OS suffix check (file-linux, file-mac, file-windows)
			if strings.Contains(srcName, "-") {
				suffix := srcName[strings.LastIndex(srcName, "-")+1:]
				if (suffix == "linux" || suffix == "mac" || suffix == "windows") && suffix != currentOS {
					continue
				}
			}

			dest := ExpandPath(destStr)

			// Determine final destination
			var finalDest string
			if isGlobSource {
				finalDest = filepath.Join(dest, srcName)
			} else if fi, err := os.Stat(src); err == nil && fi.IsDir() && destIsContainer {
				// Expand contents: link each child file into dest individually
				entries, _ := os.ReadDir(src)
				for _, child := range entries {
					childPath := filepath.Join(src, child.Name())
					childDest := filepath.Join(dest, child.Name())
					st := resolveSingleState(childPath, childDest, modulePath, homeDir)
					statuses = append(statuses, st)
				}
				continue
			} else if fi, err := os.Stat(src); err == nil && fi.IsDir() {
				finalDest = dest
			} else {
				if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
					finalDest = filepath.Join(dest, srcName)
				} else {
					finalDest = dest
				}
			}

			st := resolveSingleState(src, finalDest, modulePath, homeDir)
			statuses = append(statuses, st)
		}
	}

	return statuses
}

// resolveSingleState resolves the state of a single source→destination mapping.
// It stores ABSOLUTE paths in LinkStatus. The `~` shorthand is only for display.
func resolveSingleState(src, dest, modulePath, homeDir string) LinkStatus {
	// Safety check
	if !system.IsSafePath(dest) {
		origPath := dest + ".orig"
		backup := ""
		if _, err := os.Stat(origPath); err == nil {
			backup = origPath
		}
		return LinkStatus{
			Source:      src,
			Destination: dest,
			State:       StateUnsafe,
			Detail:      "path outside home directory",
			BackupPath:  backup,
		}
	}

	origPath := dest + ".orig"
	backup := ""
	if _, err := os.Stat(origPath); err == nil {
		backup = origPath
	}

	// Strip trailing slash so os.Lstat doesn't follow symlinks
	dest = strings.TrimRight(dest, "/")

	// Case 1: destination exists and is a symlink
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(dest)
		if err != nil {
			return LinkStatus{
				Source:      src,
				Destination: dest,
				State:       StateConflict,
				Detail:      fmt.Sprintf("cannot read link: %v", err),
				BackupPath:  backup,
			}
		}

		// Expand ~ BEFORE IsAbs — OS stores symlinks with literal ~ strings.
		// filepath.IsAbs("~/...") == false, so without expansion it gets joined
		// to the destination's parent directory, producing a wrong path.
		target = system.ExpandPath(target)

		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(dest), target)
		}

		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			// Target doesn't exist on disk — dangling symlink
			return LinkStatus{
				Source:      src,
				Destination: dest,
				State:       StateConflict,
				Detail:      fmt.Sprintf("points to %s (dangling)", shortPath(target, homeDir)),
				BackupPath:  backup,
			}
		}

		// Resolve the expected source too (handles symlinks inside the repo)
		srcResolved, err := filepath.EvalSymlinks(src)
		if err != nil {
			srcResolved = src
		}

		if resolvedTarget == srcResolved {
			return LinkStatus{
				Source:      src,
				Destination: dest,
				State:       StateLinked,
				Detail:      "",
				BackupPath:  backup,
			}
		}

		return LinkStatus{
			Source:      src,
			Destination: dest,
			State:       StateConflict,
			Detail:      fmt.Sprintf("points to %s", shortPath(resolvedTarget, homeDir)),
			BackupPath:  backup,
		}
	}

	// Case 2: destination exists but is a regular file/dir (needs backup before linking)
	if _, err := os.Stat(dest); err == nil {
		return LinkStatus{
			Source:      src,
			Destination: dest,
			State:       StatePending,
			Detail:      "backup needed",
			BackupPath:  backup,
		}
	}

	// Case 3: destination doesn't exist — ready to link
	return LinkStatus{
		Source:      src,
		Destination: dest,
		State:       StatePending,
		Detail:      "will create",
		BackupPath:  "",
	}
}

// shortPath replaces homeDir with ~ for display.
func shortPath(path, homeDir string) string {
	if strings.HasPrefix(path, homeDir) {
		return "~" + strings.TrimPrefix(path, homeDir)
	}
	return path
}



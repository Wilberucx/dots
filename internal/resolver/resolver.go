// Package resolver scans dotfile modules and resolves symlink states.
package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	luacfg "github.com/Wilberucx/dots/internal/lua"
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
	modulePath := filepath.Join(cfg.RepoRoot, moduleName)
	yamlPath := filepath.Join(modulePath, "path.yaml")

	// Try Lua variant detection first
	dotLuaPath := filepath.Join(modulePath, "dots.lua")
	if _, err := os.Stat(dotLuaPath); err == nil {
		return getLuaVariantInfo(cfg, moduleName, modulePath)
	}

	// Fall back to YAML variant detection
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

// getLuaVariantInfo detects variants from a Lua module config.
// Supports two modes:
//   - Explicit: files use :variant("name") — variants are named explicitly
//   - Implicit: multiple files with same destination — variants inferred from source path
func getLuaVariantInfo(cfg *config.DotsConfig, moduleName, modulePath string) (*yaml.VariantInfo, error) {
	luaPath := filepath.Join(modulePath, "dots.lua")
	vm := luacfg.NewLuaVM()
	defer vm.Close()

	moduleCfg, err := vm.LoadModuleConfig(luaPath)
	if err != nil {
		return nil, err
	}
	if moduleCfg == nil {
		return nil, nil
	}

	// Check for explicit :variant() declarations
	hasExplicitVariants := false
	for _, f := range moduleCfg.Files {
		if f.VariantName != "" {
			hasExplicitVariants = true
			break
		}
	}

	if hasExplicitVariants {
		return getExplicitLuaVariants(moduleCfg, cfg.CurrentOS)
	}

	// Implicit mode: detect variants by grouping files with same destination
	return getImplicitLuaVariants(moduleCfg, cfg.CurrentOS)
}

// getExplicitLuaVariants detects variants using explicit :variant("name") declarations.
func getExplicitLuaVariants(moduleCfg *luacfg.ModuleConfig, currentOS string) (*yaml.VariantInfo, error) {
	// Group files by variant name
	variantToFiles := make(map[string][]luacfg.FileOp)
	var variantOrder []string

	for _, f := range moduleCfg.Files {
		if f.VariantName == "" {
			continue // base files are not variants
		}
		dest := resolveLuaDest(f, currentOS)
		if dest == "" {
			continue
		}
		if _, seen := variantToFiles[f.VariantName]; !seen {
			variantOrder = append(variantOrder, f.VariantName)
		}
		variantToFiles[f.VariantName] = append(variantToFiles[f.VariantName], f)
	}

	if len(variantOrder) == 0 {
		return &yaml.VariantInfo{}, nil
	}

	variants := variantOrder
	defaultVariant := variants[len(variants)-1] // last variant is default

	// Build variant destinations: first file's destination per variant
	variantDests := make(map[string]string)
	for _, v := range variants {
		files := variantToFiles[v]
		if len(files) > 0 {
			dest := resolveLuaDest(files[0], currentOS)
			if dest != "" {
				variantDests[v] = dest
			}
		}
	}

	return &yaml.VariantInfo{
		HasVariants:        true,
		Variants:           variants,
		DefaultVariant:     defaultVariant,
		VariantDestinations: variantDests,
	}, nil
}

// getImplicitLuaVariants detects variants by grouping files with the same destination.
func getImplicitLuaVariants(moduleCfg *luacfg.ModuleConfig, currentOS string) (*yaml.VariantInfo, error) {
	destToSources := make(map[string][]string)
	for _, f := range moduleCfg.Files {
		dest := resolveLuaDest(f, currentOS)
		if dest == "" {
			continue
		}
		destToSources[dest] = append(destToSources[dest], f.Source)
	}

	var variants []string
	variantDests := make(map[string]string)

	for dest, sources := range destToSources {
		if len(sources) > 1 {
			for _, src := range sources {
				variants = append(variants, src)
				variantDests[src] = dest
			}
		}
	}

	if len(variants) == 0 {
		return &yaml.VariantInfo{}, nil
	}

	defaultVariant := variants[len(variants)-1]
	return &yaml.VariantInfo{
		HasVariants:        true,
		Variants:           variants,
		DefaultVariant:     defaultVariant,
		VariantDestinations: variantDests,
	}, nil
}

// GetActiveVariant determines which variant is currently active for a module
// by inspecting existing symlinks.
func GetActiveVariant(cfg *config.DotsConfig, moduleName string) (string, error) {
	modulePath := filepath.Join(cfg.RepoRoot, moduleName)
	dotLuaPath := filepath.Join(modulePath, "dots.lua")

	// Check if it's a Lua module
	if _, err := os.Stat(dotLuaPath); err == nil {
		return getLuaActiveVariant(cfg, moduleName, modulePath)
	}

	yamlPath := filepath.Join(modulePath, "path.yaml")
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

// getLuaActiveVariant determines the active variant for a Lua module.
func getLuaActiveVariant(cfg *config.DotsConfig, moduleName, modulePath string) (string, error) {
	vInfo, err := getLuaVariantInfo(cfg, moduleName, modulePath)
	if err != nil || vInfo == nil || !vInfo.HasVariants {
		return "", err
	}

	luaPath := filepath.Join(modulePath, "dots.lua")
	vm := luacfg.NewLuaVM()
	defer vm.Close()

	moduleCfg, err := vm.LoadModuleConfig(luaPath)
	if err != nil {
		return "", err
	}

	// Check if using explicit variants
	hasExplicitVariants := false
	for _, f := range moduleCfg.Files {
		if f.VariantName != "" {
			hasExplicitVariants = true
			break
		}
	}

	if hasExplicitVariants {
		// Explicit mode: check each variant by its :variant("name")
		for _, variantName := range vInfo.Variants {
			allLinked := true
			for _, f := range moduleCfg.Files {
				if f.VariantName != variantName {
					continue
				}
				if !luaFileIsLinked(f, modulePath, cfg.CurrentOS) {
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

	// Implicit mode: check each variant by source path matching
	for _, variantName := range vInfo.Variants {
		allLinked := true
		for _, f := range moduleCfg.Files {
			dest := resolveLuaDest(f, cfg.CurrentOS)
			if dest == "" {
				continue
			}
			dest = system.ExpandPath(dest)
			srcPath := filepath.Join(modulePath, strings.TrimLeft(f.Source, "/"))

			if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				linkTarget, err := os.Readlink(dest)
				if err != nil {
					allLinked = false
					break
				}

				if !filepath.IsAbs(linkTarget) {
					linkTarget = filepath.Join(filepath.Dir(dest), linkTarget)
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

// luaFileIsLinked checks if a specific Lua FileOp is correctly symlinked.
// Handles FileOpFile, FileOpDirTo, FileOpDirInto, and FileOpGlob.
func luaFileIsLinked(f luacfg.FileOp, modulePath, currentOS string) bool {
	dest := resolveLuaDest(f, currentOS)
	if dest == "" {
		return false
	}
	dest = system.ExpandPath(dest)
	srcPath := filepath.Join(modulePath, strings.TrimLeft(f.Source, "/"))

	switch f.Type {
	case luacfg.FileOpDirInto:
		// For dir():into(), check each child file individually
		fi, err := os.Stat(srcPath)
		if err != nil || !fi.IsDir() {
			return false
		}
		entries, err := os.ReadDir(srcPath)
		if err != nil {
			return false
		}
		for _, child := range entries {
			childSrc := filepath.Join(srcPath, child.Name())
			childDest := filepath.Join(dest, child.Name())
			if !isSingleSymlink(childSrc, childDest) {
				return false
			}
		}
		return true

	case luacfg.FileOpGlob:
		// For glob():into(), check each matched file
		srcPattern := filepath.Join(modulePath, f.Pattern)
		matches, err := filepath.Glob(srcPattern)
		if err != nil || len(matches) == 0 {
			return false
		}
		for _, match := range matches {
			childDest := filepath.Join(dest, filepath.Base(match))
			if !isSingleSymlink(match, childDest) {
				return false
			}
		}
		return true

	case luacfg.FileOpDirTo:
		// For dir():to(), check if dest is a symlink to the source directory
		return isSingleSymlink(srcPath, dest)

	default:
		// For file() and other types, check if dest is a symlink to the source file
		return isSingleSymlink(srcPath, dest)
	}
}

// isSingleSymlink checks if dest is a symlink pointing to src.
func isSingleSymlink(src, dest string) bool {
	fi, err := os.Lstat(dest)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
	}

	linkTarget, err := os.Readlink(dest)
	if err != nil {
		return false
	}

	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Join(filepath.Dir(dest), linkTarget)
	}
	linkTarget, err = filepath.EvalSymlinks(linkTarget)
	if err != nil {
		return false
	}

	srcResolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		srcResolved = src
	}

	return linkTarget == srcResolved
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
		if mod.Type == int(luacfg.ModuleTypeLua) {
			// Lua module resolution (with variant support)
			statuses, err := resolveLuaModule(cfg, mod, variant)
			if err != nil {
				return nil, fmt.Errorf("resolving lua module %s: %w", mod.Name, err)
			}

			// If no statuses and no variant specified, check for variant-only modules
			if len(statuses) == 0 && variant == "" {
				vInfo, vErr := getLuaVariantInfo(cfg, mod.Name, mod.Path)
				if vErr == nil && vInfo != nil && vInfo.HasVariants {
					// All files are behind variants — resolve with active or default variant
					activeVariant, _ := getLuaActiveVariant(cfg, mod.Name, mod.Path)
					effectiveVariant := activeVariant
					if effectiveVariant == "" {
						effectiveVariant = vInfo.DefaultVariant
					}
					statuses, err = resolveLuaModule(cfg, mod, effectiveVariant)
					if err != nil {
						return nil, fmt.Errorf("resolving lua module %s with variant %s: %w", mod.Name, effectiveVariant, err)
					}
					results[mod.Name] = statuses
					continue
				}
			}

			if len(statuses) > 0 || variant != "" {
				results[mod.Name] = statuses
			}
		} else {
			// YAML module resolution (original behavior)
			statuses, err := resolveYAMLModule(cfg, mod, variant)
			if err != nil {
				return nil, fmt.Errorf("resolving yaml module %s: %w", mod.Name, err)
			}
			if len(statuses) > 0 {
				results[mod.Name] = statuses
			}
		}
	}

	return results, nil
}

// resolveYAMLModule resolves a YAML-based module (original behavior).
func resolveYAMLModule(cfg *config.DotsConfig, mod config.ModuleDir, variant string) ([]LinkStatus, error) {
	yamlPath := filepath.Join(mod.Path, "path.yaml")

	mappings, err := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", yamlPath, err)
	}
	if mappings == nil {
		return nil, nil
	}

	// Detect variants
	variantInfo := yaml.DetectVariants(mappings)

	// Apply variant filtering
	if variant != "" {
		mappings = yaml.FilterByVariant(mappings, variant)
	} else if variantInfo.HasVariants {
		active, _ := GetActiveVariant(cfg, mod.Name)
		effective := active
		if effective == "" {
			effective = variantInfo.DefaultVariant
		}
		mappings = yaml.FilterByVariant(mappings, effective)
	}

	return resolveModuleMappings(cfg, mod.Path, mappings, cfg.CurrentOS), nil
}

// resolveLuaModule resolves a Lua-based module using the Lua VM.
// Applies variant filtering: if variant != "", only files matching that variant
// (or base files without a variant) are included.
func resolveLuaModule(cfg *config.DotsConfig, mod config.ModuleDir, variant string) ([]LinkStatus, error) {
	luaPath := filepath.Join(mod.Path, "dots.lua")
	if _, err := os.Stat(luaPath); os.IsNotExist(err) {
		return nil, nil
	}

	vm := luacfg.NewLuaVM()
	defer vm.Close()

	moduleCfg, err := vm.LoadModuleConfig(luaPath)
	if err != nil {
		return nil, err
	}
	if moduleCfg == nil {
		return nil, nil
	}

	// Check if the module uses explicit variants
	hasExplicitVariants := false
	for _, f := range moduleCfg.Files {
		if f.VariantName != "" {
			hasExplicitVariants = true
			break
		}
	}

	// For explicit variants: filter by variant name
	// For implicit variants: filter by source path (variant name = source path)
	homeDir := system.HomeDir()
	var statuses []LinkStatus

	for _, fileOp := range moduleCfg.Files {
		// Explicit variant filtering
		if hasExplicitVariants && variant != "" && fileOp.VariantName != "" && fileOp.VariantName != variant {
			continue // skip: doesn't match the requested variant
		}
		// When no variant specified and file has explicit variant, skip it
		if hasExplicitVariants && variant == "" && fileOp.VariantName != "" {
			continue // skip: no variant selected, don't link variant files
		}
		// Implicit variant filtering: only include the file whose source matches the variant name
		// In implicit mode, variant names ARE the source paths
		if !hasExplicitVariants && variant != "" && fileOp.Source != variant {
			continue // skip: source doesn't match the requested variant
		}
		sts := resolveLuaFileOp(cfg, mod, fileOp, homeDir)
		statuses = append(statuses, sts...)
	}

	return statuses, nil
}

// resolveLuaFileOp converts a single FileOp into LinkStatus(es).
func resolveLuaFileOp(cfg *config.DotsConfig, mod config.ModuleDir, op luacfg.FileOp, homeDir string) []LinkStatus {
	var statuses []LinkStatus

	switch op.Type {
	case luacfg.FileOpFile:
		// Simple file symlink
		srcPath := filepath.Join(mod.Path, strings.TrimLeft(op.Source, "/"))
		dest := resolveLuaDest(op, cfg.CurrentOS)
		if dest == "" || op.Source == "" {
			return nil
		}
		dest = system.ExpandPath(dest)

		if _, err := os.Stat(srcPath); err == nil {
			st := resolveSingleState(srcPath, dest, mod.Path, homeDir)
			statuses = append(statuses, st)
		}

	case luacfg.FileOpDirTo:
		// Directory symlink: source → dest
		srcPath := filepath.Join(mod.Path, strings.TrimLeft(op.Source, "/"))
		dest := resolveLuaDest(op, cfg.CurrentOS)
		if dest == "" || op.Source == "" {
			return nil
		}
		dest = system.ExpandPath(dest)

		if fi, err := os.Stat(srcPath); err == nil && fi.IsDir() {
			st := resolveSingleState(srcPath, dest, mod.Path, homeDir)
			statuses = append(statuses, st)
		}

	case luacfg.FileOpDirInto:
		// Expand contents: each child file goes to dest individually
		srcPath := filepath.Join(mod.Path, strings.TrimLeft(op.Source, "/"))
		dest := resolveLuaDest(op, cfg.CurrentOS)
		if dest == "" || op.Source == "" {
			return nil
		}
		dest = system.ExpandPath(dest)

		if fi, err := os.Stat(srcPath); err == nil && fi.IsDir() {
			entries, _ := os.ReadDir(srcPath)
			for _, child := range entries {
				childPath := filepath.Join(srcPath, child.Name())
				childDest := filepath.Join(dest, child.Name())
				st := resolveSingleState(childPath, childDest, mod.Path, homeDir)
				statuses = append(statuses, st)
			}
		}

	case luacfg.FileOpGlob:
		// Glob pattern matching
		srcPattern := filepath.Join(mod.Path, op.Pattern)
		dest := resolveLuaDest(op, cfg.CurrentOS)
		if dest == "" {
			return nil
		}
		dest = system.ExpandPath(dest)

		matches, err := filepath.Glob(srcPattern)
		if err != nil || len(matches) == 0 {
			return nil
		}
		for _, match := range matches {
			childDest := filepath.Join(dest, filepath.Base(match))
			st := resolveSingleState(match, childDest, mod.Path, homeDir)
			statuses = append(statuses, st)
		}
	}

	return statuses
}

// resolveLuaDest determines the final destination for a FileOp,
// respecting OS filtering and per_os overrides.
func resolveLuaDest(op luacfg.FileOp, currentOS string) string {
	// OS filter check
	if op.OSFilter != "" && op.OSFilter != currentOS {
		return "" // filtered out
	}

	// Per-OS destination
	if op.PerOS != nil {
		if d, ok := op.PerOS[currentOS]; ok {
			return d
		}
	}

	return op.Destination
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



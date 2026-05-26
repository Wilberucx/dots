package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FindModules discovers modules in a dotfiles repository.
// It respects the RootConfig.ModulePaths to limit the search scope.
// Returns modules found sorted by name.
func FindModules(repoRoot string, initCfg *RootConfig) ([]ModuleDir, error) {
	var searchDirs []string

	if initCfg != nil && len(initCfg.ModulePaths) > 0 {
		// module_paths specified: search ONLY in those paths
		for _, p := range initCfg.ModulePaths {
			fullPath := filepath.Join(repoRoot, p)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[WARN] module_paths %q does not exist — skipping\n", p)
				continue
			}
			searchDirs = append(searchDirs, fullPath)
		}
	} else {
		// Default: search in repo root
		searchDirs = append(searchDirs, repoRoot)
	}

	var modules []ModuleDir
	seen := make(map[string]bool) // name → already found

	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); os.IsNotExist(err) {
			continue
		}

		// Walk recursively, looking for dots.lua or path.yaml
		filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip inaccessible
			}

			// Skip hidden directories, .git, .dots, node_modules
			if info.IsDir() {
				name := info.Name()
				if name == ".git" || name == ".dots" || name == "node_modules" || name == "cli" {
					return filepath.SkipDir
				}
				if strings.HasPrefix(name, ".") && path != searchDir {
					return filepath.SkipDir
				}
				return nil
			}

			// Only care about config files
			if info.Name() != "dots.lua" && info.Name() != "path.yaml" {
				return nil
			}

			modDir := filepath.Dir(path)
			modName := filepath.Base(modDir)

			// Skip root-level config files (init.lua's sibling)
			if modDir == repoRoot || isSubdirOf(searchDir, modDir, searchDir) && modDir == searchDir {
				// For non-root search dirs, also skip the search dir itself
				if modDir != repoRoot {
					for _, sd := range searchDirs {
						if modDir == sd {
							return nil
						}
					}
				}
				return nil
			}

			// Skip if already seen (module_paths should not overlap)
			if seen[modName] {
				return nil
			}

			// Determine module type
			dotLuaPath := filepath.Join(modDir, "dots.lua")
			pathYAMLPath := filepath.Join(modDir, "path.yaml")

			hasLua := false
			if _, err := os.Stat(dotLuaPath); err == nil {
				hasLua = true
			}
			hasYAML := false
			if _, err := os.Stat(pathYAMLPath); err == nil {
				hasYAML = true
			}

			if hasLua && hasYAML {
				// Both exist — Lua takes priority with warning
				seen[modName] = true
				modules = append(modules, ModuleDir{Name: modName, Path: modDir, Type: ModuleTypeLua})
				fmt.Fprintf(os.Stderr, "[WARN] Module '%s' has both dots.lua and path.yaml. Using dots.lua.\n", modName)
				return nil
			}

			if hasLua {
				seen[modName] = true
				modules = append(modules, ModuleDir{Name: modName, Path: modDir, Type: ModuleTypeLua})
				return nil
			}

			if hasYAML {
				seen[modName] = true
				modules = append(modules, ModuleDir{Name: modName, Path: modDir, Type: ModuleTypeYAML})
				return nil
			}

			return nil
		})
	}

	// Sort by name for consistent output
	sortModules(modules)

	return modules, nil
}

// isSubdirOf returns true if path is a subdirectory of base (or the same).
func isSubdirOf(path, base string, searchDir string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// sortModules sorts modules by name in-place.
func sortModules(modules []ModuleDir) {
	for i := 0; i < len(modules); i++ {
		for j := i + 1; j < len(modules); j++ {
			if modules[i].Name > modules[j].Name {
				modules[i], modules[j] = modules[j], modules[i]
			}
		}
	}
}

// IsLuaRepo checks if a path contains init.lua as a dotfiles repo marker.
func IsLuaRepo(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "init.lua")); err == nil {
		return true
	}
	// Also check config.lua as fallback
	if _, err := os.Stat(filepath.Join(path, "config.lua")); err == nil {
		return true
	}
	return false
}

// LoadInitConfig loads init.lua and returns the RootConfig.
func LoadInitConfig(repoRoot string) (*RootConfig, error) {
	initPath := filepath.Join(repoRoot, "init.lua")
	if _, err := os.Stat(initPath); os.IsNotExist(err) {
		return nil, nil
	}

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadRootConfig(initPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadModuleConfigForModule loads a module's configuration (dots.lua or path.yaml).
func LoadModuleConfigForModule(module ModuleDir) (*ModuleConfig, error) {
	if module.Type == ModuleTypeLua {
		return loadLuaModuleConfig(module.Path)
	}
	// YAML modules are handled by the existing yaml package
	return nil, nil
}

// loadLuaModuleConfig loads a dots.lua file from a module directory.
func loadLuaModuleConfig(modPath string) (*ModuleConfig, error) {
	luaPath := filepath.Join(modPath, "dots.lua")
	if _, err := os.Stat(luaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("dots.lua not found in %s", modPath)
	}

	vm := NewLuaVM()
	defer vm.Close()

	cfg, err := vm.LoadModuleConfig(luaPath)
	if err != nil {
		return nil, fmt.Errorf("module %s: %v", filepath.Base(modPath), err)
	}

	return cfg, nil
}

// ConvertYAMLDepsToDepOps converts YAML Dependency objects to DepOp (for unified handling).
func ConvertYAMLDepsToDepOps(yamlDeps []interface{}) []DepOp {
	var deps []DepOp
	for _, d := range yamlDeps {
		if depMap, ok := d.(map[string]interface{}); ok {
			dep := DepOp{
				Name:        getStrFromMap(depMap, "name"),
				Type:        getStrFromMap(depMap, "type", "package"),
				URL:         getStrFromMap(depMap, "url"),
				Destination: getStrFromMap(depMap, "dest"),
				Version:     getStrFromMap(depMap, "version"),
				Ref:         getStrFromMap(depMap, "ref"),
				Extract:     getStrFromMap(depMap, "extract"),
				Bin:         getStrFromMap(depMap, "bin"),
				PostInstall: getStrFromMap(depMap, "post-install"),
			}

			if dep.PostInstall == "" {
				dep.PostInstall = getStrFromMap(depMap, "post_install")
			}

			if archRaw, ok := depMap["arch"].(map[string]interface{}); ok {
				dep.Arch = make(map[string]string)
				for k, v := range archRaw {
					dep.Arch[k] = fmt.Sprintf("%v", v)
				}
			}

			if mgrsRaw, ok := depMap["managers"].(map[string]interface{}); ok {
				dep.Managers = make(map[string]string)
				for k, v := range mgrsRaw {
					dep.Managers[k] = fmt.Sprintf("%v", v)
				}
			}

			if fbRaw, ok := depMap["fallback"].(map[string]interface{}); ok {
				fb := ConvertYAMLDepsToDepOps([]interface{}{fbRaw})
				if len(fb) > 0 {
					dep.Fallback = &fb[0]
				}
			}

			deps = append(deps, dep)
		}
	}
	return deps
}

func getStrFromMap(m map[string]interface{}, key string, defaults ...string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

// ParseYAMLFile parses a YAML file into a map for dependency conversion.
func ParseYAMLFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

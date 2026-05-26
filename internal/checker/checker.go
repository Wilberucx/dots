// Package checker validates path.yaml and dots.lua schemas, reporting issues like an LSP.
// It scans all modules in the dotfiles repository and produces diagnostic
// messages for any syntax or semantic problems.
package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	luacfg "github.com/Wilberucx/dots/internal/lua"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/yaml"
	"github.com/charmbracelet/lipgloss"
	yamlv3 "gopkg.in/yaml.v3"
)

// Severity indicates how serious an issue is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityHint    Severity = "hint"
)

// Issue represents a single diagnostic finding.
type Issue struct {
	Module   string   // Module name (e.g. "Nvim")
	File     string   // Path to config file relative to repo root
	Severity Severity
	Message  string
	Field    string   // Optional: the problematic field name
}

// Result holds all findings from a syntax check run.
type Result struct {
	Issues []Issue
}

// HasErrors returns true if any issue has error severity.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any issue has warning severity.
func (r *Result) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// CheckOptions controls optional behavior of the syntax checker.
type CheckOptions struct {
	// NoHints suppresses informational hints (e.g. migration suggestions).
	NoHints bool
}

// RunSyntaxCheck scans all modules and validates their config files (dots.lua or path.yaml).
// Pass CheckOptions{NoHints: true} to suppress migration hints.
func RunSyntaxCheck(cfg *config.DotsConfig, opts ...CheckOptions) *Result {
	options := CheckOptions{}
	if len(opts) > 0 {
		options = opts[0]
	}

	result := &Result{}

	modDirs, err := cfg.GetModuleDirs(nil, nil)
	if err != nil {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("cannot scan modules: %v", err),
		})
		return result
	}

	var hasYAMLModules bool

	for _, mod := range modDirs {
		// Check Lua modules
		luaPath := filepath.Join(mod.Path, "dots.lua")
		if mod.Type == int(luacfg.ModuleTypeLua) {
			relPath := filepath.Join(mod.Name, "dots.lua")
			checkLuaModule(mod.Name, relPath, luaPath, cfg, result)
			continue
		}

		// Check YAML modules
		yamlPath := filepath.Join(mod.Path, "path.yaml")
		relPath := filepath.Join(mod.Name, "path.yaml")
		checkPathYAML(mod.Name, relPath, yamlPath, cfg, result)
		hasYAMLModules = true

		// Hint: suggest migration from YAML to Lua
		if !options.NoHints {
			result.Issues = append(result.Issues, Issue{
				Module:   mod.Name,
				File:     relPath,
				Severity: SeverityHint,
				Message:  "module uses legacy YAML format — consider migrating to dots.lua (see: https://github.com/Wilberucx/dots/docs/lua-syntax.md)",
			})
		}
	}

	// Advisory: no init.lua detected
	if !options.NoHints && !cfg.IsLuaRepo && hasYAMLModules {
		result.Issues = append(result.Issues, Issue{
			Severity: SeverityHint,
			Message:  "no init.lua found in repo root — create one to enable Lua configuration (see: https://github.com/Wilberucx/dots/docs/lua-syntax.md)",
		})
	}

	return result
}

// checkLuaModule validates a single dots.lua file for syntax errors and source existence.
func checkLuaModule(moduleName, relPath, luaPath string, cfg *config.DotsConfig, result *Result) {
	if _, err := os.Stat(luaPath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Message:  "dots.lua not found in module directory",
		})
		return
	}

	// Syntax check using the Lua VM
	err := luacfg.CheckSyntax(luaPath)
	if err != nil {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Message:  fmt.Sprintf("syntax/load error: %v", err),
		})
		return
	}

	// Validate source file existence (reuse the already-parsed config by loading it again)
	checkLuaSourceExistence(moduleName, relPath, luaPath, cfg, result)
}

// checkLuaSourceExistence validates that source files referenced in a Lua config exist.
// Takes an optional pre-loaded config to avoid loading the same file twice.
func checkLuaSourceExistence(moduleName, relPath, luaPath string, cfg *config.DotsConfig, result *Result, existingCfg ...*luacfg.ModuleConfig) {
	var moduleCfg *luacfg.ModuleConfig

	if len(existingCfg) > 0 && existingCfg[0] != nil {
		// Reuse already-loaded config to avoid creating another LState
		moduleCfg = existingCfg[0]
	} else {
		vm := luacfg.NewLuaVM()
		defer vm.Close()

		var err error
		moduleCfg, err = vm.LoadModuleConfig(luaPath)
		if err != nil || moduleCfg == nil {
			return
		}
	}

	for _, f := range moduleCfg.Files {
		// Skip glob patterns
		if f.Type == luacfg.FileOpGlob || strings.ContainsAny(f.Source, "*?[") {
			continue
		}

		sourcePath := filepath.Join(cfg.RepoRoot, moduleName, strings.TrimLeft(f.Source, "/"))
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("file %q: source path not found in module directory", f.Source),
			})
		}
	}
}

// CheckBrokenLinks resolves all modules and appends any conflicts/unsafe paths to the issues result.
func CheckBrokenLinks(cfg *config.DotsConfig, result *Result) {
	allModules, err := resolver.ResolveModules(cfg, nil, nil, "")
	if err == nil {
		for modName, statuses := range allModules {
			configFile := "path.yaml"
			// Check if it's a Lua module
			luaPath := filepath.Join(cfg.RepoRoot, modName, "dots.lua")
			if _, err := os.Stat(luaPath); err == nil {
				configFile = "dots.lua"
			}
			relPath := filepath.Join(modName, configFile)
			for _, st := range statuses {
				if st.State == resolver.StateConflict {
					result.Issues = append(result.Issues, Issue{
						Module:   modName,
						File:     relPath,
						Severity: SeverityError,
						Message:  fmt.Sprintf("broken link: destination '%s' %s", resolver.ExpandPath(st.Destination), st.Detail),
					})
				} else if st.State == resolver.StateUnsafe {
					result.Issues = append(result.Issues, Issue{
						Module:   modName,
						File:     relPath,
						Severity: SeverityError,
						Message:  fmt.Sprintf("unsafe path: destination '%s' %s", resolver.ExpandPath(st.Destination), st.Detail),
					})
				}
			}
		}
	}
}

// checkPathYAML validates a single path.yaml file.
func checkPathYAML(moduleName, relPath, yamlPath string, cfg *config.DotsConfig, result *Result) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Message:  fmt.Sprintf("cannot read file: %v", err),
		})
		return
	}

	var raw map[string]interface{}
	if err := yamlv3.Unmarshal(data, &raw); err != nil {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid YAML: %v", err),
		})
		return
	}

	if raw == nil {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityWarning,
			Message:  "file is empty",
		})
		return
	}

	// Check structure: must be a dict
	if _, ok := raw["dependencies"]; !ok {
		if _, ok := raw["files"]; !ok {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityWarning,
				Message:  "no 'dependencies' or 'files' sections — module is unused",
			})
			return
		}
	}

	// Check for v2 schema fields (dependencies)
	hasV2 := false
	if deps, ok := raw["dependencies"].([]interface{}); ok {
		for i, dep := range deps {
			if d, ok := dep.(map[string]interface{}); ok {
				for field := range d {
					if yaml.V2DepFields[field] {
						hasV2 = true
						result.Issues = append(result.Issues, Issue{
							Module:   moduleName,
							File:     relPath,
							Severity: SeverityError,
							Field:    field,
							Message:  fmt.Sprintf("v2 field '%s' in dependencies[%d]: use '%s' instead (v3 schema)", field, i, yaml.DepV2ToV3(field)),
						})
					}
				}
			}
		}
	}

	// Check for v2 schema fields (files)
	if files, ok := raw["files"].([]interface{}); ok {
		for i, f := range files {
			if file, ok := f.(map[string]interface{}); ok {
				for field := range file {
					if yaml.V2FileFields[field] {
						hasV2 = true
						result.Issues = append(result.Issues, Issue{
							Module:   moduleName,
							File:     relPath,
							Severity: SeverityError,
							Field:    field,
							Message:  fmt.Sprintf("v2 field '%s' in files[%d]: use 'per-os' instead (v3 schema)", field, i),
						})
					}
				}
			}
		}
	}

	// Don't continue validating structure if v2 fields are present — the parser
	// will silently ignore these files, making further validation misleading.
	if hasV2 {
		return
	}

	// Validate dependency structure
	if deps, ok := raw["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			if d, ok := dep.(map[string]interface{}); ok {
				validateDep(d, moduleName, relPath, result)
			}
		}
	}

	// Validate file mapping structure and existence
	if files, ok := raw["files"].([]interface{}); ok {
		sourceSet := make(map[string]bool)
		for _, f := range files {
			if file, ok := f.(map[string]interface{}); ok {
				validateFileMapping(file, moduleName, relPath, cfg, result)
				checkSourceExistence(file, moduleName, relPath, cfg, result)
				checkDuplicates(file, sourceSet, moduleName, relPath, result)
			}
		}
	}

	// Check for unknown top-level keys
	validKeys := map[string]bool{
		"dependencies": true,
		"files":        true,
		"type":         true,
	}
	for key := range raw {
		if !validKeys[key] {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityWarning,
				Field:    key,
				Message:  fmt.Sprintf("unknown top-level key '%s'", key),
			})
		}
	}
}

// validateDep checks a single dependency entry for structural validity.
func validateDep(dep map[string]interface{}, moduleName, relPath string, result *Result) {
	depName, _ := dep["name"].(string)
	if depName == "" {
		depName = "<unnamed>"
	}

	depType, _ := dep["type"].(string)
	if depType == "" {
		depType = "package" // default
	}

	prefix := fmt.Sprintf("dependency '%s'", depName)

	// Validate type (reuse schema's known type registry)
	if _, ok := yaml.RequiredFieldsByType[depType]; !ok {
		knownTypes := make([]string, 0, len(yaml.RequiredFieldsByType))
		for t := range yaml.RequiredFieldsByType {
			knownTypes = append(knownTypes, t)
		}
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Field:    "type",
			Message:  fmt.Sprintf("%s: unknown type '%s' (known: %v)", prefix, depType, knownTypes),
		})
	}

	// Check required fields by type (reuse schema's RequiredFieldsByType)
	if fields, ok := yaml.RequiredFieldsByType[depType]; ok {
		for _, field := range fields {
			if getStr(dep, field) == "" {
				result.Issues = append(result.Issues, Issue{
					Module:   moduleName,
					File:     relPath,
					Severity: SeverityError,
					Field:    field,
					Message:  fmt.Sprintf("%s: required field '%s' missing for type '%s'", prefix, field, depType),
				})
			}
		}
	}

	// Check for unknown fields
	knownDepFields := map[string]bool{
		"name": true, "type": true, "url": true, "dest": true,
		"version": true, "ref": true, "arch": true, "managers": true,
		"extract": true, "fallback": true, "post-install": true, "post_install": true,
		"bin": true,
	}
	for key := range dep {
		if !knownDepFields[key] {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityWarning,
				Field:    key,
				Message:  fmt.Sprintf("%s: unknown field '%s'", prefix, key),
			})
		}
	}

	// Type-check fields
	typeChecks := map[string]string{
		"name":         "string",
		"type":         "string",
		"url":          "string",
		"dest":         "string",
		"version":      "string",
		"ref":          "string",
		"extract":      "string",
		"post-install": "string",
		"post_install": "string",
		"bin":          "string",
	}
	for key, expectType := range typeChecks {
		if v, ok := dep[key]; ok {
			switch expectType {
			case "string":
				if _, ok := v.(string); !ok {
					result.Issues = append(result.Issues, Issue{
						Module:   moduleName,
						File:     relPath,
						Severity: SeverityError,
						Field:    key,
						Message:  fmt.Sprintf("%s: field '%s' must be a string, got %T", prefix, key, v),
					})
				}
			}
		}
	}

	// arch must be a dict
	if v, ok := dep["arch"]; ok {
		if _, ok := v.(map[string]interface{}); !ok {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityError,
				Field:    "arch",
				Message:  fmt.Sprintf("%s: 'arch' must be a dict (e.g. x86_64: amd64)", prefix),
			})
		}
	}

	// managers must be a dict
	if v, ok := dep["managers"]; ok {
		if _, ok := v.(map[string]interface{}); !ok {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityError,
				Field:    "managers",
				Message:  fmt.Sprintf("%s: 'managers' must be a dict (e.g. brew: fd)", prefix),
			})
		}
	}
}

// validateFileMapping checks a single file entry for structural validity.
func validateFileMapping(entry map[string]interface{}, moduleName, relPath string, cfg *config.DotsConfig, result *Result) {
	source, _ := entry["source"].(string)
	if source == "" {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Field:    "source",
			Message:  "file entry: missing required field 'source'",
		})
		return
	}

	prefix := fmt.Sprintf("file '%s'", source)

	// Check destination or per-os
	_, hasDest := entry["destination"].(string)
	_, hasPerOS := entry["per-os"].(map[string]interface{})

	if !hasDest && !hasPerOS {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityError,
			Message:  fmt.Sprintf("%s: no 'destination' or 'per-os' — nowhere to link", prefix),
		})
	}

	// Validate per-os structure
	if perOS, ok := entry["per-os"]; ok {
		if perOSMap, ok := perOS.(map[string]interface{}); ok {
			for osName := range perOSMap {
				if !isValidOS(osName) {
					result.Issues = append(result.Issues, Issue{
						Module:   moduleName,
						File:     relPath,
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("%s: 'per-os' contains unknown OS '%s' (expected: linux, mac, windows)", prefix, osName),
					})
				}
			}
		} else {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityError,
				Field:    "per-os",
				Message:  fmt.Sprintf("%s: 'per-os' must be a dict (e.g. linux: /path, mac: /path)", prefix),
			})
		}
	}

	// Validate os filter
	if osFilter, ok := entry["os"]; ok {
		if osList, ok := osFilter.([]interface{}); ok {
			for _, o := range osList {
				if osStr, ok := o.(string); ok {
					if !isValidOS(osStr) {
						result.Issues = append(result.Issues, Issue{
							Module:   moduleName,
							File:     relPath,
							Severity: SeverityWarning,
							Message:  fmt.Sprintf("%s: 'os' filter contains unknown OS '%s' (expected: linux, mac, windows)", prefix, osStr),
						})
					}
				}
			}
		} else {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityError,
				Field:    "os",
				Message:  fmt.Sprintf("%s: 'os' must be a list (e.g. [linux, mac])", prefix),
			})
		}
	}

	// Check for unknown fields
	knownFileFields := map[string]bool{
		"source": true, "destination": true, "per-os": true, "os": true,
	}
	for key := range entry {
		if !knownFileFields[key] {
			result.Issues = append(result.Issues, Issue{
				Module:   moduleName,
				File:     relPath,
				Severity: SeverityWarning,
				Field:    key,
				Message:  fmt.Sprintf("%s: unknown field '%s'", prefix, key),
			})
		}
	}
}

// checkSourceExistence checks that the source file/directory exists in the module.
func checkSourceExistence(entry map[string]interface{}, moduleName, relPath string, cfg *config.DotsConfig, result *Result) {
	source, _ := entry["source"].(string)
	if source == "" {
		return
	}

	// Skip glob patterns (they're resolved at link time)
	if strings.ContainsAny(source, "*?[") {
		return
	}

	sourcePath := filepath.Join(cfg.RepoRoot, moduleName, strings.TrimLeft(source, "/"))
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("file '%s': source path not found in module directory", source),
		})
	}
}

// checkDuplicates detects duplicate source entries within the same module.
func checkDuplicates(entry map[string]interface{}, sourceSet map[string]bool, moduleName, relPath string, result *Result) {
	source, _ := entry["source"].(string)
	if source == "" {
		return
	}

	if sourceSet[source] {
		result.Issues = append(result.Issues, Issue{
			Module:   moduleName,
			File:     relPath,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("file '%s': duplicate source entry (will be ignored)", source),
		})
	}
	sourceSet[source] = true
}

// isValidOS checks if the OS name is one of the supported values.
func isValidOS(os string) bool {
	switch os {
	case "linux", "mac", "windows":
		return true
	}
	return false
}

// getStr safely extracts a string from a map.
func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// FormatIssue formats an issue as a human-readable string.
func FormatIssue(issue Issue) string {
	var icon string
	var msgStyle lipgloss.Style

	switch issue.Severity {
	case SeverityError:
		icon = ui.IconError
		msgStyle = ui.ErrorStyle
	case SeverityWarning:
		icon = ui.IconConflict
		msgStyle = ui.WarningStyle
	default:
		icon = ui.IconPending
		msgStyle = ui.DimStyle
	}

	return msgStyle.Render(fmt.Sprintf("  %s [%s] %s", icon, issue.File, issue.Message))
}

// PrintResult prints all issues to stdout in a user-friendly format.
func PrintResult(result *Result) {
	if len(result.Issues) == 0 {
		return
	}

	for _, issue := range result.Issues {
		fmt.Println(FormatIssue(issue))
	}

	// Summary
	errCount := 0
	warnCount := 0
	for _, issue := range result.Issues {
		switch issue.Severity {
		case SeverityError:
			errCount++
		case SeverityWarning:
			warnCount++
		}
	}

	var parts []string
	if errCount > 0 {
		parts = append(parts, ui.ErrorStyle.Render(fmt.Sprintf("%d error(s)", errCount)))
	}
	if warnCount > 0 {
		parts = append(parts, ui.WarningStyle.Render(fmt.Sprintf("%d warning(s)", warnCount)))
	}

	fmt.Printf("  ── %s found\n", strings.Join(parts, ", "))
}

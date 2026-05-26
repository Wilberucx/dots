package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// InitLuaTemplate returns the template for a new init.lua file.
func InitLuaTemplate(repoName string) string {
	if repoName == "" {
		repoName = "dotfiles"
	}

	return fmt.Sprintf(`-- init.lua — Root configuration for dots
-- This file identifies this directory as a dotfiles repository managed by dots.
-- See: https://github.com/Wilberucx/dots

return {
  name = %q,

  -- module_paths: optional, restricts where dots looks for modules.
  -- Default: scan the repo root for directories with dots.lua or path.yaml.
  -- Examples:
  --   module_paths = "modules/"           -- scan only modules/
  --   module_paths = {"pkgs/", "cfg/"}    -- scan multiple directories

  -- plugins: optional list of built-in plugins to load.
  -- Available: dots.http, dots.archive, dots.git
  -- plugins = { "dots.http", "dots.archive", "dots.git" },
}
`, repoName)
}

// InitLuaTemplateMinimal is a minimal init.lua template for empty repos.
const InitLuaTemplateMinimal = `-- init.lua — Root configuration for dots
return {
  name = "dotfiles",
}
`

// MigrateModule migrates a single module from path.yaml to dots.lua.
// Returns the generated Lua content or an error.
func MigrateModule(modPath string) (string, error) {
	yamlPath := filepath.Join(modPath, "path.yaml")
	luaPath := filepath.Join(modPath, "dots.lua")

	// Check if dots.lua already exists
	if _, err := os.Stat(luaPath); err == nil {
		return "", fmt.Errorf("dots.lua already exists in %s", filepath.Base(modPath))
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return "", fmt.Errorf("reading path.yaml: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parsing YAML: %w", err)
	}

	if raw == nil {
		return "", fmt.Errorf("empty path.yaml")
	}

	return generateLuaFromYAML(raw, filepath.Base(modPath))
}// generateLuaFromYAML converts a parsed path.yaml dict to dots.lua Lua content.
func generateLuaFromYAML(raw map[string]interface{}, modName string) (string, error) {
	var b strings.Builder

	b.WriteString("-- dots.lua — generated from path.yaml\n")
	b.WriteString("-- Module: ")
	b.WriteString(modName)
	b.WriteString("\n")
	b.WriteString("return {\n")

	// Type
	if typ, ok := raw["type"].(string); ok && typ != "" {
		fmt.Fprintf(&b, "  type = %q,\n", typ)
	}

	// Files
	if filesRaw, ok := raw["files"].([]interface{}); ok && len(filesRaw) > 0 {
		b.WriteString("\n  files = {\n")
		for _, f := range filesRaw {
			if fMap, ok := f.(map[string]interface{}); ok {
				line := generateFileEntry(fMap)
				b.WriteString("    ")
				b.WriteString(line)
				b.WriteString(",\n")
			}
		}
		b.WriteString("  },\n")
	}

	// Dependencies
	if depsRaw, ok := raw["dependencies"].([]interface{}); ok && len(depsRaw) > 0 {
		b.WriteString("\n  dependencies = {\n")
		for _, d := range depsRaw {
			switch v := d.(type) {
			case string:
				fmt.Fprintf(&b, "    pkg %q,\n", v)
			case map[string]interface{}:
				line := generateDepEntry(v)
				b.WriteString("    ")
				b.WriteString(line)
				b.WriteString(",\n")
			}
		}
		b.WriteString("  },\n")
	}

	b.WriteString("}\n")

	return b.String(), nil
}

// generateFileEntry converts a YAML file mapping to a Lua file() line.
// modPath is the full path to the module directory, used for file existence checks.
func generateFileEntry(f map[string]interface{}) string {
	source := getYAMLString(f, "source")
	dest := getYAMLString(f, "destination")
	osFilter := f["os"]

	// Check for directory expansion (/* in destination or directory source)
	destIsExpand := strings.HasSuffix(dest, "/*")

	// Per-OS destinations
	perOS, hasPerOS := f["per-os"].(map[string]interface{})

	// Determine the operation type based on source/destination patterns
	if isDirLike(source) {
		if destIsExpand {
			// dir():into() — expand directory contents into destination
			cleanDest := strings.TrimSuffix(strings.TrimSuffix(dest, "/*"), "/")
			if hasPerOS {
				firstDest := cleanDest
				for _, v := range perOS {
					if s, ok := v.(string); ok {
						firstDest = strings.TrimSuffix(strings.TrimSuffix(s, "/*"), "/")
						break
					}
				}
				return fmt.Sprintf("-- TODO: per-OS dir expansion not directly supported\n  dir(%q):into(%q)", source, firstDest)
			}
			return fmt.Sprintf("dir(%q):into(%q)", source, cleanDest)
		}
		// dir():to() — symlink entire directory to destination
		return fmt.Sprintf("dir(%q):to(%q)", source, dest)
	}

	// Regular file
	if hasPerOS {
		return fmt.Sprintf("file(%q, %q):per_os(%s)", source, dest, formatPerOSTable(perOS))
	}

	// OS filter — handles both string form ("os: linux") and list form ("os: [linux]")
	var osStr string
	switch v := osFilter.(type) {
	case string:
		osStr = v
	case []interface{}:
		if len(v) > 0 {
			osStr = getYAMLStringVal(v[0])
		}
	}
	if osStr != "" && !strings.HasPrefix(osStr, "file(") {
		return fmt.Sprintf("file(%q, %q):when(%q)", source, dest, osStr)
	}

	return fmt.Sprintf("file(%q, %q)", source, dest)
}

// generateDepEntry converts a YAML dependency dict to a Lua dep expression.
func generateDepEntry(d map[string]interface{}) string {
	depType := getYAMLString(d, "type", "package")
	name := getYAMLString(d, "name")
	url := getYAMLString(d, "url")
	dest := getYAMLString(d, "dest")
	ref := getYAMLString(d, "ref")
	extract := getYAMLString(d, "extract")
	bin := getYAMLString(d, "bin")
	postInstall := getYAMLString(d, "post-install")
	version := getYAMLString(d, "version")
	managers, hasManagers := d["managers"].(map[string]interface{})
	arch, hasArch := d["arch"].(map[string]interface{})
	fallback, hasFallback := d["fallback"].(map[string]interface{})

	switch depType {
	case "git":
		expr := fmt.Sprintf("git(%q)", url)
		if name != "" {
			expr += fmt.Sprintf(":bin(%q)", name)
		}
		if dest != "" {
			expr += fmt.Sprintf(":to(%q)", dest)
		}
		if ref != "" {
			expr += fmt.Sprintf(":at(%q)", ref)
		}
		if postInstall != "" {
			expr += fmt.Sprintf(":post(%q)", postInstall)
		}
		return expr

	case "binary":
		expr := fmt.Sprintf("curl(%q)", url)
		if name != "" {
			expr += fmt.Sprintf(":bin(%q)", name)
		}
		if extract != "" {
			expr += fmt.Sprintf(":extract(%q)", extract)
		}
		if dest != "" {
			expr += fmt.Sprintf(":to(%q)", dest)
		}
		if hasArch {
			expr += fmt.Sprintf(":arch(%s)", formatMapTable(arch))
		}
		if version != "" {
			expr += fmt.Sprintf(":version(%q)", version)
		}
		return expr

	default: // package
		if !hasManagers && bin == "" && postInstall == "" && !hasFallback {
			return fmt.Sprintf("pkg %q", name)
		}

		expr := fmt.Sprintf("pkg(%q)", name)
		if hasManagers {
			expr += fmt.Sprintf(":on(%s)", formatMapTable(managers))
		}
		if bin != "" {
			expr += fmt.Sprintf(":bin(%q)", bin)
		}
		if postInstall != "" {
			expr += fmt.Sprintf(":post(%q)", postInstall)
		}
		if hasFallback {
			fbExpr := generateDepEntry(fallback)
			expr += fmt.Sprintf(":fallback(%s)", fbExpr)
		}
		return expr
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func getYAMLString(m map[string]interface{}, key string, defaults ...string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

func getYAMLStringVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func formatPerOSTable(m map[string]interface{}) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s = %q", k, m[k]))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

func formatMapTable(m map[string]interface{}) string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s = %q", k, m[k]))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

// knownFileNames is a blocklist of common filenames without extensions that are
// definitely files, not directories. Used to prevent isDirLike false positives.
var knownFileNames = map[string]bool{
	"Makefile":   true,
	"README":     true,
	"LICENSE":    true,
	"Dockerfile": true,
	"CHANGELOG":  true,
	"Vagrantfile": true,
	"TODO":       true,
	"NOTES":      true,
	"COPYING":    true,
}

// isDirLike checks if a source path looks like a directory.
// Uses extension heuristic since we don't have the full module path here.
// Migration is best-effort; users can adjust the generated Lua output.
func isDirLike(source string) bool {
	// Known files without extensions that should NOT be treated as directories
	if knownFileNames[source] {
		return false
	}

	ext := filepath.Ext(source)
	return ext == "" && !strings.Contains(source, ".") && !strings.Contains(source, "*") && !strings.Contains(source, "?")
}

// WriteLuaModule writes the Lua config to dots.lua in the module directory.
func WriteLuaModule(modPath, luaContent string) error {
	luaPath := filepath.Join(modPath, "dots.lua")
	return os.WriteFile(luaPath, []byte(luaContent), 0644)
}

// WriteInitLua writes init.lua in the repo root.
func WriteInitLua(repoRoot, content string) error {
	initPath := filepath.Join(repoRoot, "init.lua")
	return os.WriteFile(initPath, []byte(content), 0644)
}

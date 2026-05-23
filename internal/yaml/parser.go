package yaml

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// DotFileMapping is an immutable source → destination mapping.
type DotFileMapping struct {
	Source      string
	Destination string
}

// Dependency represents a dependency to be installed.
type Dependency struct {
	Name        string
	Type        string // "package", "git", "binary"
	URL         string
	Dest        string
	Version     string
	Ref         string
	Arch        map[string]string
	Managers    map[string]string
	Extract     string
	PostInstall string
	Bin         string
	Fallback    *Dependency
}

// VariantInfo holds variant detection results.
type VariantInfo struct {
	HasVariants        bool
	Variants           []string          // source names that share destinations
	DefaultVariant     string            // last variant (cascade)
	VariantDestinations map[string]string // source → destination mapping
}

// ParsePathYAML reads a path.yaml file and returns file mappings for the current OS.
func ParsePathYAML(yamlPath, currentOS string) ([]DotFileMapping, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}

	if raw == nil {
		return nil, nil
	}

	// Early fail on v2 schema
	if v2Errors := DetectV2Schema(raw, yamlPath); len(v2Errors) > 0 {
		return nil, nil
	}

	filesRaw, ok := raw["files"].([]interface{})
	if !ok {
		return nil, nil
	}

	var mappings []DotFileMapping
	for _, item := range filesRaw {
		f, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		source, _ := f["source"].(string)
		if source == "" {
			continue
		}

		// OS filtering at item level
		if osListRaw, ok := f["os"].([]interface{}); ok {
			found := false
			for _, o := range osListRaw {
				if o.(string) == currentOS {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Resolve destination: per-os > destination
		var dest string
		if perOSRaw, ok := f["per-os"].(map[string]interface{}); ok {
			if d, ok := perOSRaw[currentOS].(string); ok {
				dest = d
			}
		}
		if dest == "" {
			dest, _ = f["destination"].(string)
		}
		if dest == "" {
			continue
		}

		source = strings.TrimRight(source, "/")
		mappings = append(mappings, DotFileMapping{
			Source:      source,
			Destination: dest,
		})
	}

	return mappings, nil
}

// ParseDependencies returns dependencies from a path.yaml file.
func ParseDependencies(yamlPath string) ([]Dependency, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}

	if raw == nil {
		return nil, nil
	}

	// Early fail on v2 schema
	if v2Errors := DetectV2Schema(raw, yamlPath); len(v2Errors) > 0 {
		return nil, nil
	}

	depsRaw, ok := raw["dependencies"].([]interface{})
	if !ok {
		return nil, nil
	}

	var deps []Dependency
	seen := make(map[string]bool)

	for _, d := range depsRaw {
		switch v := d.(type) {
		case string:
			// Legacy string format → Package
			if !seen[v] {
				deps = append(deps, Dependency{Name: v, Type: "package"})
				seen[v] = true
			}
		case map[string]interface{}:
			dep := parseSingleDependency(v)
			if dep.Name != "" && !seen[dep.Name] {
				deps = append(deps, dep)
				seen[dep.Name] = true
			}
		}
	}

	return deps, nil
}

func parseSingleDependency(raw map[string]interface{}) Dependency {
	dep := Dependency{
		Name:        getString(raw, "name"),
		Type:        getString(raw, "type", "package"),
		URL:         getString(raw, "url"),
		Dest:        getString(raw, "dest"),
		Version:     getString(raw, "version"),
		Ref:         getString(raw, "ref"),
		Extract:     getString(raw, "extract"),
		PostInstall: getString(raw, "post-install"),
		Bin:         getString(raw, "bin"),
	}

	if dep.PostInstall == "" {
		dep.PostInstall = getString(raw, "post_install")
	}

	if archRaw, ok := raw["arch"].(map[string]interface{}); ok {
		dep.Arch = make(map[string]string)
		for k, v := range archRaw {
			dep.Arch[k] = fmt.Sprintf("%v", v)
		}
	}

	if mgrsRaw, ok := raw["managers"].(map[string]interface{}); ok {
		dep.Managers = make(map[string]string)
		for k, v := range mgrsRaw {
			dep.Managers[k] = fmt.Sprintf("%v", v)
		}
	}

	if fallbackRaw, ok := raw["fallback"].(map[string]interface{}); ok {
		fb := parseSingleDependency(fallbackRaw)
		dep.Fallback = &fb
	}

	return dep
}

// DetectVariants detects variant configurations in mappings.
func DetectVariants(mappings []DotFileMapping) VariantInfo {
	if len(mappings) == 0 {
		return VariantInfo{}
	}

	destToSources := make(map[string][]string)
	for _, m := range mappings {
		key := variantKey(m.Source)
		destToSources[m.Destination] = append(destToSources[m.Destination], key)
	}

	allVariants := make([]string, 0)
	variantDests := make(map[string]string)

	for dest, sources := range destToSources {
		if len(sources) > 1 {
			for _, src := range sources {
				allVariants = append(allVariants, src)
				variantDests[src] = dest
			}
		}
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	orderedVariants := make([]string, 0)
	for _, v := range allVariants {
		if !seen[v] {
			orderedVariants = append(orderedVariants, v)
			seen[v] = true
		}
	}

	if len(orderedVariants) == 0 {
		return VariantInfo{}
	}

	defaultVariant := orderedVariants[len(orderedVariants)-1]
	return VariantInfo{
		HasVariants:        true,
		Variants:           orderedVariants,
		DefaultVariant:     defaultVariant,
		VariantDestinations: variantDests,
	}
}

// FilterByVariant filters mappings to only include a specific variant source.
func FilterByVariant(mappings []DotFileMapping, variant string) []DotFileMapping {
	if variant == "" {
		return mappings
	}

	variantNorm := strings.TrimRight(variant, "/")
	var result []DotFileMapping
	for _, m := range mappings {
		if normalizedSource(m.Source) == variantNorm {
			result = append(result, m)
		}
	}
	return result
}

func variantKey(source string) string {
	if idx := strings.Index(source, "*"); idx != -1 {
		return strings.TrimRight(source[:idx], "/")
	}
	return source
}

func normalizedSource(source string) string {
	return strings.TrimRight(source, "/*")
}

func getString(m map[string]interface{}, key string, defaults ...string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if len(defaults) > 0 {
		return defaults[0]
	}
	return ""
}

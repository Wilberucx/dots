package template

import (
	"regexp"
	"runtime"
	"strings"
)

var placeholderRe = regexp.MustCompile(`\{\{[^}]+}}`)

// GetSystemArch detects the system architecture.
// Returns "x86_64", "aarch64", or the raw machine value.
func GetSystemArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

// ResolveArch translates architecture using an arch map if provided.
func ResolveArch(archMap map[string]string) string {
	raw := GetSystemArch()
	if archMap != nil {
		if mapped, ok := archMap[raw]; ok {
			return mapped
		}
	}
	return raw
}

// Render replaces {{key}} placeholders with values from the context map.
// Any remaining unresolved placeholders (missing keys) are removed.
func Render(tmpl string, context map[string]string) string {
	result := tmpl
	for key, value := range context {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	// Remove any remaining unresolved placeholders
	result = placeholderRe.ReplaceAllString(result, "")
	return result
}

// BuildContext builds a template context with arch and optional version.
func BuildContext(version string, archMap map[string]string) map[string]string {
	return map[string]string{
		"arch":    ResolveArch(archMap),
		"version": version,
	}
}

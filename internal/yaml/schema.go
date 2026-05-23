package yaml

import "fmt"

// V2DepFields are fields that indicate an obsolete v2 schema for dependencies.
var V2DepFields = map[string]bool{
	"source": true, "target": true, "extract-path": true,
	"arch_map": true, "package-managers": true,
}

// V2FileFields are fields that indicate an obsolete v2 schema for files.
var V2FileFields = map[string]bool{
	"destination-linux": true, "destination-mac": true,
	"destination-override": true,
}

// RequiredFieldsByType maps dependency types to their required fields.
var RequiredFieldsByType = map[string][]string{
	"binary":  {"url", "dest"},
	"git":     {"url", "dest"},
	"package": {"name"},
}

// DetectV2Schema checks if a YAML data dict uses v2 schema.
func DetectV2Schema(data map[string]interface{}, yamlPath string) []string {
	var errors []string

	// Check dependencies
	if deps, ok := data["dependencies"].([]interface{}); ok {
		for i, dep := range deps {
			if d, ok := dep.(map[string]interface{}); ok {
				for field := range d {
					if V2DepFields[field] {
						errors = append(errors, fmt.Sprintf(
							"Schema v2 detected in dependencies[%d] (%s). Run 'dots migrate' to upgrade to v3 automatically.",
							i, yamlPath,
						))
						break
					}
				}
			}
		}
	}

	// Check files
	if files, ok := data["files"].([]interface{}); ok {
		for i, f := range files {
			if file, ok := f.(map[string]interface{}); ok {
				for field := range file {
					if V2FileFields[field] {
						errors = append(errors, fmt.Sprintf(
							"Schema v2 detected in files[%d] (%s). Run 'dots migrate' to upgrade to v3 automatically.",
							i, yamlPath,
						))
						break
					}
				}
			}
		}
	}

	return errors
}

// ValidateDependency validates a single dependency dict.
func ValidateDependency(raw map[string]interface{}, yamlPath string) []string {
	var errors []string
	depType, _ := raw["type"].(string)
	if depType == "" {
		depType = "package"
	}
	depName, _ := raw["name"].(string)
	if depName == "" {
		depName = "<unnamed>"
	}
	prefix := fmt.Sprintf("[%s] dependency '%s'", yamlPath, depName)

	// Valid type
	if _, ok := RequiredFieldsByType[depType]; !ok {
		knownTypes := make([]string, 0, len(RequiredFieldsByType))
		for t := range RequiredFieldsByType {
			knownTypes = append(knownTypes, t)
		}
		errors = append(errors, fmt.Sprintf(
			"%s: type '%s' unknown (known: %v)", prefix, depType, knownTypes,
		))
	}

	// Required fields by type
	if fields, ok := RequiredFieldsByType[depType]; ok {
		for _, field := range fields {
			if v, _ := raw[field].(string); v == "" {
				errors = append(errors, fmt.Sprintf(
					"%s: required field '%s' missing for type '%s'", prefix, field, depType,
				))
			}
		}
	}

	return errors
}

// ValidateFileMapping validates a single file mapping dict.
func ValidateFileMapping(raw map[string]interface{}, yamlPath string) []string {
	var errors []string
	source, _ := raw["source"].(string)
	if source == "" {
		source = "<unnamed>"
	}
	prefix := fmt.Sprintf("[%s] file mapping '%s'", yamlPath, source)

	if raw["source"] == nil || raw["source"].(string) == "" {
		errors = append(errors, fmt.Sprintf("%s: missing 'source'", prefix))
	}

	_, hasDest := raw["destination"].(string)
	_, hasPerOS := raw["per-os"].(map[string]interface{})
	if !hasDest && !hasPerOS {
		errors = append(errors, fmt.Sprintf("%s: no 'destination' or 'per-os'", prefix))
	}

	if perOS, ok := raw["per-os"]; ok {
		if _, ok := perOS.(map[string]interface{}); !ok {
			errors = append(errors, fmt.Sprintf("%s: 'per-os' must be a dict", prefix))
		}
	}

	if osFilter, ok := raw["os"]; ok {
		if _, ok := osFilter.([]interface{}); !ok {
			errors = append(errors, fmt.Sprintf("%s: 'os' must be a list", prefix))
		}
	}

	return errors
}

// ValidatePathYAML validates a complete path.yaml structure.
func ValidatePathYAML(data map[string]interface{}, yamlPath string) []string {
	var errors []string

	if data == nil {
		return []string{fmt.Sprintf("[%s]: must be a dict", yamlPath)}
	}

	// Validate dependencies
	if deps, ok := data["dependencies"].([]interface{}); ok {
		for _, dep := range deps {
			if d, ok := dep.(map[string]interface{}); ok {
				errors = append(errors, ValidateDependency(d, yamlPath)...)
			}
		}
	}

	// Validate files
	if files, ok := data["files"].([]interface{}); ok {
		for _, f := range files {
			if file, ok := f.(map[string]interface{}); ok {
				errors = append(errors, ValidateFileMapping(file, yamlPath)...)
			}
		}
	}

	return errors
}

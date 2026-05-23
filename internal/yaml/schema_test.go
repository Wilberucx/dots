package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectV2Schema_NoV2(t *testing.T) {
	data := map[string]interface{}{
		"files": []interface{}{
			map[string]interface{}{
				"source":      "file.conf",
				"destination": "~/.config/file.conf",
			},
		},
	}
	errors := DetectV2Schema(data, "test/path.yaml")
	assert.Empty(t, errors)
}

func TestDetectV2Schema_DepsV2(t *testing.T) {
	data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name":   "fd",
				"type":   "binary",
				"source": "https://example.com/fd.tar.gz", // v2 field
			},
		},
	}
	errors := DetectV2Schema(data, "test/path.yaml")
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0], "Schema v2 detected")
	assert.Contains(t, errors[0], "dependencies[0]")
}

func TestDetectV2Schema_FileV2(t *testing.T) {
	data := map[string]interface{}{
		"files": []interface{}{
			map[string]interface{}{
				"source":            "file.conf",
				"destination-linux": "~/.config/file.conf", // v2 field
			},
		},
	}
	errors := DetectV2Schema(data, "test/path.yaml")
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0], "Schema v2 detected")
	assert.Contains(t, errors[0], "files[0]")
}

func TestValidateDependency_Valid(t *testing.T) {
	raw := map[string]interface{}{
		"name": "fd",
		"type": "binary",
		"url":  "https://example.com/fd.tar.gz",
		"dest": "~/.local/bin/fd",
	}
	errors := ValidateDependency(raw, "test/path.yaml")
	assert.Empty(t, errors)
}

func TestValidateDependency_MissingRequired(t *testing.T) {
	raw := map[string]interface{}{
		"name": "fd",
		"type": "binary",
		// missing url and dest
	}
	errors := ValidateDependency(raw, "test/path.yaml")
	assert.Len(t, errors, 2)
}

func TestValidateDependency_UnknownType(t *testing.T) {
	raw := map[string]interface{}{
		"name": "foo",
		"type": "unknown-type",
	}
	errors := ValidateDependency(raw, "test/path.yaml")
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0], "type 'unknown-type' unknown")
}

func TestValidateFileMapping_Valid(t *testing.T) {
	raw := map[string]interface{}{
		"source":      "init.lua",
		"destination": "~/.config/nvim/init.lua",
	}
	errors := ValidateFileMapping(raw, "test/path.yaml")
	assert.Empty(t, errors)
}

func TestValidateFileMapping_NoSource(t *testing.T) {
	raw := map[string]interface{}{
		"destination": "~/.config/file",
	}
	errors := ValidateFileMapping(raw, "test/path.yaml")
	assert.NotEmpty(t, errors)
}

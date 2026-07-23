package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wilberucx/dots/internal/config"
)

// ─── resolveStatusFormat ─────────────────────────────────────────────────────.

func TestResolveStatusFormat_UsesConfigDefault(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Status: "table"},
		},
	}
	result := resolveStatusFormat("default", cfg)
	assert.Equal(t, "table", result)
}

func TestResolveStatusFormat_FallbackToDefault(t *testing.T) {
	// Config without InitCfg
	assert.Equal(t, "default", resolveStatusFormat("default", &config.DotsConfig{}))

	// Nil config
	assert.Equal(t, "default", resolveStatusFormat("default", nil))

	// InitCfg without Output
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{},
	}
	assert.Equal(t, "default", resolveStatusFormat("default", cfg))

	// InitCfg with Output but empty Status
	cfg2 := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{},
		},
	}
	assert.Equal(t, "default", resolveStatusFormat("default", cfg2))
}

func TestResolveStatusFormat_ExplicitFlagOverridesConfig(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Status: "table"},
		},
	}
	assert.Equal(t, "json", resolveStatusFormat("json", cfg))
	assert.Equal(t, "porcelain", resolveStatusFormat("porcelain", cfg))
	assert.Equal(t, "table", resolveStatusFormat("table", cfg))
}

func TestResolveStatusFormat_PorcelainFromFlag(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Status: "table"},
		},
	}
	result := resolveStatusFormat("porcelain", cfg)
	assert.Equal(t, "porcelain", result, "--porcelain flag should always win")
}

func TestResolveStatusFormat_AllValidFormats(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Status: "json"},
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"default", "json"},        // uses config default
		{"table", "table"},         // explicit
		{"json", "json"},           // explicit
		{"porcelain", "porcelain"}, // explicit
	}

	for _, tt := range tests {
		result := resolveStatusFormat(tt.input, cfg)
		assert.Equal(t, tt.expected, result, "resolveStatusFormat(%q) should be %q", tt.input, tt.expected)
	}
}

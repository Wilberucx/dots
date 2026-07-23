package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wilberucx/dots/internal/config"
)

// ─── resolvePlanFormat ───────────────────────────────────────────────────.

func TestResolvePlanFormat_UsesConfigDefault(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Plan: "json"},
		},
	}
	result := resolvePlanFormat("default", cfg)
	assert.Equal(t, "json", result)
}

func TestResolvePlanFormat_FallbackToDefault(t *testing.T) {
	// Config without InitCfg
	assert.Equal(t, "default", resolvePlanFormat("default", &config.DotsConfig{}))

	// Nil config
	assert.Equal(t, "default", resolvePlanFormat("default", nil))

	// InitCfg without Output
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{},
	}
	assert.Equal(t, "default", resolvePlanFormat("default", cfg))

	// InitCfg with Output but empty Plan
	cfg2 := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{},
		},
	}
	assert.Equal(t, "default", resolvePlanFormat("default", cfg2))
}

func TestResolvePlanFormat_ExplicitFlagOverridesConfig(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Plan: "json"},
		},
	}
	assert.Equal(t, "table", resolvePlanFormat("table", cfg))
	assert.Equal(t, "json", resolvePlanFormat("json", cfg))
	assert.Equal(t, "porcelain", resolvePlanFormat("porcelain", cfg))
}

func TestResolvePlanFormat_AllValidFormats(t *testing.T) {
	cfg := &config.DotsConfig{
		InitCfg: &config.RootConfig{
			Output: &config.OutputConfig{Plan: "table"},
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"default", "table"},         // uses config default
		{"table", "table"},           // explicit
		{"json", "json"},             // explicit
		{"porcelain", "porcelain"},   // explicit
	}

	for _, tt := range tests {
		result := resolvePlanFormat(tt.input, cfg)
		assert.Equal(t, tt.expected, result, "resolvePlanFormat(%q) should be %q", tt.input, tt.expected)
	}
}

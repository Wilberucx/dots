package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	ctx := map[string]string{
		"arch":    "x86_64",
		"version": "v8.7.0",
	}

	result := Render("fd-{{version}}-{{arch}}.tar.gz", ctx)
	assert.Equal(t, "fd-v8.7.0-x86_64.tar.gz", result)
}

func TestRender_MultiplePlaceholders(t *testing.T) {
	ctx := map[string]string{
		"arch":    "aarch64",
		"version": "v1.0.0",
	}

	result := Render("{{version}}/binary-{{arch}}.tar.gz", ctx)
	assert.Equal(t, "v1.0.0/binary-aarch64.tar.gz", result)
}

func TestRender_NoPlaceholders(t *testing.T) {
	ctx := map[string]string{
		"arch": "x86_64",
	}

	result := Render("static-url.tar.gz", ctx)
	assert.Equal(t, "static-url.tar.gz", result)
}

func TestRender_MissingKey(t *testing.T) {
	ctx := map[string]string{
		"arch": "x86_64",
	}

	result := Render("{{version}}-package.tar.gz", ctx)
	assert.Equal(t, "-package.tar.gz", result)
}

func TestBuildContext(t *testing.T) {
	archMap := map[string]string{
		"x86_64": "amd64",
	}
	ctx := BuildContext("v1.0.0", archMap)
	assert.Equal(t, "v1.0.0", ctx["version"])
	assert.NotEmpty(t, ctx["arch"])
}

func TestBuildContext_NoVersion(t *testing.T) {
	ctx := BuildContext("", nil)
	assert.Equal(t, "", ctx["version"])
	assert.NotEmpty(t, ctx["arch"])
}

func TestResolveArch(t *testing.T) {
	archMap := map[string]string{
		"x86_64": "amd64",
		"aarch64": "arm64",
	}
	arch := ResolveArch(archMap)
	assert.NotEmpty(t, arch)
}

func TestResolveArch_NoMap(t *testing.T) {
	arch := ResolveArch(nil)
	assert.NotEmpty(t, arch) // Should return raw system arch
}

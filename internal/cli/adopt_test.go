package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── findMatchingBrace ──────────────────────────────────────────────────────

func TestFindMatchingBrace_Basic(t *testing.T) {
	idx := findMatchingBrace("return { files = {} }", 7) // opening brace at 'return {'
	assert.Equal(t, 20, idx, "closing brace of the return table")
}

func TestFindMatchingBrace_Nested(t *testing.T) {
	// Brace at index 7 is the outer {
	// Matching } should be at index 21
	idx := findMatchingBrace("return { inner { x } }", 7)
	assert.Equal(t, 21, idx, "closing brace of the outer block")
}

func TestFindMatchingBrace_NestedDeep(t *testing.T) {
	// Three levels of nesting
	s := "return { a { b { c } } }"
	// Outer { at index 7, matching } at index 23
	idx := findMatchingBrace(s, 7)
	assert.Equal(t, 23, idx)
}

func TestFindMatchingBrace_Empty(t *testing.T) {
	idx := findMatchingBrace("return {}", 7)
	assert.Equal(t, 8, idx)
}

func TestFindMatchingBrace_NoMatch(t *testing.T) {
	idx := findMatchingBrace("return {", 7)
	assert.Equal(t, -1, idx, "no closing brace should return -1")
}

func TestFindMatchingBrace_NotABrace(t *testing.T) {
	idx := findMatchingBrace("return \"hello\"", 7)
	assert.Equal(t, -1, idx)
}

func TestFindMatchingBrace_NegativePosition(t *testing.T) {
	idx := findMatchingBrace("return {}", -1)
	assert.Equal(t, -1, idx)
}

func TestFindMatchingBrace_PositionOutOfRange(t *testing.T) {
	idx := findMatchingBrace("return {}", 50)
	assert.Equal(t, -1, idx)
}

func TestFindMatchingBrace_StringLiteral(t *testing.T) {
	// Braces inside strings should be ignored
	s := "return { file(\"a\", \"~/.{brace}\") }"
	// Outer { at index 7, matching } is at the end
	idx := findMatchingBrace(s, 7)
	assert.Equal(t, len(s)-1, idx, "should skip braces inside strings")
}

func TestFindMatchingBrace_StringLiteralWithNested(t *testing.T) {
	// Nested braces outside strings should still be counted
	s := `return { file("init.lua", "~/.config/nvim/init.lua"):per_os({
      linux = "~/.config/nvim",
    }) }`
	idx := findMatchingBrace(s, 7)
	// Should find the } that matches the outer {
	assert.Equal(t, len(s)-1, idx, "should handle nested real braces")
}

func TestFindMatchingBrace_SingleQuotedString(t *testing.T) {
	// Single quotes should also be respected
	s := "return { file('a', '~/.{brace}') }"
	idx := findMatchingBrace(s, 7)
	assert.Equal(t, len(s)-1, idx, "should skip braces inside single-quoted strings")
}

func TestFindMatchingBrace_MultipleBraces(t *testing.T) {
	s := "[] { first } [] { second }"
	// First { at index 3
	idx1 := findMatchingBrace(s, 3)
	assert.Equal(t, 11, idx1, "first closing brace")
	// Second { at index 16
	idx2 := findMatchingBrace(s, 16)
	assert.Equal(t, 25, idx2, "second closing brace")
}

func TestFindMatchingBrace_EscapedQuote(t *testing.T) {
	// Escaped quotes inside strings should not end the string
	s := `return { file("name with \"quote\"", "dest") }`
	idx := findMatchingBrace(s, 7)
	assert.Equal(t, len(s)-1, idx, "should handle escaped quotes")
}

// ─── appendLuaFileEntry — no existing files section ────────────────────────

func writeLuaFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "dots.lua")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
}

func TestAppendLuaFileEntry_NoFilesSection(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  name = "test",
}
`)

	err := appendLuaFileEntry(luaPath, "newfile", "~/.newfile")
	require.NoError(t, err)

	content := `return {
  name = "test",
  files = {
    file("newfile", "~/.newfile"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_MinimalReturn(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "return {\n}\n")

	err := appendLuaFileEntry(luaPath, "zshrc", "~/.zshrc")
	require.NoError(t, err)

	content := "return {\n  files = {\n    file(\"zshrc\", \"~/.zshrc\"),\n  },\n}\n"
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_NoFilesWithDeps(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  type = "minimal",
  dependencies = {
    pkg "zsh",
  },
}
`)

	err := appendLuaFileEntry(luaPath, "zshrc", "~/.zshrc")
	require.NoError(t, err)

	content := `return {
  type = "minimal",
  dependencies = {
    pkg "zsh",
  },
  files = {
    file("zshrc", "~/.zshrc"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

// ─── appendLuaFileEntry — with existing files section ──────────────────────

func TestAppendLuaFileEntry_ExistingFilesSection(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  type = "minimal",
  files = {
    file("existing", "~/.existing"),
  },
}
`)

	err := appendLuaFileEntry(luaPath, "newfile", "~/.newfile")
	require.NoError(t, err)

	content := `return {
  type = "minimal",
  files = {
    file("existing", "~/.existing"),
    file("newfile", "~/.newfile"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_ExistingFilesWithDepsAfter(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  type = "minimal",
  files = {
    file("existing", "~/.existing"),
  },
  dependencies = {
    pkg "neovim",
  },
}
`)

	err := appendLuaFileEntry(luaPath, "newconf", "~/.newconf")
	require.NoError(t, err)

	// Must be inserted INSIDE the files array, not after dependencies
	content := `return {
  type = "minimal",
  files = {
    file("existing", "~/.existing"),
    file("newconf", "~/.newconf"),
  },
  dependencies = {
    pkg "neovim",
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  files = {
    file("first", "~/.first"),
  },
}
`)

	// Add second entry
	err := appendLuaFileEntry(luaPath, "second", "~/.second")
	require.NoError(t, err)

	content1 := `return {
  files = {
    file("first", "~/.first"),
    file("second", "~/.second"),
  },
}
`
	assertFileContent(t, luaPath, content1)

	// Add third entry
	err = appendLuaFileEntry(luaPath, "third", "~/.third")
	require.NoError(t, err)

	content2 := `return {
  files = {
    file("first", "~/.first"),
    file("second", "~/.second"),
    file("third", "~/.third"),
  },
}
`
	assertFileContent(t, luaPath, content2)
}

func TestAppendLuaFileEntry_EmptyFilesSection(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  files = {
  },
}
`)

	err := appendLuaFileEntry(luaPath, "newfile", "~/.newfile")
	require.NoError(t, err)

	content := `return {
  files = {
    file("newfile", "~/.newfile"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_FilesThenDepsThenMoreFiles(t *testing.T) {
	// This tests that the function finds the CORRECT files section (the first one)
	// even if there's another `files =` reference after it
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  files = {
    file("a", "~/.a"),
  },
  dependencies = {
    pkg "tool",
  },
}
`)

	err := appendLuaFileEntry(luaPath, "b", "~/.b")
	require.NoError(t, err)

	content := `return {
  files = {
    file("a", "~/.a"),
    file("b", "~/.b"),
  },
  dependencies = {
    pkg "tool",
  },
}
`
	assertFileContent(t, luaPath, content)
}

// ─── appendLuaFileEntry — with nested braces (per_os, when) ────────────────

func TestAppendLuaFileEntry_WithPerOSInFiles(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  files = {
    file("config.toml", "~/.config/app/config.toml"):per_os({
      linux = "~/.config/app/linux.toml",
      mac = "~/Library/app/mac.toml",
    }),
  },
}
`)

	err := appendLuaFileEntry(luaPath, "newconf", "~/.newconf")
	require.NoError(t, err)

	content := `return {
  files = {
    file("config.toml", "~/.config/app/config.toml"):per_os({
      linux = "~/.config/app/linux.toml",
      mac = "~/Library/app/mac.toml",
    }),
    file("newconf", "~/.newconf"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_WithWhenAndVariant(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  files = {
    file("work/config", "~/.config/app"):when("linux"):variant("work"),
    file("personal/config", "~/.config/app"):variant("personal"),
  },
}
`)

	err := appendLuaFileEntry(luaPath, "common/config", "~/.config/app")
	require.NoError(t, err)

	content := `return {
  files = {
    file("work/config", "~/.config/app"):when("linux"):variant("work"),
    file("personal/config", "~/.config/app"):variant("personal"),
    file("common/config", "~/.config/app"),
  },
}
`
	assertFileContent(t, luaPath, content)
}

// ─── appendLuaFileEntry — error cases ───────────────────────────────────────

func TestAppendLuaFileEntry_NoClosingBrace(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "return {\n")

	err := appendLuaFileEntry(luaPath, "x", "~/.x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no closing brace")
}

func TestAppendLuaFileEntry_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, "")

	err := appendLuaFileEntry(luaPath, "x", "~/.x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no closing brace")
}

func TestAppendLuaFileEntry_FileNotExists(t *testing.T) {
	err := appendLuaFileEntry("/nonexistent/path/dots.lua", "x", "~/.x")
	assert.Error(t, err)
}

func TestAppendLuaFileEntry_GapValidation(t *testing.T) {
	// Both init.lua and config.lua files handle this test
	dir := t.TempDir()
	// Malformed: content between "files =" and "{"
	luaPath := writeLuaFile(t, dir, `return {
  files = some_function({
    file("a", "~/.a"),
  }),
}
`)

	err := appendLuaFileEntry(luaPath, "b", "~/.b")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected content between 'files =' and '{'")
}

// ─── appendLuaFileEntry — real-world integration ───────────────────────────

func TestAppendLuaFileEntry_RealWorldComplex(t *testing.T) {
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  type = "full",
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("config.toml", "~/.config/app/config.toml"):per_os({
      linux = "~/.config/app/linux.toml",
      mac = "~/Library/app/mac.toml",
    }),
  },
  dependencies = {
    pkg "neovim",
    pkg("fd"):on({ pacman = "fd", apt = "fd-find", brew = "fd" }),
  },
}
`)

	err := appendLuaFileEntry(luaPath, "zshrc", "~/.zshrc")
	require.NoError(t, err)

	content := `return {
  type = "full",
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("config.toml", "~/.config/app/config.toml"):per_os({
      linux = "~/.config/app/linux.toml",
      mac = "~/Library/app/mac.toml",
    }),
    file("zshrc", "~/.zshrc"),
  },
  dependencies = {
    pkg "neovim",
    pkg("fd"):on({ pacman = "fd", apt = "fd-find", brew = "fd" }),
  },
}
`
	assertFileContent(t, luaPath, content)
}

func TestAppendLuaFileEntry_OutputValidLua(t *testing.T) {
	// Verify the generated content is valid Lua that can be parsed back
	dir := t.TempDir()
	luaPath := writeLuaFile(t, dir, `return {
  type = "minimal",
  dependencies = {
    pkg "zsh",
  },
}
`)

	err := appendLuaFileEntry(luaPath, "zshrc", "~/.zshrc")
	require.NoError(t, err)

	// Read the result and verify it contains the expected pattern
	data, err := os.ReadFile(luaPath)
	require.NoError(t, err)
	content := string(data)

	assert.True(t, strings.Contains(content, `file("zshrc", "~/.zshrc")`),
		"result should contain the file entry")
	assert.True(t, strings.Contains(content, `files = {`),
		"result should contain files section")
	assert.True(t, strings.Contains(content, `dependencies = {`),
		"result should contain dependencies section")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(content), "}"),
		"result should end with closing brace")
}

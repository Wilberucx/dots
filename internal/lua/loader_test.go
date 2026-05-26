package lua

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── loadPlugin (Go function) ───────────────────────────────────────────────

func TestLoadPlugin_BuiltinHTTP(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	fn, err := loadPlugin(L, "http", "")
	require.NoError(t, err, "built-in http plugin should load")
	require.NotNil(t, fn)

	// Execute and verify the module table with correct API
	L.Push(fn)
	err = L.PCall(0, 1, nil)
	require.NoError(t, err)

	result := L.Get(-1)
	L.Pop(1)

	tbl, ok := result.(*lua.LTable)
	require.True(t, ok, "http plugin should return a table")
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("download").Type(), "http.download should be a function")
}

func TestLoadPlugin_BuiltinArchive(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	fn, err := loadPlugin(L, "archive", "")
	require.NoError(t, err, "built-in archive plugin should load")
	require.NotNil(t, fn)

	L.Push(fn)
	err = L.PCall(0, 1, nil)
	require.NoError(t, err)

	result := L.Get(-1)
	L.Pop(1)

	tbl, ok := result.(*lua.LTable)
	require.True(t, ok, "archive plugin should return a table")
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("extract_tar").Type(), "archive.extract_tar should be a function")
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("extract_zip").Type(), "archive.extract_zip should be a function")
}

func TestLoadPlugin_BuiltinGit(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	fn, err := loadPlugin(L, "git", "")
	require.NoError(t, err, "built-in git plugin should load")
	require.NotNil(t, fn)

	L.Push(fn)
	err = L.PCall(0, 1, nil)
	require.NoError(t, err)

	result := L.Get(-1)
	L.Pop(1)

	tbl, ok := result.(*lua.LTable)
	require.True(t, ok, "git plugin should return a table")
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("clone").Type(), "git.clone should be a function")
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("checkout").Type(), "git.checkout should be a function")
}

func TestLoadPlugin_NotFound(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	fn, err := loadPlugin(L, "nonexistent_plugin", "")
	assert.Error(t, err)
	assert.Nil(t, fn)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadPlugin_CustomPluginDir(t *testing.T) {
	dir := t.TempDir()

	dotsDir := filepath.Join(dir, "dots")
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)

	pluginContent := `return { hello = function() return "world" end }`
	err = os.WriteFile(filepath.Join(dotsDir, "custom.lua"), []byte(pluginContent), 0644)
	require.NoError(t, err)

	L := lua.NewState()
	defer L.Close()

	fn, err := loadPlugin(L, "custom", dir)
	require.NoError(t, err)
	require.NotNil(t, fn)

	L.Push(fn)
	err = L.PCall(0, 1, nil)
	require.NoError(t, err)

	result := L.Get(-1)
	L.Pop(1)

	tbl, ok := result.(*lua.LTable)
	require.True(t, ok)

	// Call the hello function via Lua
	tbl.RawSetString("hello_fn", tbl.RawGetString("hello"))
	L.SetGlobal("custom_tbl", tbl)
	err = L.DoString(`assert(custom_tbl.hello() == "world")`)
	assert.NoError(t, err)
}

func TestLoadPlugin_BuiltinTakesPriority(t *testing.T) {
	dir := t.TempDir()

	// Create a conflicting plugin in dots/ — should NOT shadow built-in
	dotsDir := filepath.Join(dir, "dots")
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)

	overrideContent := `return { download = function() return "OVERRIDE" end }`
	err = os.WriteFile(filepath.Join(dotsDir, "http.lua"), []byte(overrideContent), 0644)
	require.NoError(t, err)

	L := lua.NewState()
	defer L.Close()

	// Built-in should be found first
	fn, err := loadPlugin(L, "http", dir)
	require.NoError(t, err)
	require.NotNil(t, fn)

	L.Push(fn)
	err = L.PCall(0, 1, nil)
	require.NoError(t, err)

	result := L.Get(-1)
	L.Pop(1)

	tbl, ok := result.(*lua.LTable)
	require.True(t, ok)

	// Built-in http.download should be a function, not "OVERRIDE"
	assert.Equal(t, lua.LTFunction, tbl.RawGetString("download").Type())
}

// ─── Plugin loader (require integration) ────────────────────────────────────

func TestPluginLoader_BuiltinViaRequire(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	RegisterPluginLoader(vm.L, "")

	// Try require for http via protected call
	err := vm.L.DoString(`
		local ok, http = pcall(require, "http")
		assert(ok, "http plugin should load via require")
		assert(type(http) == "table", "http should be a table")
		assert(type(http.download) == "function", "http.download should be a function")
	`)
	assert.NoError(t, err, "http plugin should load via require")
}

func TestPluginLoader_NotFoundViaRequire(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	RegisterPluginLoader(vm.L, "")

	err := vm.L.DoString(`
		local ok, result = pcall(require, "nonexistent")
		assert(not ok, "should fail")
		assert(type(result) == "string")
	`)
	assert.NoError(t, err, "pcall should succeed")
}

func TestPluginLoader_CustomViaRequire(t *testing.T) {
	dir := t.TempDir()

	dotsDir := filepath.Join(dir, "dots")
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)

	pluginContent := `return { hello = function() return "world" end }`
	err = os.WriteFile(filepath.Join(dotsDir, "greeter.lua"), []byte(pluginContent), 0644)
	require.NoError(t, err)

	vm := NewLuaVM()
	defer vm.Close()

	RegisterPluginLoader(vm.L, dir)

	err = vm.L.DoString(`
		local ok, greeter = pcall(require, "greeter")
		assert(ok, "greeter should load")
		assert(greeter.hello() == "world")
	`)
	assert.NoError(t, err)
}

// ─── LoadModulePlugins ──────────────────────────────────────────────────────

func TestLoadModulePlugins_Empty(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	cfg := &RootConfig{
		Name:    "test",
		Plugins: []string{},
	}

	// Should not panic
	LoadModulePlugins(vm, cfg, "")
}

func TestLoadModulePlugins_AllBuiltins(t *testing.T) {
	vm := NewLuaVM()
	defer vm.Close()

	cfg := &RootConfig{
		Name:    "test",
		Plugins: []string{"dots.http", "dots.archive", "dots.git"},
	}

	// Should not panic - plugins load without error
	LoadModulePlugins(vm, cfg, "")

	// Verify they're accessible
	err := vm.L.DoString(`
		assert(type(http) == "table")
		assert(type(archive) == "table")
		assert(type(git) == "table")
	`)
	assert.NoError(t, err)
}

func TestLoadModulePlugins_CustomPlugin(t *testing.T) {
	dir := t.TempDir()

	dotsDir := filepath.Join(dir, "dots")
	err := os.MkdirAll(dotsDir, 0755)
	require.NoError(t, err)

	pluginContent := `return { greet = function(name) return "hello " .. name end }`
	err = os.WriteFile(filepath.Join(dotsDir, "greeter.lua"), []byte(pluginContent), 0644)
	require.NoError(t, err)

	vm := NewLuaVM()
	defer vm.Close()

	cfg := &RootConfig{
		Name:    "test",
		Plugins: []string{"dots.greeter"},
	}

	LoadModulePlugins(vm, cfg, dir)

	err = vm.L.DoString(`
		assert(type(greeter) == "table")
		assert(greeter.greet("world") == "hello world")
	`)
	assert.NoError(t, err)
}

// ─── Embed.FS access ────────────────────────────────────────────────────────

func TestBuiltinPluginsEmbed(t *testing.T) {
	expected := []string{"http.lua", "archive.lua", "git.lua"}

	for _, name := range expected {
		data, err := builtinPlugins.ReadFile("plugins/" + name)
		require.NoError(t, err, "expected embedded plugin: plugins/%s", name)
		assert.NotEmpty(t, data, "embedded plugin should not be empty")
	}
}

func TestBuiltinPlugins_ContentValidity(t *testing.T) {
	for _, name := range []string{"http", "archive", "git"} {
		t.Run(name, func(t *testing.T) {
			data, err := builtinPlugins.ReadFile("plugins/" + name + ".lua")
			require.NoError(t, err)

			L := lua.NewState()
			defer L.Close()

			_, err = L.LoadString(string(data))
			assert.NoError(t, err, "plugin %s should be valid Lua", name)
		})
	}
}

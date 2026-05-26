package lua

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

//go:embed plugins/*.lua
var builtinPlugins embed.FS

// RegisterPluginLoader registers a custom loader for the "require" function
// that resolves plugins in this order:
//  1. Built-in plugins embedded in the binary (dots.http, dots.archive, dots.git)
//  2. Plugins from the <repo_root>/dots/ directory
func RegisterPluginLoader(L *lua.LState, repoRoot string) {
	// Prepend our loader to package.loaders
	L.DoString(`
		if package == nil then package = {} end
		if package.loaders == nil then package.loaders = {} end
		table.insert(package.loaders, 1, function(name)
			return dots_loader(name)
		end)
	`)

	// Register the Go callback
	L.SetGlobal("dots_loader", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)

		// Try built-in plugins first
		builtinName := "plugins/" + name + ".lua"
		if data, err := builtinPlugins.ReadFile(builtinName); err == nil {
			fn, err := L.LoadString(string(data))
			if err != nil {
				L.RaiseError("built-in plugin %s: %v", name, err)
				return 0
			}
			L.Push(fn)
			return 1
		}

		// Try repo's dots/ directory — use LoadFile to return a loader function
		if repoRoot != "" {
			pluginPath := filepath.Join(repoRoot, "dots", name+".lua")
			if _, err := os.Stat(pluginPath); err == nil {
				fn, err := L.LoadFile(pluginPath)
				if err != nil {
					L.RaiseError("plugin %s (%s): %v", name, pluginPath, err)
					return 0
				}
				L.Push(fn)
				return 1
			}
		}

		// Not found
		searchedPaths := fmt.Sprintf("built-in, %s/dots/", repoRoot)
		L.RaiseError("plugin '%s' not found. Searched: %s", name, searchedPaths)
		return 0
	}))
}

// LoadModulePlugins registers all plugins from the RootConfig into the VM.
// Each plugin is loaded and its module table is set as a global variable
// under the plugin's clean name (e.g., "dots.http" → global "http").
func LoadModulePlugins(vm *LuaVM, cfg *RootConfig, repoRoot string) {
	RegisterPluginLoader(vm.L, repoRoot)

	for _, plugin := range cfg.Plugins {
		cleanName := strings.TrimPrefix(plugin, "dots.")
		fn, err := loadPlugin(vm.L, cleanName, repoRoot)
		if err != nil {
			continue
		}
		if fn != nil {
			vm.L.Push(fn)
			if err := vm.L.PCall(0, 1, nil); err == nil {
				result := vm.L.Get(-1)
				vm.L.Pop(1)
				vm.L.SetGlobal(cleanName, result)
			}
		}
	}
}

// loadPlugin attempts to load a plugin by name (built-in or filesystem).
func loadPlugin(L *lua.LState, name string, repoRoot string) (*lua.LFunction, error) {
	// Try built-in
	builtinName := "plugins/" + name + ".lua"
	if data, err := builtinPlugins.ReadFile(builtinName); err == nil {
		return L.LoadString(string(data))
	}

	// Try repo's dots/ directory
	if repoRoot != "" {
		pluginPath := filepath.Join(repoRoot, "dots", name+".lua")
		if _, err := os.Stat(pluginPath); err == nil {
			fn, err := L.LoadFile(pluginPath)
			if err != nil {
				return nil, err
			}
			return fn, nil
		}
	}

	return nil, fmt.Errorf("plugin '%s' not found", name)
}

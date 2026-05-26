package lua

import (
	lua "github.com/yuin/gopher-lua"
)

// apiPkg implements the Lua pkg() function.
// Usage:
//   pkg "ripgrep"           → string shorthand (dep type = package)
//   pkg("fd"):on({...})     → explicit table with chaining
func (vm *LuaVM) apiPkg(L *lua.LState) int {
	// Get the argument safely — handle both pkg "name" and pkg("name")
	name := ""
	if L.GetTop() >= 1 {
		val := L.Get(1)
		if val.Type() == lua.LTNil {
			L.RaiseError("pkg(): missing package name")
			return 0
		}
		if s, ok := val.(lua.LString); ok {
			name = string(s)
		} else {
			name = L.CheckString(1)
		}
	}

	if name == "" {
		L.RaiseError("pkg(): package name cannot be empty")
		return 0
	}

	// Return a table for potential chaining (e.g., pkg("fd"):on({...}))
	obj := vm.newDepOpTable("package", name)
	L.Push(obj)
	return 1
}

// apiCurl implements the Lua curl() function.
// Usage: curl(url):extract("bin"):to("~/.local/bin/x"):version("v1.0"):arch({...})
func (vm *LuaVM) apiCurl(L *lua.LState) int {
	url := L.OptString(1, "")
	if url == "" {
		L.RaiseError("curl(): URL argument is required")
		return 0
	}

	obj := vm.newDepOpTable("binary", "")
	obj.RawSetString("url", lua.LString(url))
	L.Push(obj)
	return 1
}

// apiGit implements the Lua git() function.
// Usage: git(url):to("~/plugins/p10k"):at("v1.19.0"):post("cmd")
func (vm *LuaVM) apiGit(L *lua.LState) int {
	url := L.OptString(1, "")
	if url == "" {
		L.RaiseError("git(): URL argument is required")
		return 0
	}

	obj := vm.newDepOpTable("git", "")
	obj.RawSetString("url", lua.LString(url))
	L.Push(obj)
	return 1
}

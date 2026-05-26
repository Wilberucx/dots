package lua

import (
	lua "github.com/yuin/gopher-lua"
)

// apiFile implements the Lua file() function.
// Usage: file(source, destination) → table with :when(os), :per_os(table)
func (vm *LuaVM) apiFile(L *lua.LState) int {
	source := L.CheckString(1)
	dest := L.CheckString(2)

	obj := vm.newFileOpTable("file", source, dest)
	L.Push(obj)
	return 1
}

// apiDir implements the Lua dir() function.
// Usage: dir(source) → table with :to(dest), :into(dest)
func (vm *LuaVM) apiDir(L *lua.LState) int {
	source := L.CheckString(1)

	obj := vm.newFileOpTable("dir", source, "")
	L.Push(obj)
	return 1
}

// apiGlob implements the Lua glob() function.
// Usage: glob(pattern) → table with :into(dest)
func (vm *LuaVM) apiGlob(L *lua.LState) int {
	pattern := L.CheckString(1)

	obj := vm.newFileOpTable("glob", "", "")
	obj.RawSetString("pattern", lua.LString(pattern))

	// Override the file_op_type for glob
	obj.RawSetString("file_op_type", lua.LString("glob"))
	L.Push(obj)
	return 1
}

// ─── String representation for debug ────────────────────────────────────────

// fileOpTypeString returns a human-readable name for the operation type.
func fileOpTypeString(t FileOpType) string {
	switch t {
	case FileOpFile:
		return "file"
	case FileOpDirTo:
		return "dir:to"
	case FileOpDirInto:
		return "dir:into"
	case FileOpGlob:
		return "glob"
	default:
		return "unknown"
	}
}

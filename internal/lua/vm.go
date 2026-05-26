package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// LuaVM wraps a gopher-lua LState with registered API functions.
type LuaVM struct {
	L *lua.LState
}

// NewLuaVM creates a new Lua VM with the dots API registered.
func NewLuaVM() *LuaVM {
	L := lua.NewState()

	vm := &LuaVM{L: L}

	// Register file API globally
	L.SetGlobal("file", L.NewFunction(vm.apiFile))
	L.SetGlobal("dir", L.NewFunction(vm.apiDir))
	L.SetGlobal("glob", L.NewFunction(vm.apiGlob))

	// Register dependency API globally
	L.SetGlobal("pkg", L.NewFunction(vm.apiPkg))
	L.SetGlobal("curl", L.NewFunction(vm.apiCurl))
	L.SetGlobal("git", L.NewFunction(vm.apiGit))

	return vm
}

// Close closes the Lua VM.
func (vm *LuaVM) Close() {
	vm.L.Close()
}

// LoadModuleConfig loads a dots.lua file and returns the parsed ModuleConfig.
func (vm *LuaVM) LoadModuleConfig(path string) (*ModuleConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("module config not found: %s", path)
	}

	if err := vm.L.DoFile(path); err != nil {
		return nil, fmt.Errorf("syntax error in %s: %v", filepath.Base(path), err)
	}

	// The script should return a table
	tbl, ok := vm.L.Get(-1).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s must return a table, got %T", filepath.Base(path), vm.L.Get(-1))
	}
	vm.L.Pop(1)

	return parseModuleConfig(tbl)
}

// LoadRootConfig loads an init.lua file and returns the parsed RootConfig.
func (vm *LuaVM) LoadRootConfig(path string) (*RootConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil // init.lua is optional
	}

	if err := vm.L.DoFile(path); err != nil {
		return nil, fmt.Errorf("syntax error in %s: %v", filepath.Base(path), err)
	}

	// The script should return a table
	tbl, ok := vm.L.Get(-1).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s must return a table, got %T", filepath.Base(path), vm.L.Get(-1))
	}
	vm.L.Pop(1)

	return parseRootConfig(tbl)
}

// CheckSyntax checks a Lua file for syntax errors by loading it.
func CheckSyntax(path string) error {
	vm := NewLuaVM()
	defer vm.Close()

	_, err := vm.LoadModuleConfig(path)
	return err
}

// ─── Parsing helpers ────────────────────────────────────────────────────────

// parseModuleConfig converts a Lua table (returned by dots.lua) into ModuleConfig.
func parseModuleConfig(tbl *lua.LTable) (*ModuleConfig, error) {
	cfg := &ModuleConfig{}

	// type field
	if typ := tbl.RawGetString("type"); typ != lua.LNil {
		cfg.Type = LuaValToString(typ)
	}

	// files array — each element is a table created by file(), dir(), or glob()
	if filesTbl := tbl.RawGetString("files"); filesTbl != lua.LNil {
		if tbl, ok := filesTbl.(*lua.LTable); ok {
			cfg.Files = parseFileOps(tbl)
		}
	}

	// dependencies array
	if depsTbl := tbl.RawGetString("dependencies"); depsTbl != lua.LNil {
		if tbl, ok := depsTbl.(*lua.LTable); ok {
			cfg.Dependencies = parseDepOps(tbl)
		}
	}

	return cfg, nil
}

// parseFileOps walks a Lua table array created by the file/dir/glob API.
func parseFileOps(tbl *lua.LTable) []FileOp {
	var ops []FileOp
	tbl.ForEach(func(_, val lua.LValue) {
		if item, ok := val.(*lua.LTable); ok {
			op := parseSingleFileOp(item)
			ops = append(ops, op)
		}
	})
	return ops
}

// parseSingleFileOp extracts a FileOp from a Lua table created by file()/dir()/glob().
func parseSingleFileOp(tbl *lua.LTable) FileOp {
	op := FileOp{
		Type:        FileOpFile, // default
		Source:      lvToString(tbl.RawGetString("source")),
		Destination: lvToString(tbl.RawGetString("destination")),
		Pattern:     lvToString(tbl.RawGetString("pattern")),
		OSFilter:    lvToString(tbl.RawGetString("os_filter")),
		VariantName: lvToString(tbl.RawGetString("variant_name")),
	}

	// FileOpType
	if opType := tbl.RawGetString("file_op_type"); opType != lua.LNil {
		switch opType.String() {
		case "dir_to":
			op.Type = FileOpDirTo
		case "dir_into":
			op.Type = FileOpDirInto
		case "glob":
			op.Type = FileOpGlob
		}
	}

	// PerOS
	if perOSTbl := tbl.RawGetString("per_os"); perOSTbl != lua.LNil {
		if t, ok := perOSTbl.(*lua.LTable); ok {
			op.PerOS = LuaTableToStringMap(t)
		}
	}

	return op
}

// parseDepOps walks a Lua table array created by the pkg/curl/git API.
func parseDepOps(tbl *lua.LTable) []DepOp {
	var deps []DepOp
	tbl.ForEach(func(_, val lua.LValue) {
		if item, ok := val.(*lua.LTable); ok {
			dep := parseSingleDepOp(item)
			deps = append(deps, dep)
		}
	})
	return deps
}

// parseSingleDepOp extracts a DepOp from a Lua table.
func parseSingleDepOp(tbl *lua.LTable) DepOp {
	dep := DepOp{
		Name:        lvToString(tbl.RawGetString("name")),
		Type:        lvToString(tbl.RawGetString("dep_type")),
		URL:         lvToString(tbl.RawGetString("url")),
		Destination: lvToString(tbl.RawGetString("destination")),
		Version:     lvToString(tbl.RawGetString("version")),
		Ref:         lvToString(tbl.RawGetString("ref")),
		Extract:     lvToString(tbl.RawGetString("extract")),
		Bin:         lvToString(tbl.RawGetString("bin")),
		PostInstall: lvToString(tbl.RawGetString("post_install")),
	}

	// Default dep type
	if dep.Type == "" {
		dep.Type = "package"
	}

	// Managers
	if mgrsTbl := tbl.RawGetString("managers"); mgrsTbl != lua.LNil {
		if t, ok := mgrsTbl.(*lua.LTable); ok {
			dep.Managers = LuaTableToStringMap(t)
		}
	}

	// Arch
	if archTbl := tbl.RawGetString("arch"); archTbl != lua.LNil {
		if t, ok := archTbl.(*lua.LTable); ok {
			dep.Arch = LuaTableToStringMap(t)
		}
	}

	// Fallback
	if fbTbl := tbl.RawGetString("fallback"); fbTbl != lua.LNil {
		if t, ok := fbTbl.(*lua.LTable); ok {
			fb := parseSingleDepOp(t)
			dep.Fallback = &fb
		}
	}

	return dep
}

// parseRootConfig converts a Lua table (returned by init.lua) into RootConfig.
func parseRootConfig(tbl *lua.LTable) (*RootConfig, error) {
	cfg := &RootConfig{
		Name: lvToString(tbl.RawGetString("name")),
	}

	// module_paths: string or table of strings
	if mp := tbl.RawGetString("module_paths"); mp != lua.LNil {
		switch v := mp.(type) {
		case lua.LString:
			cfg.ModulePaths = []string{string(v)}
		case *lua.LTable:
			cfg.ModulePaths = LuaTableToStringSlice(v)
		}
	}

	// plugins: table of strings
	if plugins := tbl.RawGetString("plugins"); plugins != lua.LNil {
		if t, ok := plugins.(*lua.LTable); ok {
			cfg.Plugins = LuaTableToStringSlice(t)
		}
	}

	// If name is empty, derive from the init.lua path
	if cfg.Name == "" {
		cfg.Name = "dotfiles"
	}

	return cfg, nil
}

// lvToString safely converts a Lua value to string, returning "" for nil.
func lvToString(lv lua.LValue) string {
	if lv == lua.LNil {
		return ""
	}
	return strings.TrimSpace(lv.String())
}

// ─── Helper to create a file-op table with methods ───────────────────────────

// newFileOpTable creates a Lua table representing a file operation with a metatable
// for method chaining.
func (vm *LuaVM) newFileOpTable(opType, source, destination string) *lua.LTable {
	obj := vm.L.NewTable()
	obj.RawSetString("file_op_type", lua.LString(opType))
	obj.RawSetString("source", lua.LString(source))

	if destination != "" {
		obj.RawSetString("destination", lua.LString(destination))
	}

	// Methods table for method chaining
	methods := vm.L.NewTable()
	methods.RawSetString("when", vm.L.NewFunction(vm.fileWhenMethod))
	methods.RawSetString("per_os", vm.L.NewFunction(vm.filePerOSMethod))
	methods.RawSetString("variant", vm.L.NewFunction(vm.fileVariantMethod))
	methods.RawSetString("to", vm.L.NewFunction(vm.dirToMethod))
	methods.RawSetString("into", vm.L.NewFunction(vm.dirIntoMethod))

	// Set metatable
	mt := vm.L.NewTable()
	mt.RawSetString("__index", methods)
	vm.L.SetMetatable(obj, mt)

	return obj
}

// newDepOpTable creates a Lua table representing a dependency with a metatable
// for method chaining.
func (vm *LuaVM) newDepOpTable(depType, name string) *lua.LTable {
	obj := vm.L.NewTable()
	obj.RawSetString("dep_type", lua.LString(depType))
	obj.RawSetString("name", lua.LString(name))

	// Methods table
	methods := vm.L.NewTable()
	methods.RawSetString("on", vm.L.NewFunction(vm.depOnMethod))
	methods.RawSetString("to", vm.L.NewFunction(vm.depToMethod))
	methods.RawSetString("at", vm.L.NewFunction(vm.depAtMethod))
	methods.RawSetString("extract", vm.L.NewFunction(vm.depExtractMethod))
	methods.RawSetString("version", vm.L.NewFunction(vm.depVersionMethod))
	methods.RawSetString("arch", vm.L.NewFunction(vm.depArchMethod))
	methods.RawSetString("bin", vm.L.NewFunction(vm.depBinMethod))
	methods.RawSetString("post", vm.L.NewFunction(vm.depPostMethod))
	methods.RawSetString("fallback", vm.L.NewFunction(vm.depFallbackMethod))

	// Set metatable
	mt := vm.L.NewTable()
	mt.RawSetString("__index", methods)
	vm.L.SetMetatable(obj, mt)

	return obj
}

// ─── File API method handlers ───────────────────────────────────────────────

// fileWhenMethod implements :when(os) for file() objects.
func (vm *LuaVM) fileWhenMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	os := L.CheckString(2)
	obj.RawSetString("os_filter", lua.LString(os))
	L.Push(obj)
	return 1
}

// filePerOSMethod implements :per_os({...}) for file() objects.
func (vm *LuaVM) filePerOSMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	perOSTbl := L.CheckTable(2)
	obj.RawSetString("per_os", perOSTbl)
	L.Push(obj)
	return 1
}

// fileVariantMethod implements :variant(name) for file/dir/glob objects.
// Assigns an explicit variant name to the file operation.
func (vm *LuaVM) fileVariantMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	name := L.CheckString(2)
	obj.RawSetString("variant_name", lua.LString(name))
	L.Push(obj)
	return 1
}

// dirToMethod implements :to(dest) for dir() objects.
func (vm *LuaVM) dirToMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	dest := L.CheckString(2)
	obj.RawSetString("destination", lua.LString(dest))
	obj.RawSetString("file_op_type", lua.LString("dir_to"))
	L.Push(obj)
	return 1
}

// dirIntoMethod implements :into(dest) for dir() and glob() objects.
// Doesn't override file_op_type if already set (e.g., glob).
func (vm *LuaVM) dirIntoMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	dest := L.CheckString(2)
	obj.RawSetString("destination", lua.LString(dest))
	// Only set to dir_into if it's a dir object (not glob, which already has its own type)
	currentType := lvToString(obj.RawGetString("file_op_type"))
	if currentType == "" || currentType == "dir" {
		obj.RawSetString("file_op_type", lua.LString("dir_into"))
	}
	L.Push(obj)
	return 1
}

// ─── Dep API method handlers ────────────────────────────────────────────────

func (vm *LuaVM) depOnMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	managers := L.CheckTable(2)
	obj.RawSetString("managers", managers)
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depToMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	dest := L.CheckString(2)
	obj.RawSetString("destination", lua.LString(dest))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depAtMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	ref := L.CheckString(2)
	obj.RawSetString("ref", lua.LString(ref))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depExtractMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	extract := L.CheckString(2)
	obj.RawSetString("extract", lua.LString(extract))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depVersionMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	ver := L.CheckString(2)
	obj.RawSetString("version", lua.LString(ver))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depArchMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	archTbl := L.CheckTable(2)
	obj.RawSetString("arch", archTbl)
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depBinMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	bin := L.CheckString(2)
	obj.RawSetString("bin", lua.LString(bin))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depPostMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	cmd := L.CheckString(2)
	obj.RawSetString("post_install", lua.LString(cmd))
	L.Push(obj)
	return 1
}

func (vm *LuaVM) depFallbackMethod(L *lua.LState) int {
	obj := L.CheckTable(1)
	fb := L.CheckTable(2)
	obj.RawSetString("fallback", fb)
	L.Push(obj)
	return 1
}

// ─── Ensure all methods are also accessible directly (api*.go) ──────────────

// detectDepTypeFromTable returns fallback type detection based on table fields.
func detectDepTypeFromTable(tbl *lua.LTable) string {
	url := lvToString(tbl.RawGetString("url"))
	if url != "" {
		if lvToString(tbl.RawGetString("ref")) != "" {
			return "git"
		}
		return "binary"
	}
	return "package"
}

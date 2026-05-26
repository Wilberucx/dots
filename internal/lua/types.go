// Package lua provides a Lua-based configuration system for dots.
// It embeds gopher-lua to load dots.lua module configs and init.lua root configs.
package lua

import "github.com/yuin/gopher-lua"

// FileOpType represents the kind of file operation.
type FileOpType int

const (
	// FileOpFile is a simple file symlink (file()).
	FileOpFile FileOpType = iota
	// FileOpDirTo is a directory symlink (dir():to()).
	FileOpDirTo
	// FileOpDirInto expands directory contents into the destination (dir():into()).
	FileOpDirInto
	// FileOpGlob matches files by glob pattern (glob():into()).
	FileOpGlob
)

// FileOp represents a single file operation declared in dots.lua.
type FileOp struct {
	Type        FileOpType
	Source      string
	Destination string
	Pattern     string
	OSFilter    string
	PerOS       map[string]string
	VariantName string // explicit variant name via :variant("name"); empty = no variant
}

// DepOp represents a dependency declared in dots.lua.
type DepOp struct {
	Name        string
	Type        string // "package", "binary", "git"
	URL         string
	Destination string
	Version     string
	Ref         string
	Extract     string
	Arch        map[string]string
	Managers    map[string]string
	Bin         string
	PostInstall string
	Fallback    *DepOp
}

// ModuleConfig represents the parsed result of a dots.lua module config.
type ModuleConfig struct {
	Type         string
	Files        []FileOp
	Dependencies []DepOp
}

// ModuleType indicates the config format for a module.
type ModuleType int

const (
	// ModuleTypeYAML means the module has path.yaml (legacy).
	// Zero value = YAML for backward compatibility.
	ModuleTypeYAML ModuleType = iota
	// ModuleTypeLua means the module has dots.lua.
	ModuleTypeLua
)

// ModuleDir describes a discovered module directory.
type ModuleDir struct {
	Name string
	Path string
	Type ModuleType // Lua or YAML
}

// RootConfig represents the parsed result of init.lua.
type RootConfig struct {
	Name        string
	ModulePaths []string // empty = default scan; one or more paths = scan only there
	Plugins     []string // built-in plugins to load
}

// LuaValToString recursively extracts a Go string value from a Lua value.
func LuaValToString(val lua.LValue) string {
	switch v := val.(type) {
	case lua.LString:
		return string(v)
	default:
		return val.String()
	}
}

// LuaTableToStringMap converts a Lua table to a Go map[string]string.
func LuaTableToStringMap(tbl *lua.LTable) map[string]string {
	result := make(map[string]string)
	tbl.ForEach(func(key, val lua.LValue) {
		result[LuaValToString(key)] = LuaValToString(val)
	})
	return result
}

// LuaTableToStringSlice converts a Lua table (array-like) to a Go []string.
func LuaTableToStringSlice(tbl *lua.LTable) []string {
	var result []string
	tbl.ForEach(func(_, val lua.LValue) {
		result = append(result, LuaValToString(val))
	})
	return result
}

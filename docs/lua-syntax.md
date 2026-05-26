# `dots` Lua Syntax

> **Version**: 1.0 — June 2026
>
> Complete guide to the Lua syntax for configuring modules in `dots`,
> including the plugin system and how to extend its capabilities.

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [`init.lua` — Root Configuration](#2-initlua--root-configuration)
3. [`dots.lua` — Module Configuration](#3-dotslua--module-configuration)
4. [File Operations API](#4-file-operations-api)
5. [Dependencies API](#5-dependencies-api)
6. [Module Discovery](#6-module-discovery)
7. [Plugin System](#7-plugin-system)
8. [YAML to Lua Migration](#8-yaml-to-lua-migration)
9. [Quick Reference](#9-quick-reference)

---

## 1. Architecture Overview

`dots` uses **two types of Lua files** for configuration:

| File       | Role                          | Location                              |
|------------|-------------------------------|---------------------------------------|
| `init.lua` | Root repository configuration | Repo root (`~/dotfiles/`)             |
| `dots.lua` | Individual module config      | Inside each module (`~/dotfiles/Zsh/dots.lua`) |

Every Lua file **must return a table** (`return { ... }`). The embedded Lua
engine (`gopher-lua`) executes the script and extracts the configuration.

```
dotfiles/                     ← repo root
├── init.lua                  ← marker file + root configuration
├── Zsh/
│   ├── dots.lua              ← Zsh module configuration
│   └── .zshrc                ← source file
├── Nvim/
│   ├── dots.lua
│   └── init.lua
├── scripts/                   ← shared plugins (optional)
│   ├── helpers.lua
│   └── colors.lua
└── .gitignore
```

> **Note**: `init.lua` also serves as a **marker file**: if it exists in a
> directory, `dots` recognizes that directory as a dotfiles repository
> (alongside legacy formats `.dots/config.yaml` and `dots.toml`).

---

## 2. `init.lua` — Root Configuration

### 2.1 Available Fields

The `init.lua` file sits at the root of the repository and defines the
global configuration. All fields are **optional**.

```lua
return {
  -- Repository name (for identification)
  -- Default: "dotfiles"
  name = "cantoarch/dotfiles",

  -- Paths where to look for modules (optional)
  -- Default: scans the repo root
  -- string: scan only that path
  -- table:  scan multiple paths
  module_paths = "modules/",
  -- module_paths = { "packages/", "configs/" },

  -- Plugins to load (optional)
  -- Plugins are loaded via require() and become available as globals
  plugins = { "dots.http", "dots.archive", "dots.git" },
}
```

### 2.2 The `name` Field

Identifies the repository. Used for informational messages and as the
default repo name.

```lua
return { name = "user/dotfiles" }
```

If omitted, the default is `"dotfiles"`.

### 2.3 The `module_paths` Field

Controls **where** `dots` looks for modules. It is a redirection — it does
not add paths, but **replaces** the repo root as the search location.

**Without `module_paths`** (default behavior):

```lua
-- Looks for modules in all directories at the repo root
return { name = "dotfiles" }
-- → Scans: Zsh/, Nvim/, Kitty/, ...
```

**With `module_paths` as a string**:

```lua
-- Looks for modules ONLY inside modules/
return {
  name = "dotfiles",
  module_paths = "modules/",
}
-- → Scans: modules/Zsh/, modules/Nvim/, ...
```

**With `module_paths` as a table**:

```lua
-- Looks for modules ONLY inside packages/ and configs/
return {
  name = "dotfiles",
  module_paths = { "packages/", "configs/" },
}
-- → Scans: packages/Zsh/, configs/Nvim/, ...
```

> **Important**: If a directory is not within the specified paths, it is
> not considered a module. This lets you keep auxiliary directories in the
> repo that are not dotfile modules.

#### Warnings

- If `module_paths` points to a non-existent directory, `dots` shows a
  warning (`[WARN] module_paths "..." does not exist — skipping`) and
  continues with the remaining paths.
- If two modules have the same name across different paths, the first one
  found (by `module_paths` order) wins — the second is skipped.

### 2.4 The `plugins` Field

List of plugins to load into the Lua VM. Plugins can be:

- **Built-in plugins** (embedded in the binary): `dots.http`, `dots.archive`,
  `dots.git`
- **Custom plugins**: placed at `<repo_root>/dots/<name>.lua`

When loaded, each plugin is executed and its return value is assigned as a
global variable with the clean name (without the `dots.` prefix):

```lua
plugins = { "dots.http", "dots.archive", "dots.git" }
-- → http, archive, git become available as Lua globals
```

See [Plugin System](#7-plugin-system) for details.

---

## 3. `dots.lua` — Module Configuration

Each dotfile module has a `dots.lua` file in its root directory.
This file defines the files to symlink and the dependencies to install.

### 3.1 Basic Structure

```lua
return {
  -- Module type (optional)
  -- Common values: "minimal", "full", "editor", "terminal", etc.
  type = "minimal",

  -- List of file operations (optional)
  files = {
    -- ... file(), dir(), glob() operations
  },

  -- List of dependencies (optional)
  dependencies = {
    -- ... pkg(), curl(), git() operations
  },
}
```

### 3.2 The `type` Field

A descriptive label you can use to filter modules via
`dots status --type editor`, `dots link --type terminal`, etc. It has no
effect on behavior.

### 3.3 The `files` Field

Array of operations that produce symlinks. Each operation is created by one
of the `file()`, `dir()`, or `glob()` functions.

```lua
files = {
  file(".zshrc", "~/.zshrc"),
  dir("config"):to("~/.config/alacritty"),
  glob("*.toml"):into("~/.config/"),
}
```

Each item is processed in order and produces one or more `LinkStatus`
entries (symlink states: `linked`, `conflict`, `pending`, `missing`,
`unsafe`).

### 3.4 The `dependencies` Field

Array of dependencies to install. Each dependency is created by one of
the `pkg()`, `curl()`, or `git()` functions.

```lua
dependencies = {
  pkg "ripgrep",
  pkg("starship"):on({ pacman = "starship", brew = "starship" }),
  curl("https://example.com/tool.tar.gz"):extract("tool"):to("~/.local/bin/tool"),
  git("https://github.com/user/repo.git"):to("~/repo"):at("v1.0"),
}
```

Dependencies are deduplicated by name: the first definition wins.

---

## 4. File Operations API

### 4.1 `file(source, destination)`

Creates a symlink from a single file.

```lua
file(".zshrc", "~/.zshrc")
```

**Parameters:**

| Parameter     | Type   | Required | Description                              |
|---------------|--------|----------|------------------------------------------|
| `source`      | string | yes      | Path relative to the module directory    |
| `destination` | string | yes      | Destination path (supports `~` → `$HOME`) |

#### Chainable Methods

##### `:when(os)`

Filters the operation by operating system.

```lua
file(".zshenv", "~/.zshenv"):when("linux")
file(".mac-config", "~/.config/mac"):when("mac")
```

Valid values: `"linux"`, `"mac"`, `"windows"`.

If the current OS does not match the filter, the operation is silently
skipped.

##### `:variant(name)`

Assigns an explicit variant name to a file operation for variant-based
configuration switching.

```lua
file("work/gitconfig", "~/.gitconfig"):variant("work")
file("personal/gitconfig", "~/.gitconfig"):variant("personal")
```

See the [Variants System](variants.md) document for full details on
how variants work.

##### `:per_os({ ... })`

Defines different destinations depending on the operating system.

```lua
file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
  linux = "~/.config/alacritty/linux.toml",
  mac   = "~/Library/Application Support/alacritty/mac.toml",
  windows = "~/AppData/alacritty/win.toml",
})
```

The second argument of `file()` is used as the default destination (fallback)
for OSes not listed in the `per_os` table.

**Combining `:when()` and `:per_os()`:**

```lua
-- :when() filters the entire file; :per_os() selects the destination
file("app.conf", "~/.config/app.conf"):when("linux"):per_os({
  linux = "~/.config/linux.conf",
  mac   = "~/Library/mac.conf",
})
```

---

### 4.2 `dir(source)`

Directory operations. Requires one of `:to()` or `:into()` to complete.

#### `:to(destination)`

Creates a symlink pointing the entire directory to the destination.

```lua
dir("config"):to("~/.config/alacritty")
-- → ~/.config/alacritty → ~/dotfiles/Alacritty/config/ (directory symlink)
```

All contents of the source directory appear at the destination as a single
symlink. Useful for configurations that expect a complete directory.

#### `:into(destination)`

Expands the directory contents: each file inside the source directory is
symlinked individually to the destination.

```lua
dir("scripts"):into("~/.local/bin")
-- If scripts/ contains foo.sh and bar.sh:
-- → ~/.local/bin/foo.sh → ~/dotfiles/Scripts/scripts/foo.sh
-- → ~/.local/bin/bar.sh → ~/dotfiles/Scripts/scripts/bar.sh
```

Unlike `:to()`, which creates a single symlink, `:into()` creates N symlinks
(one per child file).

> **Note**: If the directory is empty, no symlinks are created (silent
> success).

> **⚠️ Common Mistake**: Using `:into()` when you mean `:to()` is the most
> frequent error with `dir()` operations.
>
> ```lua
> -- WRONG: If nvim/ is a directory and you want to symlink it as a whole:
> dir("nvim"):into("~/.config/nvim")  -- ❌ errors: tries to create symlinks
>                                    -- ~/.config/nvim/init.lua instead of:
>
> -- CORRECT: Symlink the entire directory
> dir("nvim"):to("~/.config/nvim")   -- ✅ ~/.config/nvim → dotfiles/Nvim/nvim
> ```
>
> **Rule of thumb:**
>
> | If you want…                               | Use…                     |
> |--------------------------------------------|--------------------------|
> | `~/.config/nvim` → `dotfiles/Nvim/nvim/`   | `dir("nvim"):to("~/.config/nvim")` |
> | `~/.config/nvim/init.lua` → `dotfiles/Nvim/nvim/init.lua` | `dir("nvim"):into("~/.config/nvim")` |
> | `~/.config/nvim/config/foo.lua` → `dotfiles/Nvim/config/foo.lua` | `file("config/foo.lua", "~/.config/nvim/config/foo.lua")` |

#### `:when(os)` and `:per_os({ ... })`

Also available for `dir()` objects:

```lua
dir("scripts"):into("~/.local/bin"):when("linux")
dir("config"):to("~/.config/app"):per_os({
  linux = "~/.config/app",
  mac   = "~/Library/Application Support/app",
})
```

---

### 4.3 `glob(pattern)`

Matches files by glob pattern and symlinks them individually to the
destination.

```lua
glob("*.toml"):into("~/.config/")
-- → ~/.config/alacritty.toml → ~/dotfiles/Configs/alacritty.toml
-- → ~/.config/kitty.toml     → ~/dotfiles/Configs/kitty.toml
-- (readme.txt is NOT linked because it doesn't match *.toml)
```

**Parameters:**

| Parameter | Type   | Description                                   |
|-----------|--------|-----------------------------------------------|
| `pattern` | string | Glob pattern relative to the module directory |

The pattern is resolved with `filepath.Glob()` from the module directory.

> **Note**: If no files match the pattern, no symlinks are created (silent
> success).

#### Methods

- **`:into(destination)`** — Required. Defines the directory where symlinks
  will be placed (each matched file keeps its base name).
- **`:when(os)`** — Optional. Filters by OS.
- **`:per_os({ ... })`** — Optional. Per-OS destinations.

---

### 4.4 Chainable Methods Summary

| Function  | Available Methods                                                     |
|-----------|-----------------------------------------------------------------------|
| `file()`  | `:when(os)`, `:per_os({...})`, `:variant(name)`                      |
| `dir()`   | `:to(dest)`, `:into(dest)`, `:when(os)`, `:per_os({...})`, `:variant(name)` |
| `glob()`  | `:into(dest)`, `:when(os)`, `:per_os({...})`, `:variant(name)`       |

---

## 5. Dependencies API

### 5.1 `pkg(name)`

Declares a system package dependency.

```lua
-- Simple form (name only)
pkg "ripgrep"

-- Explicit form with options
pkg("fd"):on({ pacman = "fd", apt = "fd-find", brew = "fd" })

-- With binary fallback
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/release.tar.gz"):extract("starship"))
```

**Parameters:**

| Parameter | Type   | Description                  |
|-----------|--------|------------------------------|
| `name`    | string | Package name (required)      |

#### Methods

##### `:on({ managers })`

Defines the package name per package manager. Keys are manager names
(`pacman`, `apt`, `brew`, etc.) and values are the corresponding package
names.

```lua
pkg("fd"):on({
  pacman = "fd",
  apt    = "fd-find",
  brew   = "fd",
})
```

##### `:bin(name)`

Specifies the resulting binary name (useful when the binary is named
differently from the package).

```lua
pkg("nodejs"):bin("node")
```

##### `:post(command)`

Command to run after installing the package.

```lua
pkg("neovim"):post("pip install pynvim")
```

##### `:fallback(dep)`

Defines a fallback dependency if the package manager is not available.
The argument must be another dependency object (usually `curl()` to
download a binary).

```lua
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/release.tar.gz"):extract("starship"))
```

---

### 5.2 `curl(url)`

Declares a binary dependency downloaded via HTTP.

```lua
curl("https://github.com/eza-community/eza/releases/latest/eza.tar.gz")
  :extract("eza")
  :to("~/.local/bin/eza")
  :version("v0.10.0")
  :arch({ x86_64 = "amd64", aarch64 = "arm64" })
```

**Parameters:**

| Parameter | Type   | Description                          |
|-----------|--------|--------------------------------------|
| `url`     | string | URL of the file to download (required) |

#### Methods

##### `:extract(member)`

Extracts a specific member from the downloaded archive (tar.gz, zip).

```lua
curl("https://example.com/tool.tar.gz"):extract("tool/bin/tool")
```

##### `:to(destination)`

Defines the final path where the extracted binary will be placed.

```lua
curl("https://example.com/tool.tar.gz"):extract("tool"):to("~/.local/bin/tool")
```

##### `:version(version)`

Specifies the version for URL interpolation (replaces `{{version}}`).

```lua
curl("https://example.com/tool-v{{version}}.tar.gz"):version("v1.2.3")
```

##### `:arch({ arch_map })`

Defines the system architecture to binary name mapping (replaces
`{{arch}}`).

```lua
curl("https://example.com/tool-{{arch}}.tar.gz"):arch({
  x86_64  = "amd64",
  aarch64 = "arm64",
})
```

##### `:bin(name)`

Binary name (for identification).

##### `:post(command)`

Post-installation command.

---

### 5.3 `git(url)`

Declares a git repository dependency.

```lua
git("https://github.com/romkatv/powerlevel10k.git")
  :to("~/.local/share/zsh/plugins/p10k")
  :at("v1.19.0")
  :post("git -C ~/p10k submodule update --init")
```

**Parameters:**

| Parameter | Type   | Description                          |
|-----------|--------|--------------------------------------|
| `url`     | string | Git repository URL (required)        |

#### Methods

##### `:to(destination)`

Path where to clone the repository.

##### `:at(ref)`

Reference to check out (tag, branch, commit hash).

```lua
git("https://github.com/user/repo.git"):to("~/repo"):at("v2.0.0")
git("https://github.com/user/repo.git"):to("~/repo"):at("main")
```

##### `:bin(name)`

Binary name (for identification).

##### `:post(command)`

Post-clone command.

---

### 5.4 Methods Summary Table

| Function | Available Methods                                                       |
|----------|-------------------------------------------------------------------------|
| `pkg()`  | `:on({...})`, `:bin()`, `:post()`, `:fallback()`                       |
| `curl()` | `:extract()`, `:to()`, `:version()`, `:arch({...})`, `:bin()`, `:post()` |
| `git()`  | `:to()`, `:at()`, `:bin()`, `:post()`                                   |

---

## 6. Module Discovery

### 6.1 Discovery Rules

1. `init.lua` is read (if it exists) to obtain `module_paths`
2. If `module_paths` is set: only those paths are scanned (recursively)
3. If `module_paths` is NOT set: the repo root is scanned (recursively)
4. Within the search paths, any directory containing `dots.lua` or
   `path.yaml` is considered a module

### 6.2 Excluded Directories

The following directories are automatically excluded:

- `.git/` — git repository
- `.dots/` — dots legacy directory
- `node_modules/` — Node.js dependencies
- `cli/` — internal CLI (generated)
- Any directory starting with `.` (hidden)

### 6.3 Lua vs YAML Priority

If a directory contains both `dots.lua` and `path.yaml`:

- **`dots.lua` takes priority** with a warning:
  `[WARN] Module 'X' has both dots.lua and path.yaml. Using dots.lua.`
- The module is treated as Lua type (ignoring `path.yaml`)

### 6.4 Directories Without Configuration

Directories without `dots.lua` or `path.yaml` are silently ignored.

### 6.5 Module Order

Modules are returned sorted alphabetically by name for consistency.

---

## 7. Plugin System

### 7.1 Architecture

The plugin system lets you extend `dots` capabilities through Lua scripts
that run in the same VM as module configurations.

```
require("dots.http")
     │
     ├── 1. Search built-in plugins (embedded)
     │   └── internal/lua/plugins/http.lua  ── compiled into the binary
     │
     ├── 2. Search <repo_root>/dots/http.lua
     │
     └── 3. Error: plugin not found
```

The search order is:

1. **Built-in plugins** — embedded in the binary (`internal/lua/plugins/`)
2. **Custom plugins** — at `<repo_root>/dots/<name>.lua`

### 7.2 Built-in Plugins

`dots` includes 3 built-in plugins:

#### `dots.http` → global `http`

```lua
-- Load in init.lua
plugins = { "dots.http" }

-- Use in dots.lua
http.download("https://example.com/file.tar.gz", "/tmp/file.tar.gz")
```

**API:**

- `http.download(url, dest)` — Downloads a URL to a local path. Uses `curl`
  or `wget` (whichever is available).

→ See: `internal/lua/plugins/http.lua`

#### `dots.archive` → global `archive`

```lua
-- Load in init.lua
plugins = { "dots.archive" }

-- Use in dots.lua
archive.extract_tar("/tmp/file.tar.gz", "/tmp/extracted", "bin/tool")
```

**API:**

- `archive.extract_tar(archive, dest, member)` — Extracts a specific member
  from a tar.gz, or everything if `member` is nil/empty.
- `archive.extract_zip(archive, dest)` — Extracts a complete zip.

→ See: `internal/lua/plugins/archive.lua`

#### `dots.git` → global `git`

```lua
-- Load in init.lua
plugins = { "dots.git" }

-- Use in dots.lua
git.clone("https://github.com/user/repo.git", "~/repo")
git.checkout("v1.0", "~/repo")
```

**API:**

- `git.clone(url, dest)` — Clones a git repository.
- `git.checkout(ref, dir)` — Checks out a specific ref.

→ See: `internal/lua/plugins/git.lua`

### 7.3 Custom Plugins

You can create your own plugins by placing `.lua` files in the `dots/`
directory at the root of your dotfiles repository.

#### Creating a Plugin

```lua
-- ~/dotfiles/dots/helpers.lua
local helpers = {}

function helpers.greet(name)
  print("Hello, " .. name .. "!")
end

function helpers.is_installed(bin)
  local handle = io.popen("command -v " .. bin .. " 2>/dev/null")
  local result = handle:read("*a")
  handle:close()
  return result:match("%S") ~= nil
end

return helpers
```

#### Registering in `init.lua`

```lua
-- ~/dotfiles/init.lua
return {
  name = "user/dotfiles",
  plugins = { "dots.http", "helpers" },
}
```

> **Note**: Plugins without the `dots.` prefix are looked up in
> `<repo_root>/dots/<name>.lua`. If they don't exist, an error is raised.

#### Using in `dots.lua`

Plugins are registered as **global variables** in the same Lua VM where
`dots.lua` runs. This means they are accessible from any `dots.lua` without
explicit imports:

```lua
-- ~/dotfiles/Zsh/dots.lua — plugins are available as globals
if helpers.is_installed("starship") then
  print("starship is ready!")
end
```

> The `require()` function is also available as an alternative mechanism.
> The custom searcher registered in `package.loaders` resolves both built-in
> and custom plugins.

### 7.4 Plugin System Limitations

- Plugins are loaded **before** any module's `dots.lua` runs
- Plugins have access to `io`, `os.execute`, and other standard Lua APIs
  (gopher-lua supports a subset of the Lua 5.4 standard library)
- There is no isolation between plugins — they can modify globals
- Plugins **cannot** extend the `dots.lua` syntax (you cannot add new
  functions like `file()` or `pkg()`) because those are registered in Go
  before any Lua script runs
- To add new functions to the `dots.lua` API, you need to modify the Go
  code in `internal/lua/api_files.go` and `internal/lua/api_deps.go`

---

## 8. YAML to Lua Migration

### 8.1 Manual Migration

> **Note**: The `dots migrate` CLI command is under development and not
> available in this version. Migration is done manually using the
> `MigrateModule()` function from Go, or by editing files directly.

The migrator converts existing modules from `path.yaml` to `dots.lua`.
You can migrate manually using the template as a guide.

### 8.2 YAML → Lua Mapping

| YAML (`path.yaml`)                               | Lua (`dots.lua`)                                       |
|---------------------------------------------------|--------------------------------------------------------|
| `source: x` + `destination: ~/x`                 | `file("x", "~/x")`                                     |
| `os: [linux]`                                     | `:when("linux")`                                        |
| `per-os: { linux: ~/x, mac: ~/y }`               | `:per_os({ linux = "~/x", mac = "~/y" })`              |
| `destination: ~/dir/*` (expansion)                | `dir("src"):into("~/dir")`                             |
| Extensionless source (directory)                  | `dir("src"):to("~/dest")`                              |
| `dependencies: ["pkg"]`                           | `pkg "pkg"`                                             |
| `type: binary` + `url:`                           | `curl(url):extract():to()`                             |
| `type: git` + `url:`                              | `git(url):to():at()`                                   |
| `managers: { pacman: x }`                         | `:on({ pacman = "x" })`                                |
| `fallback:`                                       | `:fallback(curl(...))`                                  |

### 8.3 Migration Example

**Before** (`path.yaml`):

```yaml
type: full
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty.yml
      mac: ~/Library/alacritty.yml
  - source: scripts
    destination: ~/.local/bin/*
dependencies:
  - neovim
  - name: fd
    type: binary
    url: https://example.com/fd.tar.gz
    dest: ~/.local/bin/fd
    extract: fd/fd
```

**After** (`dots.lua`):

```lua
-- dots.lua — generated from path.yaml
return {
  type = "full",

  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("alacritty.yml", "~/.config/alacritty.yml"):per_os({
      linux = "~/.config/alacritty.yml",
      mac = "~/Library/alacritty.yml",
    }),
    dir("scripts"):into("~/.local/bin"),
  },

  dependencies = {
    pkg "neovim",
    curl("https://example.com/fd.tar.gz"):bin("fd"):extract("fd/fd"):to("~/.local/bin/fd"),
  },
}
```

### 8.4 Coexistence

During the transition, both formats coexist:

- Modules with `dots.lua` use the new system
- Modules with only `path.yaml` use the legacy system
- If both exist, `dots.lua` wins (with a warning)

---

## 9. Quick Reference

### 9.1 `init.lua` — Fields

```lua
return {
  name         = "string",        -- Repo name (default: "dotfiles")
  module_paths = "path/" | {...}, -- Search paths (default: root)
  plugins      = { "dots.http" }, -- Plugins to load
}
```

### 9.2 `dots.lua` — Fields

```lua
return {
  type         = "string",        -- Module label (optional)
  files        = { ... },         -- File operations (optional)
  dependencies = { ... },         -- Dependencies (optional)
}
```

### 9.3 File Operations

```lua
-- Single file
file("source", "~/.dest")

-- With OS filter
file("source", "~/.dest"):when("linux")

-- With per-OS destination
file("source", "~/.dest"):per_os({ linux = "~/.a", mac = "~/.b" })

-- With variant declaration
file("work/config", "~/.config/app"):variant("work")
file("personal/config", "~/.config/app"):variant("personal")

-- Full directory (symlink)
dir("folder"):to("~/.dest")

-- Expand directory contents
dir("folder"):into("~/.dest")

-- Glob pattern
glob("*.toml"):into("~/.config/")
```

### 9.4 Dependencies

```lua
-- System package (simple)
pkg "ripgrep"

-- Package with managers
pkg("fd"):on({ pacman = "fd", apt = "fd-find" })

-- Package with binary fallback
pkg("starship"):on({ pacman = "starship" })
  :fallback(curl("https://..."):extract("starship"))

-- Downloaded binary
curl("https://..."):extract("tool"):to("~/.local/bin/tool"):version("v1.0"):arch({ x86_64 = "amd64" })

-- Git repository
git("https://github.com/user/repo.git"):to("~/repo"):at("v1.0"):post("make install")
```

### 9.5 Plugins

```lua
-- init.lua: load built-in and custom plugins
return {
  plugins = { "dots.http", "dots.archive", "dots.git", "helpers" },
}

-- dots.lua: use plugins
http.download("https://example.com/file.tar.gz", "/tmp/file.tar.gz")
archive.extract_tar("/tmp/file.tar.gz", "/tmp/out", "bin/tool")
git.clone("https://github.com/user/repo.git", "~/repo")
```

---

## 10. Compatibility Notes

### 10.1 `config.lua` as Alternative Marker

In addition to `init.lua`, `dots` also recognizes `config.lua` as a marker
file for Lua repositories. If `config.lua` exists but `init.lua` does not,
the repo is still detected as a Lua repository.

> `init.lua` takes priority over `config.lua` if both exist.

### 10.2 Coexistence with Legacy Formats

`dots` supports three repository formats simultaneously:

| Marker             | Format   | Status        |
|--------------------|----------|---------------|
| `init.lua`         | Lua      | **Recommended** |
| `.dots/config.yaml` | YAML (v3) | Supported     |
| `dots.toml`        | TOML     | Legacy        |

Individual modules can also coexist: modules with `dots.lua` (Lua) and
modules with `path.yaml` (YAML) in the same repo.

---

## Appendix: Complete Examples

### Example 1: Minimal Editor

```lua
-- ~/dotfiles/Nvim/dots.lua
return {
  type = "editor",
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("lazy-lock.json", "~/.config/nvim/lazy-lock.json"):when("linux"),
    file("neovide.toml", "~/.config/neovide/config.toml"):per_os({
      linux = "~/.config/neovide/config.toml",
      mac   = "~/Library/Application Support/neovide/config.toml",
    }),
  },
  dependencies = {
    pkg "neovim",
    pkg("lazygit"):on({ pacman = "lazygit", brew = "lazygit" }),
  },
}
```

### Example 2: Terminal with Themes

```lua
-- ~/dotfiles/Kitty/dots.lua
return {
  type = "terminal",
  files = {
    file("kitty.conf", "~/.config/kitty/kitty.conf"),
    dir("themes"):into("~/.config/kitty/themes"),
  },
  dependencies = {
    pkg "kitty",
    curl("https://github.com/dexpota/kitty-themes/archive/master.tar.gz")
      :extract("kitty-themes-master/themes")
      :to("~/.config/kitty/themes")
      :version("latest"),
  },
}
```

### Example 3: Multi-OS Configuration

```lua
-- ~/dotfiles/Alacritty/dots.lua
return {
  type = "terminal",
  files = {
    -- File with different destinations per OS
    file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
      linux   = "~/.config/alacritty/linux.toml",
      mac     = "~/Library/Application Support/alacritty/mac.toml",
      windows = "~/AppData/Roaming/alacritty/win.toml",
    }),
    -- Linux only
    dir("linux-scripts"):into("~/.local/bin"):when("linux"),
    -- Mac only
    file("mac-fonts.xml", "~/Library/Fonts/jetbrains.xml"):when("mac"),
  },
  dependencies = {
    pkg("alacritty"):on({
      pacman = "alacritty",
      brew   = "alacritty",
    }),
  },
}
```

---

## See Also

- [Spanish version](docs/sintaxis-lua.md) — Versión en español
- [path.yaml reference](docs/path-yaml-reference.md) — Legacy YAML format
- [Schema v3](docs/schema-v3.md) — YAML schema specification

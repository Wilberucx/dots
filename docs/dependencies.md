# Dependencies System

This document explains how the dependency system works in `dots` and how the `install` command processes them.

For the legacy YAML schema reference, see [path-yaml-reference.md](./path-yaml-reference.md).

---

## Dependency Types Overview

There are three dependency types, each with a Lua constructor function:

| Function | Type | Use Case |
|----------|------|----------|
| `pkg()` | System package | Git, curl, fzf — installed via pacman/apt/brew |
| `git()` | Git repository | Plugins, themes — cloned from a git URL |
| `curl()` | Binary download | Precompiled releases (starship, eza) — downloaded + extracted |

---

## 1. `pkg(name)` — System Packages

For packages available in system package managers.

### Simple Declaration

When a package has the same name across all package managers:

```lua
dependencies = {
  pkg "git",
  pkg "curl",
  pkg "zsh",
}
```

This installs `git`, `curl`, and `zsh` via whatever package manager is detected (pacman → apt → brew).

### Per-Manager Names (`:on({...})`)

Some packages have different names per PM. **ripgrep** is the canonical example: same binary name everywhere, but the package name `fd` is `fd-find` on Debian/Ubuntu.

```lua
pkg("fd"):on({
  pacman = "fd",
  apt    = "fd-find",
  brew   = "fd",
})
```

The argument to `pkg()` is the **deduplication key** — it's used to check if already installed via `shutil.which()` and to avoid installing the same dependency twice. The package manager receives the mapped name.

### Binary Name (`:bin()`)

When the binary is named differently from the package:

```lua
pkg("nodejs"):bin("node")
```

### Post-Install (`:post()`)

Command to run after installing the package:

```lua
pkg("neovim"):post("pip install pynvim")
```

### Fallback (`:fallback()`)

If the package manager is not available or the package isn't found, provide a fallback. The argument must be another dependency object (usually `curl()` to download a binary):

```lua
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/starship/releases/download/v{{version}}/starship-{{arch}}-unknown-linux-musl.tar.gz")
    :extract("starship")
    :to("~/.local/bin/starship")
    :version("1.19.0")),
```

### Behavior Flow

```
1. Is :on({...}) defined?
   ├─ No: use pkg name directly
   └─ Yes: look up current PM
       ├─ Found: use mapped name
       └─ Not found: check fallback
           ├─ Has fallback: use fallback dep
           └─ No fallback: skip with warning
2. Is binary already in PATH?
   ├─ Yes: skip silently
   └─ No: run PM install command
```

### Why This Design?

`pkg():on()` exists because distros rename upstream software. `ripgrep` → `rg` on Debian, `fd` → `fd-find` on Ubuntu. The deduplication key (the `pkg()` argument) is what matters for tracking — the mapped name is only used at install time.

---

## 2. `git(url)` — Git Repositories

Clones a Git repository to a local path. Use for plugins, themes, and tools that need the full repo history or require specific git refs.

### Example: Zsh Plugin

```lua
git("https://github.com/romkatv/powerlevel10k.git")
  :to("~/.local/share/zsh/plugins/powerlevel10k")
  :at("v1.19.0")
  :post("~/.fzf/install --all"),
```

### Chainable Methods

| Method | Required | Purpose |
|--------|----------|---------|
| `:to(dest)` | Yes | Destination path (`~` expanded) |
| `:at(ref)` | No | Tag, branch, or commit hash to check out |
| `:post(cmd)` | No | Shell command to run after clone |
| `:bin(name)` | No | Binary name (for identification) |

**Why `:at()` matters:** Without it, you get whatever the default branch is at clone time — a moving target. Pinning `:at("v1.19.0")` ensures reproducible installs.

**`:post()` runs after clone + checkout.** The `~/.fzf/install` example sets up fzf's shell integrations.

### Behavior

```
target exists? → skip silently
clone source to target
  └─ :at() provided? → git checkout <ref>
```

---

## 3. `curl(url)` — Binary Downloads

Downloads a precompiled binary release (tarball or raw binary). Use when upstream doesn't provide a package or you want a specific version.

### Example: eza

```lua
curl("https://github.com/eza-community/eza/releases/download/v{{version}}/eza_{{arch}}-unknown-linux-musl.tar.gz")
  :extract("eza")
  :to("~/.local/bin/eza")
  :version("0.18.21")
  :arch({ x86_64 = "x86_64", aarch64 = "aarch64" }),
```

### Chainable Methods

| Method | Required | Purpose |
|--------|----------|---------|
| `:to(dest)` | Yes | Final path for the extracted binary |
| `:extract(member)` | Depends | Archive member to extract (tar.gz/zip) |
| `:version(v)` | No | Template variable `{{version}}` in URL |
| `:arch({...})` | No | Architecture mapping for `{{arch}}` in URL |
| `:bin(name)` | No | Binary name (for identification) |
| `:post(cmd)` | No | Post-installation command |

### Template Variables

The URL supports two template variables:

| Variable | Replaced With |
|----------|---------------|
| `{{version}}` | The `:version()` value |
| `{{arch}}` | Mapped via `:arch()`; detected from `uname -m` |

Architecture detection (`uname -m` → `x86_64`, `aarch64`, `armv7l`, etc.) uses the detected value as the key in the `:arch()` map to produce the URL segment.

### The `:arch()` Map

Some projects use different architecture strings in their URLs. eza uses `x86_64` and `aarch64` (matching our detection). But some projects use `amd64` instead of `x86_64`:

```lua
curl("https://example.com/tool-{{arch}}.tar.gz")
  :arch({ x86_64 = "amd64", aarch64 = "arm64" }),
  -- detects x86_64 → URL gets "amd64"
```

### The `:extract()` Field

Tarballs from GitHub releases often contain multiple files:

```
starship-{{arch}}-unknown-linux-musl.tar.gz
├── starship          ← binary we want
├── LICENSE
└── README.md
```

Without `:extract()`, tarfile extracts **all members** to the target's parent directory — polluting `$HOME/.local/bin/` with LICENSE files.

With `:extract("starship")`, only the `starship` member is extracted to the path specified by `:to()`.

### Example: starship (with fallback)

```lua
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/starship/releases/download/v{{version}}/starship-{{arch}}-unknown-linux-musl.tar.gz")
    :to("~/.local/bin/starship")
    :version("1.19.0")
    :extract("starship")),
```

---

## 4. The `:fallback()` System

### When It Activates

`:fallback()` triggers when:

1. A `pkg()` dependency uses `:on({...})` for per-manager names
2. The current PM is **not** in that dict
3. A `:fallback()` is defined

### Why It Exists

Consider Ubuntu/Debian users who want `starship`. The official `starship` package might be outdated or unavailable. Rather than forcing all users to use a binary, you can:

- Define `:on({pacman = "starship", brew = "starship"})` for distros with up-to-date packages
- Provide a `:fallback(curl(...))` binary for apt users

This gives per-PM installation strategies without duplicating dependency definitions.

### Supported Fallback Types

| Fallback Function | Handler |
|-------------------|---------|
| `curl()` | Download + extract binary |
| `git()` | Git clone + checkout |

### Binary vs Git Fallback

**Binary fallback** is most common — it's how you handle "PM doesn't have it, download instead."

**Git fallback** is rare but useful for:
- A plugin that has a special install method (e.g., vim plugin that needs `git clone` into `~/.vim/pack/`)
- A tool with a git-based installation script

---

## 5. `install` Command

### Command Signature

```bash
dots install [OPTIONS]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run`, `-n` | Boolean | Show what would be installed without executing |
| `--module`, `-m` | List[str] | Install only deps from specific modules (repeatable) |
| `--type`, `-t` | List[str] | Install only deps from modules with matching `type` field (repeatable) |

### Processing Order

```
1. Detect package manager (pacman → apt → brew)
2. Load modules filtered by --module/--type flags
3. Parse dependencies from each module's config (dots.lua or path.yaml)
4. Deduplicate by name (first occurrence wins)
5. For each dependency:
   a. Dispatch by type to appropriate handler
   b. Run post_install if defined
6. Print summary
```

### Deduplication Logic

Dependencies are deduplicated by `name` (the first argument to `pkg()`, `curl()`, or `git()`). **First definition wins:**

```lua
-- Module A
dependencies = { pkg "starship" }  -- added here

-- Module B
dependencies = { pkg "starship" }  -- SKIPPED — already seen
```

This prevents the same dependency from being installed twice when multiple modules declare it.

### Common Errors

**"No supported package manager found"**

```
Error: No supported package manager found (pacman, apt, brew).
```

`dots` only supports pacman (Arch), apt (Debian/Ubuntu), and brew (macOS/Linux). If you see this, either:
- You're on an unsupported distro (contribute a plugin!)
- The PM detection failed

**`post_install` failures don't block installation**

If `:post()` fails (returns non-zero), the dependency is still marked as installed. Errors are silently ignored unless you use `--dry-run`.

### Dry Run Output

```bash
$ dots install --dry-run
[DRY] Would run: sudo pacman -S --noconfirm ripgrep
[DRY] Would run: sudo apt-get install -y curl
```

---

## 6. Methods Summary

| Function | Available Methods |
|----------|-------------------|
| `pkg()` | `:on({...})`, `:bin()`, `:post()`, `:fallback()` |
| `curl()` | `:extract()`, `:to()`, `:version()`, `:arch({...})`, `:bin()`, `:post()` |
| `git()` | `:to()`, `:at()`, `:bin()`, `:post()` |

---

## 7. YAML Legacy Reference

> **Note**: This section documents the legacy YAML format for backward compatibility.
> New configurations should use the Lua syntax from sections 1-3.

### YAML to Lua Quick Map

```lua
-- YAML: dependencies: ["ripgrep"]
-- Lua:
pkg "ripgrep"

-- YAML: type: package with managers
--   name: fd
--   type: package
--   managers: { pacman: fd, apt: fd-find }
--   bin: fd
-- Lua:
pkg("fd"):on({ pacman = "fd", apt = "fd-find" }):bin("fd")

-- YAML: type: binary
--   name: eza
--   type: binary
--   url: https://.../eza_{{arch}}-linux.tar.gz
--   dest: ~/.local/bin/eza
--   version: "0.18.21"
--   extract: eza
-- Lua:
curl("https://.../eza_{{arch}}-linux.tar.gz"):extract("eza"):to("~/.local/bin/eza"):version("0.18.21")

-- YAML: type: git
--   name: p10k
--   type: git
--   url: https://github.com/romkatv/powerlevel10k.git
--   dest: ~/p10k
--   ref: v1.19.0
-- Lua:
git("https://github.com/romkatv/powerlevel10k.git"):to("~/p10k"):at("v1.19.0")
```

### Legacy Field Reference

| Field | YAML Types | Lua Equivalent |
|-------|------------|----------------|
| `name` | all | First arg to `pkg()`/`curl()`/`git()` |
| `type` | all | Constructor: `pkg`/`curl`/`git` |
| `url` | git, binary | First arg to `curl()`/`git()` |
| `dest` | git, binary | `:to(dest)` |
| `version` | binary | `:version(v)` |
| `ref` | git | `:at(ref)` |
| `arch` | binary | `:arch({...})` |
| `managers` | package | `:on({...})` |
| `extract` | binary | `:extract(member)` |
| `fallback` | package | `:fallback(...)` |
| `post-install` | all | `:post(cmd)` |
| `bin` | package | `:bin(name)` |

### Deprecated: `type: system`

`type: system` is an alias for `type: package` (legacy, kept for backwards compatibility). In Lua, always use `pkg()`.

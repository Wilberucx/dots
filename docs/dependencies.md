# Dependencies System

This document explains how the dependency system works in `dots` and how the `install` command processes them. For the schema reference, see [path-yaml-reference.md](./path-yaml-reference.md).

---

## Dependency Types Overview

There are three dependency types:

| Type | Use Case | Installs Via |
|------|----------|--------------|
| `package` | System packages (git, curl, fzf) | pacman, apt, brew |
| `git` | Plugins/themes from repos | git clone |
| `binary` | Precompiled releases (starship, eza) | Download + extract |

The type field defaults to `package` if omitted.

---

## `type: package`

For packages available in system package managers.

### String Shorthand

When a package has the same name across all package managers, use the string shorthand:

```yaml
dependencies:
  - git
  - curl
  - zsh
```

This installs `git`, `curl`, and `zsh` via whatever package manager is detected.

### `package-managers` Field

Some packages have different names per PM. **ripgrep** is the canonical example:

| Manager | Package Name |
|---------|--------------|
| pacman | `ripgrep` |
| apt | `ripgrep` |
| brew | `ripgrep` |

But `rg` (the binary) is the same everywhere. Use `package-managers` to map:

```yaml
- name: rg
  type: package
  package-managers:
    pacman: ripgrep
    apt: ripgrep
    brew: ripgrep
```

The `name` field (`rg`) is the deduplication key — used to check if already installed via `shutil.which()`. The package manager receives the mapped name (`ripgrep`).

### Behavior Flow

```
1. Is package-managers defined?
   ├─ No: use name directly
   └─ Yes: look up current PM
       ├─ Found: use mapped name
       └─ Not found: check fallback → skip with warning
2. Is binary already in PATH (shutil.which)?
   ├─ Yes: skip silently
   └─ No: run PM install command
```

### Why This Design?

`package-managers` exists because distros rename upstream software. `ripgrep` → `rg` on Debian, `fd` → `fd-find` on Ubuntu. The deduplication key (`name`) is what matters for tracking — the mapped name is only used at install time.

---

## `type: git`

Clones a Git repository to a local path. Use for plugins, themes, and tools that need the full repo history or require specific git refs.

### Fields

| Field | Required | Purpose |
|-------|----------|---------|
| `source` | Yes | Git URL (https or ssh) |
| `target` | Yes | Destination path (`~` expanded) |
| `ref` | No | Tag, branch, or commit hash |
| `post_install` | No | Shell command after clone |

### Example: Zsh Plugin

```yaml
- name: powerlevel10k
  type: git
  source: https://github.com/romkatv/powerlevel10k.git
  target: ~/.local/share/zsh/plugins/powerlevel10k
  ref: v1.19.0
  post_install: ~/.fzf/install --all
```

**Why `ref` matters:** Without it, you get whatever the default branch is at clone time — a moving target. Pinning `ref` ensures reproducible installs.

**`post_install` runs after clone + checkout.** The `~/.fzf/install` example sets up fzf's shell integrations.

### Behavior

```
target exists? → skip silently
clone source to target
  └─ ref provided? → git checkout <ref>
```

---

## `type: binary`

Downloads a precompiled binary release (tarball or raw binary). Use when upstream doesn't provide a package or you want a specific version.

### Template Variables

The `source` URL supports two template variables:

| Variable | Replaced With |
|----------|---------------|
| `{{version}}` | The `version` field value |
| `{{arch}}` | Detected system architecture |

Architecture detection:

```python
platform.machine() → x86_64, aarch64, armv7l, etc.
```

### `arch_map` Field

Some projects use different arch strings in their URLs. eza uses `x86_64` and `aarch64` (matching our detection). But starship uses `x86_64` and `aarch64` too. 

A project that differs: some use `amd64` instead of `x86_64`. Use `arch_map` to translate:

```yaml
arch_map:
  x86_64: amd64      # detection → URL segment
  aarch64: arm64
```

### `extract_path` Field

Tarballs from GitHub releases often contain multiple files:

```
starship-{{arch}}-unknown-linux-musl.tar.gz
├── starship          ← binary we want
├── LICENSE
└── README.md
```

Without `extract_path`, tarfile extracts **all members** to the target's parent directory — polluting `$HOME/.local/bin/` with LICENSE files.

With `extract_path: starship`, only the `starship` member is extracted to `target`.

### Example: eza

```yaml
- name: eza
  type: binary
  source: https://github.com/eza-community/eza/releases/download/v{{version}}/eza_{{arch}}-unknown-linux-musl.tar.gz
  target: ~/.local/bin/eza
  version: "0.18.21"
  extract-path: eza
  arch_map:
    x86_64: x86_64
    aarch64: aarch64
```

**Why a static arch_map?** Consistency. If eza ever changed their URL format to `eza_x86_64` instead of `eza_x86_64`, you'd only change the map, not the URL template.

### Example: starship (with fallback)

```yaml
- name: starship
  type: package
  package-managers:
    pacman: starship
    brew: starship
  fallback:
    type: binary
    source: https://github.com/starship/starship/releases/download/v{{version}}/starship-{{arch}}-unknown-linux-musl.tar.gz
    target: ~/.local/bin/starship
    version: "1.19.0"
    extract-path: starship
```

---

## The `fallback` Field

### When It Activates

`fallback` triggers when:

1. A `package` dependency uses `package-managers`
2. The current PM is **not** in that dict
3. A `fallback` is defined

### Why It Exists

Consider Ubuntu/Debian users who want `starship`. The official `starship` package might be outdated or unavailable. Rather than forcing all users to use a binary, you can:

- Define `package-managers` for pacman and brew (who have up-to-date packages)
- Provide a `fallback` binary for apt users

This gives per-PM installation strategies without duplicating dependency definitions.

### Supported Fallback Types

| Fallback Type | Handler |
|---------------|---------|
| `binary` | `install_binary_dep()` |
| `git` | `install_git_dep()` |
| Other | **skipped with warning** |

### Binary vs Git Fallback

**Binary fallback** is most common — it's how you handle "PM doesn't have it, download instead."

**Git fallback** is rare but useful for:

- A plugin that has a special install method (e.g., vim plugin that needs `git clone` into `~/.vim/pack/`)
- A tool with a git-based installation script

---

## `install` Command

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
3. Parse dependencies from each path.yaml
4. Deduplicate by name (first occurrence wins)
5. For each dependency:
   a. Dispatch by type to appropriate handler
   b. Run post_install if defined
6. Print summary
```

### Deduplication Logic

Dependencies are deduplicated by `name`. **First definition wins:**

```yaml
# Module A
dependencies:
  - starship  # added here

# Module B  
dependencies:
  - starship  # SKIPPED — already seen
```

This prevents the same dependency from being installed twice when multiple modules declare it.

### Common Errors

**"No supported package manager found"**

```bash
Error: No supported package manager found (pacman, apt, brew).
```

`dots` only supports pacman (Arch), apt (Debian/Ubuntu), and brew (macOS/Linux). If you see this, either:

- You're on an unsupported distro (contribute a plugin!)
- The PM detection failed

**`post_install` failures don't block installation**

If `post_install` fails (returns non-zero), the dependency is still marked as installed. `post_install` runs via `shell=True` — errors are silently ignored unless you use `--dry-run`.

### Dry Run Output

```bash
$ dots install --dry-run
[DRY] Would run: sudo pacman -S --noconfirm ripgrep
[DRY] Would run: sudo apt-get install -y curl
```

---

## Conflict Resolution

Conflict resolution applies to **`dots link`**, not `dots install`. The install command only fetches and installs dependencies.

### Link States

When `dots link` encounters a destination path:

| State | Meaning |
|-------|---------|
| `linked` | Symlink points to correct source — skip |
| `conflict` | Symlink points elsewhere — replace |
| `pending` | Regular file exists — backup + link |
| `missing` | Nothing exists — create link |
| `unsafe` | Target outside `$HOME` — blocked |

### Conflict Resolution Table

| Situation | Behavior |
|-----------|----------|
| File exists, not a symlink | Creates `.bak`, then creates symlink |
| Symlink exists, points elsewhere | **Replaces** with correct symlink |
| Symlink already correct | **Skips** — already linked |
| `.bak` already exists | **Blocks** — manual intervention required |

### The `.bak` Strategy

**One `.bak` per file. No timestamps.**

Why no timestamps? Imagine `zshrc.bak.2024-01-15_14-30-22` — now you have N backups from N runs. The intention is **friction**: if `.bak` already exists, you have to consciously decide what to do with it.

If you really want to proceed:

```bash
rm ~/.zshrc.bak
dots link
```

### Example Flow

```bash
$ dots link
⚠️  File exists at ~/.zshrc — creating .bak
✅ Linked: module/zsh/.zshrc → ~/.zshrc
```

Next run, if you haven't resolved the `.bak`:

```bash
⚠️  Backup already exists at ~/.zshrc.bak — remove manually to proceed
```

---

## Field Reference

| Field | Types | Description |
|-------|-------|-------------|
| `name` | all | Unique identifier; used for deduplication |
| `type` | all | `package` (default), `git`, `binary` |
| `source` | git, binary | URL or repo |
| `target` | git, binary | Destination path |
| `version` | binary | Template variable for URL |
| `ref` | git | tag/branch/commit |
| `arch_map` | binary | Arch string translation |
| `package-managers` | package | PM → package name mapping |
| `extract-path` | binary | Archive member to extract |
| `fallback` | package | Inline dep when PM unavailable |
| `post_install` | all | Shell command after install |

### Deprecated: `type: system`

`type: system` is an alias for `type: package` (legacy, kept for backwards compatibility). New configs should use `type: package`.

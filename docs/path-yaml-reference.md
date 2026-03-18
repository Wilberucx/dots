# path.yaml Reference

Each module in your dotfiles repo is a directory containing:
- A `path.yaml` file — declares files to link and dependencies to install
- The actual config files or directories to be symlinked

## Module structure

```
Zsh/
├── path.yaml
├── .zshrc
├── .zshenv
└── .zprofile
```

---

## Top-level fields

### `type` (optional)
Groups the module for filtered operations with `--type`.
Value is free-form — define your own groups.

```yaml
type: minimal   # or: work, gaming, server — anything you want
```

Usage:
```bash
dots link --type minimal     # link only minimal modules
dots install --type minimal  # install deps only for minimal modules
```

Modules without `type` are excluded when `--type` is used.

---

## `files` section

Declares which files to symlink and where.

```yaml
files:
  - source: .zshrc          # path relative to the module directory
    os: [linux, mac]        # OS filter — omit to apply on all OS
    destination: ~/.zshrc   # symlink target, ~ is expanded
```

### OS filtering

`os` accepts: `linux`, `mac`, `windows`

```yaml
- source: .zshrc
  os: [linux, mac]     # only linked on linux and mac
  destination: ~/.zshrc
```

### OS-specific destination

Use `destination` as the default and `destination-override` for exceptions:

```yaml
- source: config.toml
  os: [linux, mac]
  destination: ~/.config/tool/config.toml        # default
  destination-override:
    mac: ~/Library/Preferences/tool/config.toml  # mac override
```

---

## `dependencies` section

Declares what to install when `dots install` is run.

### String shorthand
For packages with the same name across all package managers:

```yaml
dependencies:
  - git
  - curl
  - zsh
```

### `type: package` — system package manager
For packages that may have different names per package manager.
Without `package-managers`, uses the name directly:

```yaml
- name: fzf
  type: package   # optional, this is the default
```

With per-manager name mapping:

```yaml
- name: rg
  type: package
  package-managers:
    pacman: ripgrep
    apt: ripgrep
    brew: ripgrep
```

If the current package manager is not listed, the package is skipped
with a warning — unless a `fallback` is declared.

### `fallback` — automatic fallback for unavailable packages
When a package is not available in the current package manager,
`fallback` provides an alternative installation method:

```yaml
- name: starship
  type: package
  package-managers:
    pacman: starship
    brew: starship
    # apt not listed — will use fallback
  fallback:
    type: binary
    source: https://github.com/starship/starship/releases/download/v{{version}}/starship-{{arch}}-unknown-linux-musl.tar.gz
    target: ~/.local/bin/starship
    version: "1.19.0"
    extract-path: starship   # path of the binary inside the archive
    arch_map:
      x86_64: x86_64
      aarch64: aarch64
```

Fallback supports `type: binary` and `type: git`.

### `type: git` — clone a repository
```yaml
- name: powerlevel10k
  type: git
  source: https://github.com/romkatv/powerlevel10k.git
  target: ~/.local/share/zsh/plugins/powerlevel10k
  ref: v1.19.0   # optional: pin to tag, branch, or commit hash
```

If `ref` is omitted, clones the default branch HEAD.
If the target already exists, skips silently.

### `type: binary` — download a precompiled binary
```yaml
- name: eza
  type: binary
  source: https://github.com/eza-community/eza/releases/download/v{{version}}/eza_{{arch}}-unknown-linux-musl.tar.gz
  target: ~/.local/bin/eza
  version: "0.18.21"
  extract-path: eza               # member path inside the archive
  arch_map:
    x86_64: x86_64
    aarch64: aarch64
```

**Template variables:**
- `{{version}}` — replaced with the `version` field value
- `{{arch}}` — replaced with detected system arch (`x86_64`, `aarch64`)
  `arch_map` maps the detected arch to a custom string if needed

**`extract-path`:** relative path of the binary inside the archive.
If omitted, extracts the entire archive to the target's parent directory.

### `post_install` — run a command after installation
Available on any dependency type:

```yaml
- name: fzf
  type: git
  source: https://github.com/junegunn/fzf.git
  target: ~/.fzf
  post_install: ~/.fzf/install --all
```

---

## Complete example

```yaml
# Module type for filtered operations
type: minimal

files:
  - source: .zshrc
    os: [linux, mac]
    destination: ~/.zshrc
  - source: config.toml
    os: [linux, mac]
    destination: ~/.config/tool/config.toml
    destination-override:
      mac: ~/Library/Preferences/tool/config.toml

dependencies:
  # Simple packages — same name everywhere
  - git
  - curl

  # Package with per-manager mapping
  - name: rg
    type: package
    package-managers:
      pacman: ripgrep
      apt: ripgrep
      brew: ripgrep

  # Package with fallback for apt
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
      arch_map:
        x86_64: x86_64
        aarch64: aarch64

  # Git repository
  - name: powerlevel10k
    type: git
    source: https://github.com/romkatv/powerlevel10k.git
    target: ~/.local/share/zsh/plugins/powerlevel10k
    ref: v1.19.0

  # Binary download
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

---

## Conflict resolution behavior

When `dots link` encounters an existing file at the destination:

| Situation | Behavior |
|---|---|
| File exists, not a symlink | Creates `.bak`, then creates symlink |
| Symlink exists, points elsewhere | Replaces with correct symlink |
| Symlink already correct | Skip — already linked |
| `.bak` already exists | **Blocks** — review and remove manually |

No timestamp suffixes. One `.bak` per file — intentional friction
to keep your dotfiles intentional and clean.

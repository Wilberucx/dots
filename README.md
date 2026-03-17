# dots

dotfile manager — declarative, symlink-based, yours.

## Install
```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Requires Python 3.10+ and git. `pipx` is installed automatically.

---

## path.yaml reference

Each module is a directory with a `path.yaml` and its config files.
```yaml
# Optional: group modules for filtered operations
type: minimal

files:
  # Same destination on all OS
  - source: .zshrc
    os: [linux, mac]
    destination: ~/.zshrc

  # OS-specific override
  - source: config.toml
    os: [linux, mac]
    destination: ~/.config/tool/config.toml
    destination-override:
      mac: ~/Library/Preferences/tool/config.toml

dependencies:
  # String shorthand — installs via system package manager
  - git
  - curl

  # Package with per-manager name mapping
  - name: rg
    type: package
    package-managers:
      pacman: ripgrep
      apt: ripgrep
      brew: ripgrep

  # Git repository — optional ref to pin version
  - name: powerlevel10k
    type: git
    source: https://github.com/romkatv/powerlevel10k.git
    target: ~/.local/share/zsh/plugins/powerlevel10k
    ref: v1.19.0

  # Binary download with arch and version templating
  - name: eza
    type: binary
    source: https://github.com/eza-community/eza/releases/download/v{{version}}/eza_{{arch}}.tar.gz
    target: ~/.local/bin/eza
    version: "0.18.0"
    extract-path: eza
    arch_map:
      x86_64: x86_64-unknown-linux-gnu
      aarch64: aarch64-unknown-linux-gnu
```

---

## Commands

| Command | Description |
|---|---|
| `dots init` | Initialize repo — creates `dots.toml` marker |
| `dots link` | Create symlinks for all modules |
| `dots unlink` | Remove symlinks |
| `dots status` | Show link state grouped by status |
| `dots adopt <path>` | Import an existing config into the repo |
| `dots install` | Install dependencies from all `path.yaml` files |
| `dots backup` | Git commit and optional push |

## Flags

Available on `link`, `unlink`, `status`, and `install`:

| Flag | Description |
|---|---|
| `-m / --module` | Filter by module name (repeatable) |
| `-t / --type` | Filter by module type (repeatable) |
| `-s / --state` | Filter by state: `linked` `unlinked` `broken` (status only) |
| `--dry-run` | Preview without executing |
```bash
dots status --type minimal
dots link -m Zsh -m Nvim
dots status --state unlinked
dots install --module Packages
```

## Conflict resolution

| Situation | Behavior |
|---|---|
| File exists, not a symlink | Creates `.bak`, then links |
| Symlink exists, points elsewhere | Replaces with correct symlink |
| `.bak` already exists | **Blocks** — review and clean manually |

No timestamp clutter. One `.bak` per file, intentional friction.

---

# dots

CLI for managing dotfiles across Linux, macOS, and Windows.

---

## Requirements

- Python 3.10+
- [`pipx`](https://pipx.pypa.io) (for stable installation)

---

## Setup: dotfiles repository

`dots` needs a **marker file** at the root of your dotfiles repository to locate it. You can generate this automatically:

```bash
cd ~/your-dotfiles
dots init
```

This creates a `dots.toml` file in that directory, and also **writes a `~/.dotsrc` file in your home directory** storing the absolute path to your repo. From then on, you can run `dots` from within that directory (or any subdirectory).

> **Global Usage**: Because `dots init` creates a `~/.dotsrc` pointing to your repo, you can run `dots` from **anywhere on your system**. If you ever move your dotfiles repo, simply run `dots init` again in the new location. You can also override the path dynamically by setting the `DOTS_REPO` environment variable.

---

## Installation

### Stable (production) — via pipx

Install once. Only updates when you explicitly run `pipx upgrade`.
Your dotfiles are **never affected** by development work you do in the `dev` branch.

```bash
pipx install ~/dots
```

To upgrade after a new release (merged to `main` and tagged):

```bash
pipx upgrade dots
# or reinstall from local path:
pipx reinstall dots
```

### Development — via local venv

```bash
cd ~/Work/dots
python3 -m venv .venv
.venv/bin/pip install -e ".[dev]"

# Use the dev binary explicitly:
.venv/bin/dots --help
```

---

## Git Workflow

```
main  ←── stable, tagged releases (v0.1.0, v0.2.0, ...)
dev   ←── active development
```

### Starting new work

```bash
git checkout dev
# ... make changes, run tests ...
.venv/bin/pytest tests/ -v
```

### Releasing a new version

```bash
git checkout main
git merge dev
# Bump version in pyproject.toml
git tag v0.x.x
git push origin main --tags

# Update stable installation:
pipx reinstall dots
```

---

## Usage

```bash
dots --help
dots init
dots status
dots link
dots unlink
```

# dots

dotfile manager — declarative, symlink-based, yours.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Requires Python 3.10+ and git.

## Setup

```bash
cd ~/dotfiles
dots init
dots link
```

That's it. Two commands and your dotfiles are linked.

---

## Commands

| Command | Description |
|---------|-------------|
| `dots init` | Initialize repo — creates `dots.toml` marker |
| `dots link` | Create symlinks for all modules |
| `dots unlink` | Remove symlinks |
| `dots status` | Show link state grouped by status |
| `dots adopt <path>` | Import an existing config into the repo |
| `dots install` | Install dependencies from `path.yaml` files |
| `dots backup` | Git commit and optional push |

## Quick examples

```bash
# Link everything
dots link

# Link specific modules
dots link -m Zsh -m Nvim

# Check what's linked
dots status

# Filter by state
dots status --state unlinked

# Import an existing config
dots adopt ~/.zshrc

# Install all dependencies
dots install

# Install only from one module
dots install -m Zsh

# Preview without executing
dots link --dry-run
```

---

## Flags

| Flag | Description |
|------|-------------|
| `-m / --module` | Filter by module name (repeatable) |
| `-t / --type` | Filter by module type (repeatable) |
| `-s / --state` | Filter by state: `linked`, `unlinked`, `broken`, `missing`, `unsafe` (repeatable) |
| `-f / --format` | Output format: `default`, `table`, `json` (solo para `status`) |
| `--force` | Overwrite existing symlinks in conflict (solo para `link`) |
| `--variant` | Select variant for modules with multiple variants (solo para `link`) |
| `-i / --interactive` | Interactively select modules to link/unlink (`link`, `unlink`) |
| `--dry-run` | Preview without executing |

---

## Conflict resolution

| Situation | Behavior |
|-----------|----------|
| File exists, not a symlink | Creates `-backup` file, then links |
| Symlink exists, points elsewhere | Replaces with correct symlink |
| Symlink already correct | Skips — already linked |
| `-backup` file already exists | Blocks — manual intervention required |

---

## Documentation

- [path.yaml reference](docs/path-yaml-reference.md) — module structure, dependency types

---

## Installation

### Stable (pipx)

```bash
pipx install ~/dots
pipx upgrade dots
```

### Development

```bash
cd ~/Work/dots
python3 -m venv .venv
.venv/bin/pip install -e ".[dev]"

.venv/bin/dots --help
```

---

## Git Workflow

```
main  ← stable, tagged releases
dev   ← active development
```

```bash
# Start working
git checkout dev

# Release
git checkout main
git merge dev
# Update version in pyproject.toml
git tag v0.x.x
git push origin main --tags
pipx reinstall dots
```

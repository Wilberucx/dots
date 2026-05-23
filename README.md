# dots

dotfile manager — declarative, symlink-based, yours.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Zero dependencies — just curl or wget. Installs to `~/.local/bin/dots`.

## Setup

```bash
cd ~/dotfiles
dots init
dots link
```

That's it. Two commands and your dotfiles are linked.

---

## Commands

| Command             | Description                                  |
| ------------------- | -------------------------------------------- |
| `dots init`         | Initialize repo — creates `.dots/config.yaml` |
| `dots link`         | Create symlinks for all modules              |
| `dots unlink`       | Remove symlinks                              |
| `dots status`       | Show link state grouped by status            |
| `dots list`         | List modules or backups with filters         |
| `dots edit`         | Open module folder or path.yaml in $EDITOR   |
| `dots adopt <path>` | Import an existing config into the repo      |
| `dots install`      | Install dependencies from `path.yaml` files  |
| `dots migrate`      | Upgrade path.yaml from v2 to v3 schema       |
| `dots backup`       | Git commit and optional push                 |

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

# List modules
dots list
dots list --variant

# Edit a module
dots edit Zsh
dots edit Nvim --config   # open path.yaml directly

# Migrate to schema v3
dots migrate
dots migrate --dry-run
```

---

## Flags

| Flag                 | Description                                                                       |
| -------------------- | --------------------------------------------------------------------------------- |
| `-m / --module`      | Filter by module name (repeatable)                                                |
| `-t / --type`        | Filter by module type (repeatable)                                                |
| `-s / --state`       | Filter by state: `linked`, `unlinked`, `broken`, `missing`, `unsafe` (repeatable) |
| `-f / --format`      | Output format: `default`, `table`, `json` (solo para `status`)                    |
| `--force`            | Overwrite existing symlinks in conflict (solo para `link`)                        |
| `--variant`          | Select variant for modules with multiple variants (solo para `link`)              |
| `-i / --interactive` | Interactively select modules to link/unlink (`link`, `unlink`)                    |
| `--dry-run`          | Preview without executing                                                         |

---

## Conflict resolution

| Situation                        | Behavior                              |
| -------------------------------- | ------------------------------------- |
| File exists, not a symlink       | Creates `.orig` file, then links     |
| Symlink exists, points elsewhere | Replaces with correct symlink         |
| Symlink already correct          | Skips — already linked                |
| `.orig` file already exists      | Blocks — manual intervention required |

---

## Documentation

- [path.yaml reference](docs/path-yaml-reference.md) — module structure, dependency types
- [Schema v3](docs/schema-v3.md) — current schema specification
- [Dependencies](docs/dependencies.md) — dependency types (git, binary, package)

---

## Installation

### Stable (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Downloads the pre-built Go binary from GitHub Releases. Zero dependencies.

### Via Go

```bash
go install github.com/Wilberucx/dots/cmd/dots@latest
```

Requires the Go toolchain.

### Development

```bash
cd ~/Work/dots
go build -o /tmp/dots ./cmd/dots/
/tmp/dots --help
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
# Update version
git tag v0.x.x
git push origin main --tags
```

# dots

dotfile manager — declarative, symlink-based, yours.

> **Versión en español**: [README.es.md](README.es.md)

## Install

### Stable (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Zero dependencies — just curl or wget. Downloads the pre-built Go binary from GitHub Releases and installs to `~/.local/bin/dots`.

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

## Mental Model

```
modules declare files  →  dots builds a plan  →  dots applies the plan
```

1. Each **module** (a directory in your repo) declares files to symlink
2. `dots` **resolves** those declarations into a **Plan** (what gets created, backed up, or skipped)
3. `dots link` **applies** the plan transactionally with rollback on failure

Everything flows from this model. `dots status` shows the current state, `dots plan` shows what would happen, and `dots link` executes it.

## Setup

```bash
cd ~/dotfiles
dots init
dots link
```

That's it. Two commands and your dotfiles are linked.

---

## Commands

| Command             | Description                                        |
| ------------------- | -------------------------------------------------- |
| `dots init`         | Initialize repo — creates `init.lua` or `.dots/config.yaml` |
| `dots link`         | Create symlinks for all modules                    |
| `dots unlink`       | Remove symlinks                                    |
| `dots status`       | Show link state grouped by status                  |
| `dots list`         | List modules or backups with filters               |
| `dots edit`         | Open module folder or config file in $EDITOR       |
| `dots adopt <path>` | Import an existing config into the repo            |
| `dots install`      | Install dependencies from config files             |
| `dots migrate`      | Migrate config between schema versions             |
| `dots backup`       | Git commit and optional push                       |

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
dots edit Nvim --config   # open config file directly
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

## Design Principles

- **Single binary** — zero runtime dependencies, installable via curl | bash
- **Symlink-based** — dotfiles stay in your repo, symlinked to their destinations
- **Dry-run first** — every mutating command supports `--dry-run` to preview before executing
- **No hidden writes** — no writes outside requested commands; all changes are explicit and transactional
- **Lua primary, YAML legacy** — new configs use `dots.lua`; `path.yaml` is supported but deprecated
- **Plan abstraction** — the resolver produces a Plan that is shared across commands for consistency

## Documentation

### Lua (recommended)

- [Lua syntax reference](docs/lua-syntax.md) — `init.lua` / `dots.lua` configuration
- [Plugin system](docs/lua-syntax.md#7-plugin-system) — extending dots with Lua scripts

### Legacy YAML

- [path.yaml reference](docs/path-yaml-reference.md) — module structure, dependency types
- [Schema v3](docs/schema-v3.md) — current schema specification
- [Dependencies](docs/dependencies.md) — dependency types (git, binary, package)

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

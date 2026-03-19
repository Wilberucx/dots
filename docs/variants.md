# Variants System

Variants allow multiple source files to share the same destination path. This enables a single module to serve different configurations—like "personal" vs "work"—without duplicating destination declarations.

## What Are Variants

A **variant** is an alternate source file that maps to the **same destination** as another source file. When `detect_variants()` finds two or more sources pointing to the same destination, it marks them as variants.

```yaml
# path.yaml
files:
  - source: personal/config.yaml
    destination: ~/.config/myapp/config.yaml
  - source: work/config.yaml
    destination: ~/.config/myapp/config.yaml
```

Both `personal/config.yaml` and `work/config.yaml` are variants—they resolve to the same destination.

## Why Variants Exist

The canonical use case: you want different configs for different contexts, but your muscle memory (and tools) expect files in the same location.

| Scenario | Without Variants | With Variants |
|----------|------------------|---------------|
| Personal + Work laptop | Two modules with duplicate destinations | One module, `~/.config/app` switches between sources |
| Dotfiles on multiple hosts | Copy configs, diverge over time | Share destination, keep per-host variants |
| Experimenting with config | Rename files manually | Add variant, switch instantly |

## Declaring Variants in path.yaml

Variants emerge from **same destination + multiple sources**. The order matters: the **last source in the YAML list is the default** (cascade behavior).

```yaml
# zsh/path.yaml
type: shell

files:
  - source: common aliases
    destination: ~/.zsh/aliases

  - source: personal/aliases
    destination: ~/.zsh/aliases

  - source: work/aliases
    destination: ~/.zsh/aliases
```

In this example, `work/aliases` is the default variant because it appears last.

### Cascade Behavior

When you run `dots link` without specifying a variant, dots uses the **last declared source** for each duplicated destination. This is the "cascade" — implicit fallback to the bottom of the list.

To override and select a specific variant explicitly, use `dots link --variant <name>`.

### Warning Output

When you run `dots link -m MyModule` without `--variant` on a module with variants, dots prints a warning:

```
WARNING: Module 'MyModule' has multiple variants. Using default: 'work' (last in YAML). Use --variant to select a specific one.
```

## Detecting the Active Variant

The active variant is determined by **inspecting existing symlinks at the destination**. The `get_active_variant()` function:

1. Parses `path.yaml` and detects variants via `detect_variants()`
2. For each variant's destination, checks if a symlink exists
3. If the symlink points to the variant's source directory, that variant is active

```python
# Simplified logic from resolver.py
for variant_source, destination in variant_info.variant_destinations.items():
    dest_path = expand_path(destination)
    src_path = (module_dir / variant_source.lstrip('/')).resolve()

    if dest_path.is_symlink() and dest_path.readlink().resolve() == src_path:
        return variant_source  # This one is active
```

If no symlink matches any variant, no variant is considered active.

## Switching Variants

### Via CLI

```bash
# Switch to a specific variant
dots link -m Nvim --variant work/aliases

# Link all modules, letting each use its cascade default
dots link

# ERROR: Module has no variants
dots link -m Zsh --variant personal
# Output: Module 'Zsh' has no variants. Available sources: common, aliases
```

### Error Cases

```bash
# Variant not found - shows available variants
dots link -m git --variant nonexistent
# Output: Variant 'nonexistent' not found in module 'git'. Available variants: common, personal, work
```

### Via TUI

When using `dots link --interactive` (TUI mode), you can select modules and variants interactively. The variant selector appears when a module has declared variants.

## Displaying Variants in `dots status`

Modules with variants are expanded in the status output:

```
✔ Linked (2 modules)
  📦 nvim
      ● work      ← active
      ○ personal
  📦 zsh
      ○ common
      ○ personal
      ● work      ← active
```

**Legend:**
- `●` — active variant (symlink points here)
- `○` — inactive variant

If you filter by state (e.g., `dots status -s linked`), modules with variants still show which variant is active.

## Variant Tag in `dots link` Output

When linking with an explicit variant, the output shows a tag with the **variant name** (e.g., `[work]`):

```
📦 nvim
   ✔ init.lua → ~/.config/nvim/init.lua [work]
   Status: 1 linked
```

This helps you verify which variant was applied during the link operation.

## Complete Example

Consider a `git` module with personal and work settings:

```
git/
├── path.yaml
├── common/
│   └── config.yaml          # shared settings
├── personal/
│   └── config.yaml          # personal email, aliases
└── work/
    └── config.yaml          # work email, different remote
```

```yaml
# git/path.yaml
type: vcs

files:
  - source: common/config.yaml
    destination: ~/.gitconfig

  - source: personal/config.yaml
    destination: ~/.gitconfig

  - source: work/config.yaml
    destination: ~/.gitconfig
```

### Behavior

| Command | Result |
|---------|--------|
| `dots link` | Links `work/config.yaml` (cascade default) |
| `dots link -m git --variant personal/config.yaml` | Links `personal/config.yaml` |
| `dots status` | Shows `● work` as active |

## Adopting Files Creates Variants

When you run `dots adopt` on a file whose **destination is already declared** in an existing module, dots detects this and **proposes creating a variant** instead:

```bash
$ dots adopt ~/.config/myapp/work.yaml -n Myapp
```

If `Myapp` already declares `~/.config/myapp/config.yaml` as a destination, dots asks:

```
Module 'Myapp' already declares '~/.config/myapp/config.yaml' as a destination.
Create a new variant in 'Myapp' for this file? [Y/n]
```

Accepting prompts for a variant name (e.g., `work`), then:

1. Creates `Myapp/work/`
2. Moves the adopted file there
3. Adds an entry to `path.yaml` with the new source path

The resulting path.yaml:

```yaml
files:
  - source: config.yaml
    destination: ~/.config/myapp/config.yaml
  - source: work/config.yaml
    destination: ~/.config/myapp/config.yaml
```

Now you can switch with `dots link -m Myapp --variant work/work.yaml`.

## Summary

| Concept | Detail |
|---------|--------|
| **Variant** | Alternate source sharing same destination |
| **Declaration** | Multiple sources → same destination in path.yaml |
| **Default** | Last source in YAML list (cascade) |
| **Detection** | Symlink inspection at destination path |
| **Switch** | `dots link -m <module> --variant <name>` |
| **Status** | `●` = active, `○` = inactive |
| **Adopt** | Auto-creates variant when destination exists |

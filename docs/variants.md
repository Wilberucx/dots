# Variants System

Variants allow multiple source files to share the same destination path.
This enables a single module to serve different configurations—like
"personal" vs "work"—without duplicating destination declarations.

## What Are Variants

A **variant** is an alternate source file that maps to the **same destination**
as another source file. When two or more sources point to the same destination,
they form a set of variants.

```lua
-- Git/dots.lua
return {
  files = {
    -- Base file: always included
    file("common/config.yaml", "~/.gitconfig"),

    -- Variants: same destination, different sources
    file("personal/config.yaml", "~/.gitconfig"):variant("personal"),
    file("work/config.yaml",     "~/.gitconfig"):variant("work"),
  },
}
```

Both `personal/config.yaml` and `work/config.yaml` are variants of each
other — they resolve to the same `~/.gitconfig`.

## Why Variants Exist

The canonical use case: you want different configs for different contexts,
but your muscle memory (and tools) expect files in the same location.

| Scenario | Without Variants | With Variants |
|----------|------------------|---------------|
| Personal + Work laptop | Two modules with duplicate destinations | One module, `~/.config/app` switches between sources |
| Dotfiles on multiple hosts | Copy configs, diverge over time | Share destination, keep per-host variants |
| Experimenting with config | Rename files manually | Add variant, switch instantly |

---

## Declaring Variants in Lua (`dots.lua`)

You have **two ways** to declare variants in Lua:

### Explicit Variants with `:variant("name")` (Recommended)

Use the `:variant("name")` method on `file()`, `dir()`, or `glob()` to
assign a meaningful name to each variant.

```lua
-- ~/dotfiles/Git/dots.lua
return {
  type = "vcs",
  files = {
    -- Base file: always included, not part of any variant
    file("common/config.yaml", "~/.gitconfig"),

    -- Variant: work
    file("work/config.yaml", "~/.gitconfig"):variant("work"),

    -- Variant: personal
    file("personal/config.yaml", "~/.gitconfig"):variant("personal"),
  },
}
```

**Rules:**

- Files **without** `:variant()` are always included (base configuration)
- Files **with** `:variant("name")` are only included when that variant is
  active or explicitly selected
- The **last variant declared** becomes the default (cascade behavior)
- You can mix variants with other methods like `:when()` and `:per_os()`:

```lua
file("work/config", "~/.config/app"):when("linux"):variant("work")
file("personal/config", "~/.config/app"):per_os({
  linux = "~/.config/app-linux",
  mac   = "~/Library/app",
}):variant("personal")
```

**Examples with `dir()` and `glob()`:**

```lua
-- Directory variants
dir("config-work"):to("~/.config/app"):variant("work")
dir("config-personal"):to("~/.config/app"):variant("personal")

-- Glob variants
glob("work/*.toml"):into("~/.config/"):variant("work")
glob("personal/*.toml"):into("~/.config/"):variant("personal")
```

### Implicit Variants (Legacy Fallback)

Multiple files with the **same destination** are automatically detected as
variants, using their source path as the variant name:

```lua
-- Same destination → variants detected automatically
return {
  files = {
    file("personal/config.yaml", "~/.gitconfig"),
    file("work/config.yaml", "~/.gitconfig"),       -- same dest → variant
  },
}
```

The last entry in the `files` array becomes the default.

### Explicit vs Implicit

| Aspect | Explicit (`:variant()`) | Implicit |
|--------|------------------------|----------|
| **Variant names** | Meaningful names (`"work"`, `"personal"`) | Source paths (`"work/config.yaml"`) |
| **Base files** | Clear: files without `:variant()` are always included | Ambiguous: all files are candidates |
| **Default** | Last declared variant | Last source for each duplicate destination |
| **CLI usage** | `--variant work` | `--variant work/config.yaml` |

> **Recommendation**: Use explicit `:variant("name")` for clarity.
> Implicit detection exists for backward compatibility with auto-migrated
> modules.

---

## Declaring Variants in YAML (`path.yaml`) — Legacy

> **Note**: YAML format is supported for backward compatibility but is
> deprecated. New modules should use Lua.

In YAML, variants emerge from **same destination + multiple sources**.
The order matters: the **last source in the YAML list is the default**
(cascade behavior).

```yaml
# zsh/path.yaml
type: shell
files:
  - source: common/aliases
    destination: ~/.zsh/aliases
  - source: personal/aliases
    destination: ~/.zsh/aliases
  - source: work/aliases
    destination: ~/.zsh/aliases   # ← default (last)
```

### Cascade Behavior

When you run `dots link` without specifying a variant, dots uses the **last
declared source** for each duplicated destination. This is the "cascade" —
implicit fallback to the bottom of the list.

To override, use `dots link --variant <name>`.

### Warning Output

When you run `dots link -m MyModule` without `--variant` on a module with
variants, dots prints a warning:

```
WARNING: Module 'MyModule' has multiple variants. Using default: 'work'.
Use --variant to select a specific one.
```

---

## Switching Variants

### Via CLI

```bash
# Switch to a specific variant (explicit name)
dots link -m Git --variant work

# Or for implicit variants (source path)
dots link -m Nvim --variant work/config.yaml

# Link all modules, letting each use its cascade default
dots link
```

### Error Cases

```bash
# Variant not found - shows available variants
dots link -m git --variant nonexistent
# Output: Variant 'nonexistent' not found in module 'git'.
# Available variants: common, personal, work
```

### Via TUI

When using `dots link --interactive` (TUI mode), you can select modules and
variants interactively. The variant selector appears when a module has
declared variants.

---

## Detecting the Active Variant

The active variant is determined by **inspecting existing symlinks** at the
destination. The function:

1. Reads the module config (`dots.lua` or `path.yaml`)
2. Detects variants (explicit or implicit)
3. For each variant's files, checks if the symlink at the destination points
   to that variant's source
4. If all symlinks for a variant match, that variant is marked active

```
# Simplified logic: for each declared variant source, check if the
# destination symlink already points to that source's directory.
```

If no symlink matches any variant, no variant is considered active.

---

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

If you filter by state (e.g., `dots status -s linked`), modules with variants
still show which variant is active.

---

## Variant Tag in `dots link` Output

When linking with an explicit variant, the output shows a tag with the
**variant name** (e.g., `[work]`):

```
📦 nvim
   ✔ init.lua → ~/.config/nvim/init.lua [work]
   Status: 1 linked
```

This helps you verify which variant was applied during the link operation.

---

## Complete Example

Consider a `git` module with personal and work settings:

```
git/
├── dots.lua
├── common/
│   └── config.yaml          # shared settings
├── personal/
│   └── config.yaml          # personal email, aliases
└── work/
    └── config.yaml          # work email, different remote
```

```lua
-- git/dots.lua
return {
  type = "vcs",
  files = {
    file("common/config.yaml", "~/.gitconfig"),
    file("personal/config.yaml", "~/.gitconfig"):variant("personal"),
    file("work/config.yaml", "~/.gitconfig"):variant("work"),
  },
}
```

### Behavior

| Command | Result |
|---------|--------|
| `dots link` | Links `common/config.yaml` + `work/config.yaml` (default variant) |
| `dots link -m git --variant personal` | Links `common/config.yaml` + `personal/config.yaml` |
| `dots status` | Shows `● work` as active |

---

## Adopting Files Creates Variants

When you run `dots adopt` on a file whose **destination is already declared**
in an existing module, dots detects this and **proposes creating a variant**
instead:

```bash
$ dots adopt ~/.config/myapp/work.yaml -n Myapp
```

If `Myapp` already declares `~/.config/myapp/config.yaml` as a destination,
dots asks:

```
Module 'Myapp' already declares '~/.config/myapp/config.yaml' as a destination.
Create a new variant in 'Myapp' for this file? [Y/n]
```

Accepting prompts for a variant name (e.g., `work`), then:

1. Creates `Myapp/work/`
2. Moves the adopted file there
3. Adds an entry to the config with the new source path and `:variant("work")`

The resulting `dots.lua`:

```lua
return {
  files = {
    file("config.yaml", "~/.config/myapp/config.yaml"),
    file("work/config.yaml", "~/.config/myapp/config.yaml"):variant("work"),
  },
}
```

Now you can switch with `dots link -m Myapp --variant work`.

---

## Summary

| Concept | Detail |
|---------|--------|
| **Variant** | Alternate source sharing same destination |
| **Declaration** | `:variant("name")` on `file()`/`dir()`/`glob()`, or implicit (multiple sources → same dest) |
| **Default** | Last declared `:variant()` (cascade) |
| **Detection** | Symlink inspection at destination path |
| **Switch** | `dots link -m <module> --variant <name>` |
| **Status** | `●` = active, `○` = inactive |
| **Adopt** | Auto-creates variant when destination already exists |

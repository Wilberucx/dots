# TUI Variant Swap Bug Diagnosis

## Location of Error Message

**File**: `src/dots/ui/dashboard.py`  
**Line**: 1767

```python
state.log("error", f"Variant switch failed: {name} → {target_variant}")
```

This error is logged when `success == False` is returned from `_capture_cmd()`.

---

## Root Cause Analysis

### The Data Flow

1. **TUI builds variant list** (`dashboard.py:469`):
   - `vinfo.variants` is populated from `detect_variants()` in `yaml_parser.py`
   - Variants are the **source path names** (e.g., `["nvim", "notevim"]`)

2. **User selects variant** via `flavors_cursor` → `target_variant = vrs[state.flavors_cursor]`

3. **TUI calls link_cmd** with `variant=target_variant` (e.g., `"notevim"`)

4. **In link_cmd** (`link.py:73-80`):
   ```python
   if variant and not force:
       for module_name in selected_modules or []:
           active = get_active_variant(config, module_name)
           if active and active != variant:
               force = True
               is_variant_swap = True
               print_info(f"Auto-swap: {module_name} variant '{active}' → '{variant}'")
   ```

### The Bug

**`get_active_variant()` returns the full source path**, not just the variant folder name.

Looking at `resolver.py:58-94`:
```python
def get_active_variant(config, module_name) -> str | None:
    # ...
    for variant_source, destination in variant_info.variant_destinations.items():
        src_path = (module_dir / variant_source.lstrip('/')).resolve()
        if dest_path.is_symlink() and link_target == src_path:
            return variant_source  # <-- Returns "nvim" or "notevim" or FULL path
```

If `variant_source` is stored as `"notevim"` it works. But if it's stored as `"notevim/"` (with trailing slash), then comparison `"notevim" != "notevim/"` → **auto-swap triggers incorrectly**.

### Why 'nvim' vs 'notevim' Appears

The log shows:
```
ℹ Auto-swap: Nvim variant 'nvim' → 'notevim'
Variant switch failed: Nvim → notevim
```

This suggests:
- `name` = `"Nvim"` (module name)
- `target_variant` = `"notevim"` (variant folder name from `vrs[flavors_cursor]`)
- The "Auto-swap" message shows `'nvim'` - this is the **active variant returned by `get_active_variant()`**

The issue: **`get_active_variant()` returns the source folder name that is currently linked**, but when `variant_destinations` maps `"nvim/"` (with slash) to destination, the return value might be inconsistent with how `vinfo.variants` is populated.

### The Real Bug: `_capture_cmd` False Failure

The most likely issue is in `_capture_cmd` (lines 1597-1618):

```python
success = True
try:
    fn(*args, **kwargs)
except SystemExit as e:
    code = getattr(e, "exit_code", None)
    if code is not None and code != 0:
        success = False  # <-- Only sets success=False for non-zero exit codes
except Exception:
    success = False
```

**Problem**: If `link_cmd` prints errors but still exits with code 0 (e.g., validation errors that don't raise `SystemExit(1)`), the TUI thinks it succeeded but the actual linking never happened.

Looking at `link_cmd` lines 90-101:
```python
if variant not in variant_info.variants:
    print_error(f"Variant '{variant}' not found...")
    raise typer.Exit(1)  # <-- This DOES raise SystemExit(1)
```

And lines 177-179:
```python
else:
    module_tree.add(f"[red]⚠[/red] {src.name} → {final_dest} [red](conflict: {status.detail})[/red]")
    module_stats["conflicts"] += 1
```

When `force=False` and there's a conflict, `link_cmd` **logs the conflict but does NOT exit with code 1**. It continues, prints summary, and exits normally (code 0).

**The TUI thinks the variant switch succeeded because `link_cmd` exited with 0, but in reality no linking happened due to the unresolved conflict.**

---

## Why CLI Works

The CLI version (`dots link -m Nvim --variant notevim`) works because:
1. CLI shows conflict warnings but continues
2. User sees the actual error message and understands
3. TUI tries to interpret "success" purely by exit code, which fails for conflict cases

---

## Suggested Fix

### Option A: Check for Conflicts in TUI

In `dashboard.py:1754-1767`, after calling `link_cmd`, check if there were conflicts:

```python
success, lines = _capture_cmd(
    link_cmd,
    module=[name],
    dry_run=False,
    force=False,
    interactive=False,
    variant=target_variant,
)

# Check if any lines indicate failure
failure_indicators = ["conflict", "error", "failed"]
conflict_detected = any(
    any(indicator in line.lower() for indicator in failure_indicators)
    for line in lines
)

if success and not conflict_detected:
    state.log("success", f"Switched {name} → {target_variant}")
elif conflict_detected:
    state.log("error", f"Variant switch failed: {name} → {target_variant} (conflicts exist)")
else:
    state.log("error", f"Variant switch failed: {name} → {target_variant}")
```

### Option B: Always Use `force=True` for Variant Swaps

The auto-swap logic already sets `force=True` internally. The TUI should do the same:

```python
success, lines = _capture_cmd(
    link_cmd,
    module=[name],
    dry_run=False,
    force=True,  # <-- Always force for variant swaps
    interactive=False,
    variant=target_variant,
)
```

This is simpler but might mask real issues.

### Option C: Return Conflict Status from `link_cmd`

Modify `link_cmd` to return a status enum or tuple that includes conflict info, so the TUI can make an informed decision.

---

## Most Likely Fix

**Option A** is the most robust because it:
1. Preserves `force=False` (user confirmation behavior)
2. Actually checks the output for real failures
3. Provides better user feedback

---

## Files Involved

| File | Line | Role |
|------|------|------|
| `src/dots/ui/dashboard.py` | 1767 | Logs "Variant switch failed" |
| `src/dots/ui/dashboard.py` | 1754-1761 | Calls `link_cmd` |
| `src/dots/ui/dashboard.py` | 1597-1618 | `_capture_cmd` determines success |
| `src/dots/commands/link.py` | 73-80 | Auto-swap logic |
| `src/dots/commands/link.py` | 177-179 | Conflict handling (no exit code 1) |
| `src/dots/core/resolver.py` | 58-94 | `get_active_variant()` |
| `src/dots/core/yaml_parser.py` | 152-195 | `detect_variants()` |

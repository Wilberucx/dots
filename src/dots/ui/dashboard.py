"""
Two-panel TUI dashboard for dots CLI.

Layout:
  Panel 1 — Module table (Modules | Status | Destination | Last backup)
  Panel 2 — Tabbed info panel (Home, Flavors, Logs, Tree, Backup, Help)
  Footer  — Context-sensitive keybindings

Modes:
  Normal  — full keybindings, all tabs
  Visual  — selection circles (●/○), reduced tabs/bindings
"""

from __future__ import annotations

import os
import subprocess
from datetime import datetime
from pathlib import Path
from typing import Optional, Callable, Literal

from prompt_toolkit import Application
from prompt_toolkit.application.current import get_app
from prompt_toolkit.key_binding import KeyBindings
from prompt_toolkit.keys import Keys
from prompt_toolkit.layout import (
    Layout,
    HSplit,
    VSplit,
    Window,
    FormattedTextControl,
    D,
)
from prompt_toolkit.formatted_text import FormattedText
from prompt_toolkit.styles import Style

from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules, LinkStatus, get_module_variant_info
from dots.core.transaction import TransactionLog

from dots.ui.theme import TUI_STYLE_DICT, ACCENT, FG


# ═══════════════════════════════════════════════════════════════════════
#  STYLE
# ═══════════════════════════════════════════════════════════════════════

TUI_STYLE = Style.from_dict(TUI_STYLE_DICT)


# ═══════════════════════════════════════════════════════════════════════
#  ROUNDED FRAME
# ═══════════════════════════════════════════════════════════════════════


def rounded_frame(
    body, title: str = "", active_fn: Callable[[], bool] = lambda: False
) -> HSplit:
    """Wrap body in a box with rounded corners and optional title."""

    def border_style() -> str:
        return "class:border.active" if active_fn() else "class:border"

    def title_style() -> str:
        return "class:panel.title.active" if active_fn() else "class:panel.title"

    def get_top_border():
        res = [(border_style(), "╭─")]
        if title:
            res.append((title_style(), f" {title} "))
            res.append((border_style(), "─"))
        return res

    def get_bot_border():
        return [(border_style(), "╰")]

    top_row = VSplit(
        [
            Window(content=FormattedTextControl(get_top_border), height=1),
            Window(
                content=FormattedTextControl(lambda: [(border_style(), "─" * 300)]),
                height=1,
            ),
            Window(
                content=FormattedTextControl(lambda: [(border_style(), "╮")]),
                width=1,
                height=1,
            ),
        ]
    )

    bot_row = VSplit(
        [
            Window(content=FormattedTextControl(get_bot_border), height=1),
            Window(
                content=FormattedTextControl(lambda: [(border_style(), "─" * 300)]),
                height=1,
            ),
            Window(
                content=FormattedTextControl(lambda: [(border_style(), "╯")]),
                width=1,
                height=1,
            ),
        ]
    )

    left_bar = Window(
        content=FormattedTextControl(lambda: [(border_style(), "│\n" * 300)]),
        width=1,
        always_hide_cursor=True,
        wrap_lines=False,
    )
    right_bar = Window(
        content=FormattedTextControl(lambda: [(border_style(), "│\n" * 300)]),
        width=1,
        always_hide_cursor=True,
        wrap_lines=False,
    )
    mid_row = VSplit([left_bar, body, right_bar])
    return HSplit([top_row, mid_row, bot_row])


def gap_h(rows: int = 1) -> Window:
    return Window(height=D(min=rows, max=rows, preferred=rows), always_hide_cursor=True)


# ═══════════════════════════════════════════════════════════════════════
#  TABS DEFINITION
# ═══════════════════════════════════════════════════════════════════════

# (id, label_when_active, label_when_inactive, key_hint, available_in_visual)
TABS = [
    ("home", "dots", "Home", "H", True),
    ("flavors", "flavors", "flavors", "F", False),
    ("logs", "logs", "logs", "L", True),
    ("tree", "tree", "tree", "T", False),
    ("backup", "backup", "backup", "B", False),
    ("help", "help", "help", "?", True),
]


# ═══════════════════════════════════════════════════════════════════════
#  STATE
# ═══════════════════════════════════════════════════════════════════════


class TUIState:
    PANEL_TABLE = 0
    PANEL_TABS = 1

    def __init__(self, config: DotsConfig):
        self.config = config

        # Mode
        self.mode: Literal["normal", "visual"] = "normal"
        self.sub_mode: str | None = None  # None, "edit", "order"

        # Panels
        self.active_panel: int = self.PANEL_TABLE

        # Module data
        self.module_names: list[str] = []
        self.module_statuses: dict[str, list[LinkStatus]] = {}
        self.module_link_state: dict[str, str] = {}  # linked/unlinked/broken
        self.module_destinations: dict[str, str] = {}
        self.module_last_backup: dict[str, str] = {}
        self.module_has_variants: dict[str, bool] = {}
        self.module_variants: dict[str, list[str]] = {}

        # Module navigation
        self.selected_module: int = 0
        self._mod_vt: int = 0
        self.mod_vis: int = 10

        # Visual selection
        self.selected_set: set[int] = set()

        # Tabs
        self.active_tab: str = "home"

        # Sorting
        self.sort_key: str = "name"
        self.sort_reverse: bool = False

        # Filter
        self.filter_active: bool = False
        self.filter_text: str = ""

        # Tree (for tree tab)
        self.tree_items: list[dict] = []
        self.selected_tree: int = 0
        self._tree_vt: int = 0
        self.tree_vis: int = 10

        # Backups (for backup tab)
        self.backup_entries: list[str] = []
        self.selected_backup: int = 0
        self._bkp_vt: int = 0
        self.bkp_vis: int = 6

        # Log (for logs tab)
        self.log_entries: list[tuple[str, str, str]] = []

        # Help (for help tab)
        self._help_vt: int = 0
        self.help_vis: int = 10
        self.help_items_count: int = 0

        # Home (for individual file selection)
        self.home_selected_entry: int = 0
        self.home_selected_set: set[int] = set()
        self._home_vt: int = 0
        self.home_vis: int = 10

        # Flavors (for flavor selection)
        self.flavors_cursor: int = 0
        self._flavors_vt: int = 0
        self.flavors_vis: int = 10
        self.module_active_variant: dict[str, str] = {}

        self.refresh_modules()
        self.refresh_backups()
        self.log("info", "Dots TUI started")

    # ── Helpers ──

    def current_name(self) -> Optional[str]:
        names = self._filtered_names()
        if names and 0 <= self.selected_module < len(names):
            return names[self.selected_module]
        return None

    def _filtered_names(self) -> list[str]:
        return self.module_names

    def current_statuses(self) -> list[LinkStatus]:
        n = self.current_name()
        return self.module_statuses.get(n, []) if n else []

    def current_module_dir(self) -> Optional[Path]:
        n = self.current_name()
        return self.config.repo_root / n if n else None

    def log(self, level: str, msg: str):
        ts = datetime.now().strftime("%H:%M:%S")
        self.log_entries.append((ts, level, msg))
        if len(self.log_entries) > 200:
            self.log_entries = self.log_entries[-200:]

    # ── Scrolling ──

    def _calc_scroll(self, s: int, vt: int, v: int, n: int, margin: int = 2) -> int:
        if v <= 0 or n <= v:
            return 0
        if s < vt + margin:
            return max(0, s - margin)
        elif s >= vt + v - margin:
            return min(n - v, s - v + margin + 1)
        return vt

    def _reset_module_sub_states(self):
        self.home_selected_entry = 0
        self.home_selected_set.clear()
        self._home_vt = 0
        self.flavors_cursor = 0
        self._flavors_vt = 0
        self._update_tree_data()

    def _scroll_mod(self, d: int):
        names = self._filtered_names()
        if not names:
            return
        n = len(names)
        old_s = self.selected_module
        self.selected_module = max(0, min(n - 1, self.selected_module + d))
        if self.selected_module != old_s:
            self._reset_module_sub_states()
        self._mod_vt = self._calc_scroll(
            self.selected_module, self._mod_vt, self.mod_vis, n
        )

    def _scroll_tree(self, d: int):
        if not self.tree_items:
            return
        n = len(self.tree_items)
        self.selected_tree = max(0, min(n - 1, self.selected_tree + d))
        self._tree_vt = self._calc_scroll(
            self.selected_tree, self._tree_vt, self.tree_vis, n
        )

    def _scroll_bkp(self, d: int):
        if not self.backup_entries:
            return
        n = len(self.backup_entries)
        self.selected_backup = max(0, min(n - 1, self.selected_backup + d))
        self._bkp_vt = self._calc_scroll(
            self.selected_backup, self._bkp_vt, self.bkp_vis, n
        )

    def _scroll_help(self, d: int):
        if self.help_items_count <= 0:
            return
        m = max(0, self.help_items_count - self.help_vis)
        self._help_vt = max(0, min(m, self._help_vt + d))

    def _scroll_home(self, d: int):
        n = len(self.current_statuses())
        if n > 0:
            self.home_selected_entry = max(0, min(n - 1, self.home_selected_entry + d))
            self._home_vt = self._calc_scroll(
                self.home_selected_entry, self._home_vt, self.home_vis, n
            )

    def _scroll_flavors(self, d: int):
        name = self.current_name()
        if not name:
            return
        vrs = self.module_variants.get(name, [])
        n = len(vrs)
        if n > 0:
            self.flavors_cursor = max(0, min(n - 1, self.flavors_cursor + d))
            self._flavors_vt = self._calc_scroll(
                self.flavors_cursor, self._flavors_vt, self.flavors_vis, n
            )

    # ── Tree Data ──

    def _update_tree_data(self):
        self.tree_items = []
        self.selected_tree = 0
        self._tree_vt = 0
        name = self.current_name()
        if not name:
            return
        root = self.config.repo_root / name
        if not root.exists() or not root.is_dir():
            return

        def walk(path: Path, prefix: str = "", depth: int = 0):
            if depth > 10:
                return
            try:
                items = sorted(
                    [
                        p
                        for p in path.iterdir()
                        if not p.name.startswith(".") and p.name != "__pycache__"
                    ],
                    key=lambda p: (not p.is_dir(), p.name.lower()),
                )
            except Exception:
                return
            for i, item in enumerate(items):
                is_last = i == len(items) - 1
                char = "└── " if is_last else "├── "
                self.tree_items.append(
                    {
                        "path": item,
                        "name": item.name,
                        "is_dir": item.is_dir(),
                        "prefix": prefix + char,
                        "next_prefix": prefix + ("    " if is_last else "│   "),
                    }
                )
                if item.is_dir():
                    walk(item, prefix + ("    " if is_last else "│   "), depth + 1)

        walk(root)

    # ── Sorting ──

    def sort_modules(self, key: str):
        self.sort_key = key
        if key == "name":
            self.module_names.sort(reverse=self.sort_reverse)
        elif key == "status":
            self.module_names.sort(
                key=lambda n: self.module_link_state.get(n, ""),
                reverse=self.sort_reverse,
            )
        elif key == "destination":
            self.module_names.sort(
                key=lambda n: self.module_destinations.get(n, ""),
                reverse=self.sort_reverse,
            )
        elif key == "backup":
            self.module_names.sort(
                key=lambda n: self.module_last_backup.get(n, ""),
                reverse=self.sort_reverse,
            )
        self.selected_module = 0
        self._mod_vt = 0
        self._reset_module_sub_states()

    # ── Data Refresh ──

    def _jump_to_match(self):
        if not self.filter_text:
            return
        ft = self.filter_text.lower()
        for idx, name in enumerate(self.module_names):
            if ft in name.lower():
                self.selected_module = idx
                self._mod_vt = self._calc_scroll(
                    self.selected_module,
                    self._mod_vt,
                    self.mod_vis,
                    len(self.module_names),
                )
                self._reset_module_sub_states()
                break

    def refresh_modules(self):
        mods = resolve_modules(self.config)
        self.module_statuses = mods
        self.module_names = sorted(mods.keys())

        # Also include module dirs without statuses
        for d in self.config.get_module_dirs():
            if d.name not in self.module_names:
                self.module_names.append(d.name)
                self.module_statuses[d.name] = []
        self.module_names.sort()

        home = str(Path.home())
        sh = lambda p: "~" + str(p)[len(home) :] if str(p).startswith(home) else str(p)

        for name in self.module_names:
            sts = self.module_statuses.get(name, [])

            # Aggregate link state
            if not sts:
                self.module_link_state[name] = "unlinked"
            elif any(s.state in ("conflict", "unsafe") for s in sts):
                self.module_link_state[name] = "broken"
            elif all(s.state == "linked" for s in sts):
                self.module_link_state[name] = "linked"
            elif any(s.state == "linked" for s in sts):
                # Mix of linked and other states (partial)
                self.module_link_state[name] = "linked"
            else:
                self.module_link_state[name] = "unlinked"

            # Primary destination
            if sts:
                self.module_destinations[name] = sh(sts[0].destination)
            else:
                self.module_destinations[name] = ""

            # Last backup (git commit message)
            try:
                r = subprocess.run(
                    ["git", "log", "-1", "--format=%s", "--", name],
                    cwd=self.config.repo_root,
                    capture_output=True,
                    text=True,
                    timeout=2,
                )
                msg = r.stdout.strip() if r.returncode == 0 else ""
                self.module_last_backup[name] = msg if msg else ""
            except Exception:
                self.module_last_backup[name] = ""

            # Variants
            try:
                vinfo = get_module_variant_info(self.config, name)
                if vinfo and vinfo.has_variants:
                    self.module_has_variants[name] = True
                    self.module_variants[name] = vinfo.variants

                    # Deduce active variant
                    active_vars = set()
                    mod_dir = self.config.repo_root / name
                    for s in sts:
                        try:
                            rel = s.source.relative_to(mod_dir)
                            part = rel.parts[0] if rel.parts else ""
                            if part in vinfo.variants:
                                active_vars.add(part)
                        except ValueError:
                            pass
                    if active_vars:
                        self.module_active_variant[name] = list(active_vars)[0]
                    else:
                        self.module_active_variant[name] = vinfo.default_variant
                else:
                    self.module_has_variants[name] = False
                    self.module_variants[name] = []
                    self.module_active_variant[name] = ""
            except Exception:
                self.module_has_variants[name] = False
                self.module_variants[name] = []
                self.module_active_variant[name] = ""

        if self.module_names:
            self.selected_module = min(self.selected_module, len(self.module_names) - 1)

        # Re-apply sort
        if self.sort_key != "name":
            self.sort_modules(self.sort_key)

        self._update_tree_data()

    def refresh_backups(self):
        try:
            r = subprocess.run(
                ["git", "log", "--oneline", "-30"],
                cwd=self.config.repo_root,
                capture_output=True,
                text=True,
                timeout=5,
            )
            if r.returncode == 0:
                self.backup_entries = [
                    l.strip() for l in r.stdout.strip().splitlines() if l.strip()
                ]
            else:
                self.backup_entries = ["(no git history)"]
        except Exception:
            self.backup_entries = ["(git unavailable)"]


# ═══════════════════════════════════════════════════════════════════════
#  RENDERERS
# ═══════════════════════════════════════════════════════════════════════


def render_module_table(state: TUIState) -> FormattedText:
    """Render the module table with columns: Modules, Status, Destination, Last backup."""
    try:
        c = get_app().output.get_size().columns
        w = max(40, c - 4)
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        usable = max(2, p1_h - 3)
        state.mod_vis = usable
    except Exception:
        w = 120

    result: list[tuple[str, str]] = []
    names = state._filtered_names()

    # Column widths dynamically based on terminal width
    prefix_w = 4 if state.mode == "visual" else 2
    rem_w = w - prefix_w
    W_MOD = max(10, int(rem_w * 0.20))
    W_ST = max(10, int(rem_w * 0.12))
    W_DEST = max(15, int(rem_w * 0.45))
    W_BKP = max(10, rem_w - W_MOD - W_ST - W_DEST)

    # Header
    hdr = f"{'Modules':<{W_MOD}}{'Status':<{W_ST}}{'Destination':<{W_DEST}}{'Last backup':<{W_BKP}}"
    hdr = hdr.ljust(rem_w)[:rem_w]
    result.append(("class:col.header", f" {'':<{prefix_w - 1}}{hdr}\n"))

    if not names:
        result.append(("class:det.dim", "  (no modules)\n"))
        return FormattedText(result)

    visible = names[state._mod_vt : state._mod_vt + state.mod_vis]
    for idx, name in enumerate(visible):
        abs_i = state._mod_vt + idx
        is_sel = abs_i == state.selected_module

        # Row base style
        row_cls = "class:row.sel" if is_sel else "class:row"

        # Build prefix (cursor + optional selection circle)
        if state.mode == "visual":
            if abs_i in state.selected_set:
                circle = ("class:select.on", " ● ")
            else:
                circle = ("class:select.off", " ○ ")
        else:
            circle = None

        cursor = " ▸" if is_sel else "  "

        # Status color
        link_st = state.module_link_state.get(name, "unlinked")
        st_cls = f"class:status.{link_st}"

        # Destination (truncate if needed)
        dest = state.module_destinations.get(name, "")
        n_sts = len(state.module_statuses.get(name, []))
        if n_sts > 1:
            dest_display = dest[: W_DEST - 2]
        else:
            dest_display = dest[: W_DEST - 1]

        # Last backup (truncate)
        bkp = state.module_last_backup.get(name, "")
        bkp_display = bkp[: W_BKP - 1] if bkp else ""

        # Assemble the row
        if circle:
            result.append(circle)
        result.append((row_cls, cursor + " "))

        f_name = f"{name:<{W_MOD}}"
        f_link = f"{link_st:<{W_ST}}"
        f_dest = f"{dest_display:<{W_DEST}}"

        # Calculate padding needed at the end
        used = len(f_name) + len(f_link) + len(f_dest)
        f_bkp = f"{bkp_display:<{rem_w - used}}"

        result.append((row_cls, f_name))
        result.append((st_cls, f_link))
        if n_sts > 1:
            result.append((row_cls, f_dest))
        else:
            result.append(("class:det.dim" if not dest else row_cls, f_dest))
        result.append(("class:det.dim" if not is_sel else row_cls, f_bkp))
        result.append(("", "\n"))

    # Fill empty rows for consistent height
    rendered = len(visible)
    for _ in range(state.mod_vis - rendered):
        result.append(("", "\n"))

    return FormattedText(result)


def render_tab_bar(state: TUIState) -> FormattedText:
    """Render the tab bar for Panel 2."""
    result: list[tuple[str, str]] = []
    result.append(("", " "))
    tabs_str = " "

    for tab_id, label_active, label_inactive, key, available_in_visual in TABS:
        # Skip tabs not available in visual mode
        if state.mode == "visual" and not available_in_visual:
            continue

        is_active = state.active_tab == tab_id

        if is_active:
            text = f"[{label_active} ({key})]"
            tabs_str += text
            result.append(("class:tab.active", text))
        else:
            text = f" {label_inactive} ({key}) "
            tabs_str += text
            result.append(("class:tab.inactive", text))

    result.append(("", "\n"))
    # Divider line under tabs
    result.append(("class:border", " " + "─" * (len(tabs_str) - 1) + "\n"))
    return FormattedText(result)


def render_tab_content(state: TUIState) -> FormattedText:
    """Dispatch to the appropriate tab renderer."""
    tab = state.active_tab
    if tab == "home":
        return _render_home_tab(state)
    elif tab == "flavors":
        return _render_flavors_tab(state)
    elif tab == "logs":
        return _render_logs_tab(state)
    elif tab == "tree":
        return _render_tree_tab(state)
    elif tab == "backup":
        return _render_backup_tab(state)
    elif tab == "help":
        return _render_help_tab(state)
    return FormattedText([("class:det.dim", " (unknown tab)\n")])


def _render_home_tab(state: TUIState) -> FormattedText:
    """Home tab — general info about the selected module (like old Details panel)."""
    try:
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        p2_h = h - 5 - p1_h
        state.home_vis = max(2, p2_h - 6)
    except Exception:
        pass

    result: list[tuple[str, str]] = []
    name = state.current_name()
    if not name:
        result.append(("class:det.dim", " select a module\n"))
        return FormattedText(result)

    link_st = state.module_link_state.get(name, "unlinked")
    st_cls = f"class:status.{link_st}"

    result.append(("class:det.label", f" {name}  "))
    result.append((st_cls, f"[{link_st}]\n"))

    # Variants info
    if vrs := state.module_variants.get(name):
        result.append(("class:det.dim", " Variants: "))
        for i, v in enumerate(vrs):
            if i > 0:
                result.append(("", ", "))
            result.append(("class:det.pending", v))
        result.append(("", "\n"))

    # File mappings
    statuses = state.current_statuses()
    if not statuses:
        result.append(("class:det.dim", "\n (no mappings)\n"))
        return FormattedText(result)

    home = str(Path.home())
    sh = lambda p: "~" + str(p)[len(home) :] if str(p).startswith(home) else str(p)
    W1, W2, W3 = 14, 28, 8

    result.append(
        ("class:det.dim", f"\n   {'Source':<{W1}} {'Destination':<{W2}} State\n")
    )
    result.append(("class:border", "   " + "─" * (W1 + W2 + W3 + 2) + "\n"))

    st_cls_map = {
        "linked": "class:status.linked",
        "pending": "class:status.unlinked",
        "conflict": "class:status.broken",
        "unsafe": "class:status.broken",
        "missing": "class:det.missing",
    }

    act = state.active_panel == TUIState.PANEL_TABS and state.active_tab == "home"

    visible_statuses = statuses[state._home_vt : state._home_vt + state.home_vis]
    for idx, s in enumerate(visible_statuses):
        abs_i = state._home_vt + idx
        is_sel = abs_i == state.home_selected_entry and act
        is_checked = abs_i in state.home_selected_set

        cursor = "▸" if is_sel else " "
        check = "●" if is_checked else "○"

        result.append(("class:row.sel" if is_sel else "", f"{cursor} "))
        result.append(
            ("class:select.on" if is_checked else "class:select.off", f"{check} ")
        )

        row_cls = "class:row.sel" if is_sel else "class:det.dim"
        result.append((row_cls, f"{s.source.name[: W1 - 1]:<{W1}} "))
        result.append((row_cls, f"{sh(s.destination)[: W2 - 1]:<{W2}} "))
        result.append((st_cls_map.get(s.state, "class:det.dim"), f"{s.state:<{W3}}\n"))

    return FormattedText(result)


def _render_flavors_tab(state: TUIState) -> FormattedText:
    """Flavors tab — variant management."""
    try:
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        p2_h = h - 5 - p1_h
        state.flavors_vis = max(2, p2_h - 4)
    except Exception:
        pass

    result: list[tuple[str, str]] = []
    name = state.current_name()
    if not name:
        result.append(("class:det.dim", " select a module\n"))
        return FormattedText(result)

    if vrs := state.module_variants.get(name):
        act = (
            state.active_panel == TUIState.PANEL_TABS and state.active_tab == "flavors"
        )
        active_var = state.module_active_variant.get(name, "")

        result.append(("class:det.label", f" Flavors for {name}\n\n"))

        visible_vrs = vrs[state._flavors_vt : state._flavors_vt + state.flavors_vis]
        for idx, v in enumerate(visible_vrs):
            abs_i = state._flavors_vt + idx
            is_sel = abs_i == state.flavors_cursor and act
            is_active = v == active_var

            cursor = "▸" if is_sel else " "
            check = "●" if is_active else "○"

            result.append(("class:row.sel" if is_sel else "", f"{cursor} "))
            result.append(
                ("class:select.on" if is_active else "class:select.off", f"{check} ")
            )

            row_cls = "class:row.sel" if is_sel else "class:det.dim"
            result.append((row_cls, f"{v:<15} "))

            if is_active:
                result.append(("class:status.linked", "[active]\n"))
            else:
                result.append(("class:det.dim", "[available]\n"))
    else:
        result.append(("class:det.dim", f" {name} has no flavors\n"))
        result.append(
            ("class:det.dim", "\n Flavors let you have multiple configurations\n")
        )
        result.append(
            ("class:det.dim", " for the same software pointing to the same\n")
        )
        result.append(("class:det.dim", " destination.\n"))

    return FormattedText(result)


def _render_logs_tab(state: TUIState) -> FormattedText:
    """Logs tab — event log."""
    if not state.log_entries:
        return FormattedText([("class:log.ts", " —\n")])
    level_cls = {
        "info": "class:log.info",
        "success": "class:log.success",
        "warning": "class:log.warning",
        "error": "class:log.error",
    }
    result = []
    for ts, level, msg in state.log_entries[-12:]:
        result.extend(
            [
                ("class:log.ts", f" {ts}  "),
                (level_cls.get(level, "class:log.info"), f"{msg}\n"),
            ]
        )
    return FormattedText(result)


def _render_tree_tab(state: TUIState) -> FormattedText:
    """Tree tab — file tree of selected module."""
    try:
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        p2_h = h - 5 - p1_h
        state.tree_vis = max(2, p2_h - 4)  # 2 borders + 2 lines for tab options/divider
    except Exception:
        pass

    result: list[tuple[str, str]] = []
    if not state.tree_items:
        result.append(("class:det.dim", " (no tree)\n"))
        return FormattedText(result)

    visible = state.tree_items[state._tree_vt : state._tree_vt + state.tree_vis]
    act = state.active_panel == TUIState.PANEL_TABS and state.active_tab == "tree"
    for idx, item in enumerate(visible):
        abs_i = state._tree_vt + idx
        is_sel = abs_i == state.selected_tree and act
        cls = (
            "class:panel.title.active"
            if is_sel
            else ("class:det.label" if item["is_dir"] else "")
        )
        icon = "󰉋 " if item["is_dir"] else "󰈔 "
        result.append(("class:border", item["prefix"]))
        result.append(
            (cls, icon + item["name"] + ("/" if item["is_dir"] else "") + "\n")
        )
    return FormattedText(result)


def _render_backup_tab(state: TUIState) -> FormattedText:
    """Backup tab — git log history."""
    try:
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        p2_h = h - 5 - p1_h
        state.bkp_vis = max(2, p2_h - 4)
    except Exception:
        pass

    result: list[tuple[str, str]] = []
    if not state.backup_entries:
        result.append(("class:det.dim", " (no backups)\n"))
        return FormattedText(result)

    visible = state.backup_entries[state._bkp_vt : state._bkp_vt + state.bkp_vis]
    act = state.active_panel == TUIState.PANEL_TABS and state.active_tab == "backup"
    for idx, entry in enumerate(visible):
        abs_i = state._bkp_vt + idx
        is_sel = abs_i == state.selected_backup and act
        label = entry.split(" ", 1)[1] if " " in entry else entry
        result.append(
            (
                "class:bkp.sel" if is_sel else "class:bkp.item",
                f"{' ▸ ' if is_sel else '   '}{label}\n",
            )
        )
    return FormattedText(result)


def _render_help_tab(state: TUIState) -> FormattedText:
    """Help tab — keybinding reference."""
    try:
        h = get_app().output.get_size().rows
        p1_h = int((h - 5) * 0.55)
        p2_h = h - 5 - p1_h
        state.help_vis = max(2, p2_h - 4)
    except Exception:
        pass

    lines: list[list[tuple[str, str]]] = []
    lines.append([("class:det.label", " Keybinding Reference")])
    lines.append([])

    sections = [
        (
            "Navigation",
            [
                ("↑/k", "Move up"),
                ("↓/j", "Move down"),
                ("Tab", "Switch panel"),
            ],
        ),
        (
            "Actions",
            [
                ("Enter", "Link/unlink module"),
                ("b", "Backup dotfiles"),
                ("e", "Edit (sub-menu)"),
                ("r", "Refresh"),
            ],
        ),
        (
            "Modes",
            [
                ("space", "Enter visual/select mode"),
                ("a", "Toggle all (visual mode)"),
                ("/", "Filter modules"),
                ("o", "Order by column"),
            ],
        ),
        (
            "Tabs (uppercase)",
            [
                ("H", "Home / dots info"),
                ("F", "Flavors"),
                ("L", "Logs"),
                ("T", "Tree"),
                ("B", "Backup history"),
                ("?", "This help"),
            ],
        ),
        (
            "General",
            [
                ("Esc", "Cancel / exit mode"),
                ("q", "Quit"),
            ],
        ),
    ]

    for section_name, bindings in sections:
        lines.append([("class:det.dim", f" {section_name}")])
        for key, desc in bindings:
            lines.append(
                [("class:footer.key", f"   {key:<8}"), ("class:det.dim", f"{desc}")]
            )
        lines.append([])

    state.help_items_count = len(lines)
    visible_lines = lines[state._help_vt : state._help_vt + state.help_vis]

    result: list[tuple[str, str]] = []
    for line in visible_lines:
        result.extend(line)
        result.append(("", "\n"))

    return FormattedText(result)


def render_filter_line(state: TUIState) -> FormattedText:
    """Render inline filter prompt when active."""
    if not state.filter_active:
        return FormattedText([])
    result: list[tuple[str, str]] = [
        ("class:filter.prompt", " Buscar: "),
        ("class:filter.text", state.filter_text),
        ("class:filter.prompt", "█"),
    ]
    return FormattedText(result)


def render_footer(state: TUIState) -> FormattedText:
    """Context-sensitive footer keybindings."""
    result: list[tuple[str, str]] = []

    # Filter mode footer
    if state.filter_active:
        keys = [("type", "Buscar"), ("Enter", "apply"), ("Esc", "cancel")]
        for i, (k, d) in enumerate(keys):
            if i > 0:
                result.append(("class:footer.sep", "  "))
            result.extend([("class:footer.key", k), ("class:footer", f":{d}")])
        return FormattedText(result)

    # Sub-mode footers
    if state.sub_mode == "edit":
        keys = [("m", "edit module"), ("p", "edit path.yaml"), ("Esc", "cancel")]
        for i, (k, d) in enumerate(keys):
            if i > 0:
                result.append(("class:footer.sep", "  "))
            result.extend([("class:sub.key", k), ("class:sub.label", f":{d}")])
        return FormattedText(result)

    if state.sub_mode == "order":
        keys = [
            ("m", "by Modules"),
            ("s", "by Status"),
            ("d", "by Destination"),
            ("l", "by Last backup"),
            ("Esc", "cancel"),
        ]
        for i, (k, d) in enumerate(keys):
            if i > 0:
                result.append(("class:footer.sep", "  "))
            result.extend([("class:sub.key", k), ("class:sub.label", f":{d}")])
        return FormattedText(result)

    # Visual mode footer
    if state.mode == "visual":
        keys = [
            ("↑↓/jk", "nav"),
            ("b", "backup"),
            ("space", "select"),
            ("a", "toggle select"),
            ("Enter", "link/unlink/toggle"),
            ("Esc/q", "quit"),
        ]
    else:
        # Normal mode footer
        keys = [
            ("↑↓/jk", "nav"),
            ("Tab", "panel"),
            ("b", "backup"),
            ("e", "edit"),
            ("Enter", "link/unlink"),
            ("space", "select"),
            ("/", "Buscar"),
            ("o", "order"),
            ("Esc/q", "quit"),
        ]

    for i, (k, d) in enumerate(keys):
        if i > 0:
            result.append(("class:footer.sep", "  "))
        result.extend([("class:footer.key", k), ("class:footer", f":{d}")])

    return FormattedText(result)


# ═══════════════════════════════════════════════════════════════════════
#  APPLICATION BUILDER
# ═══════════════════════════════════════════════════════════════════════


def _build_app(state: TUIState) -> Application:
    kb = KeyBindings()

    # ── Navigation (j/k are also filter chars) ──
    @kb.add("j")
    def _dn_j(_):
        if state.filter_active:
            state.filter_text += "j"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        if state.active_panel == TUIState.PANEL_TABLE:
            state._scroll_mod(+1)
        elif state.active_panel == TUIState.PANEL_TABS:
            if state.active_tab == "tree":
                state._scroll_tree(+1)
            elif state.active_tab == "backup":
                state._scroll_bkp(+1)
            elif state.active_tab == "help":
                state._scroll_help(+1)
            elif state.active_tab == "home":
                state._scroll_home(+1)
            elif state.active_tab == "flavors":
                state._scroll_flavors(+1)

    @kb.add("down")
    def _dn_arrow(_):
        if state.filter_active or state.sub_mode:
            return
        if state.active_panel == TUIState.PANEL_TABLE:
            state._scroll_mod(+1)
        elif state.active_panel == TUIState.PANEL_TABS:
            if state.active_tab == "tree":
                state._scroll_tree(+1)
            elif state.active_tab == "backup":
                state._scroll_bkp(+1)
            elif state.active_tab == "help":
                state._scroll_help(+1)
            elif state.active_tab == "home":
                state._scroll_home(+1)
            elif state.active_tab == "flavors":
                state._scroll_flavors(+1)

    @kb.add("k")
    def _up_k(_):
        if state.filter_active:
            state.filter_text += "k"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        if state.active_panel == TUIState.PANEL_TABLE:
            state._scroll_mod(-1)
        elif state.active_panel == TUIState.PANEL_TABS:
            if state.active_tab == "tree":
                state._scroll_tree(-1)
            elif state.active_tab == "backup":
                state._scroll_bkp(-1)
            elif state.active_tab == "help":
                state._scroll_help(-1)
            elif state.active_tab == "home":
                state._scroll_home(-1)
            elif state.active_tab == "flavors":
                state._scroll_flavors(-1)

    @kb.add("up")
    def _up_arrow(_):
        if state.filter_active or state.sub_mode:
            return
        if state.active_panel == TUIState.PANEL_TABLE:
            state._scroll_mod(-1)
        elif state.active_panel == TUIState.PANEL_TABS:
            if state.active_tab == "tree":
                state._scroll_tree(-1)
            elif state.active_tab == "backup":
                state._scroll_bkp(-1)
            elif state.active_tab == "help":
                state._scroll_help(-1)
            elif state.active_tab == "home":
                state._scroll_home(-1)
            elif state.active_tab == "flavors":
                state._scroll_flavors(-1)

    # ── Tab (panel switch) ──
    @kb.add("tab")
    def _tab(_):
        if state.filter_active or state.sub_mode:
            return
        state.active_panel = 1 - state.active_panel

    # ── Tab keys (uppercase — not affected by filter since filter uses lowercase) ──
    @kb.add("H")
    def _tab_home(_):
        if state.filter_active:
            state.filter_text += "H"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        state.active_tab = "home"

    @kb.add("F")
    def _tab_flavors(_):
        if state.filter_active or state.sub_mode:
            return
        if state.mode == "visual":
            return
        state.active_tab = "flavors"

    @kb.add("L")
    def _tab_logs(_):
        if state.filter_active or state.sub_mode:
            return
        state.active_tab = "logs"

    @kb.add("T")
    def _tab_tree(_):
        if state.filter_active or state.sub_mode:
            return
        if state.mode == "visual":
            return
        state.active_tab = "tree"

    @kb.add("B")
    def _tab_backup(_):
        if state.filter_active or state.sub_mode:
            return
        if state.mode == "visual":
            return
        state.active_tab = "backup"

    @kb.add("?")
    def _tab_help(_):
        if state.filter_active:
            state.filter_text += "?"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        state.active_tab = "help"

    # ── Space (visual mode / select) ──
    @kb.add("space")
    def _space(_):
        if state.filter_active:
            state.filter_text += " "
            state._jump_to_match()
            return
        if state.sub_mode:
            return

        if state.active_panel == TUIState.PANEL_TABS and state.active_tab == "home":
            sts = state.current_statuses()
            if sts and 0 <= state.home_selected_entry < len(sts):
                idx = state.home_selected_entry
                if idx in state.home_selected_set:
                    state.home_selected_set.remove(idx)
                else:
                    state.home_selected_set.add(idx)
            return

        if state.mode == "normal":
            state.mode = "visual"
            state.selected_set = set()
            if state.active_tab not in ("home", "logs", "help"):
                state.active_tab = "home"
            state.log("info", "Visual/Select mode activated")
        else:
            if state.selected_module in state.selected_set:
                state.selected_set.discard(state.selected_module)
            else:
                state.selected_set.add(state.selected_module)

    # ── a (toggle all / filter char) ──
    @kb.add("a")
    def _toggle_all(_):
        if state.filter_active:
            state.filter_text += "a"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        if state.mode != "visual":
            return
        names = state._filtered_names()
        if len(state.selected_set) == len(names):
            state.selected_set.clear()
        else:
            state.selected_set = set(range(len(names)))

    # ── Enter (link/unlink / apply filter) ──
    @kb.add("enter")
    def _enter(event):
        if state.filter_active:
            state.filter_active = False
            state.selected_module = 0
            state._mod_vt = 0
            state.log(
                "info",
                f"Filter: '{state.filter_text}'"
                if state.filter_text
                else "Filter cleared",
            )
            return
        if state.sub_mode:
            return

        if state.active_panel == TUIState.PANEL_TABS:
            if state.active_tab == "home":
                _action_link_home_items(event, state)
            elif state.active_tab == "flavors":
                _action_switch_variant(event, state)
            return

        if state.mode == "normal":
            name = state.current_name()
            if not name:
                return
            link_st = state.module_link_state.get(name, "unlinked")
            if link_st == "linked":
                _action_unlink(event, state, [name])
            else:
                _action_link(event, state, [name])
        else:
            names = state._filtered_names()
            selected_names = [
                names[i] for i in sorted(state.selected_set) if i < len(names)
            ]
            if not selected_names:
                return
            states = [
                state.module_link_state.get(n, "unlinked") for n in selected_names
            ]
            to_link = [n for n, s in zip(selected_names, states) if s != "linked"]
            to_unlink = [n for n, s in zip(selected_names, states) if s == "linked"]
            if to_link:
                _action_link(event, state, to_link)
            if to_unlink:
                _action_unlink(event, state, to_unlink)

    # ── e (edit sub-mode / filter char) ──
    @kb.add("e")
    def _edit(_):
        if state.filter_active:
            state.filter_text += "e"
            state._jump_to_match()
            return
        if state.mode == "visual":
            return
        if state.sub_mode:
            return
        state.sub_mode = "edit"

    # ── m (edit module / order by Modules / filter char) ──
    @kb.add("m")
    def _key_m(event):
        if state.filter_active:
            state.filter_text += "m"
            state._jump_to_match()
            return
        if state.sub_mode == "edit":
            p = state.current_module_dir()
            if not p:
                state.sub_mode = None
                return
            state.sub_mode = None
            editor = os.environ.get("EDITOR", "vim")
            event.app.suspend_to_background()
            subprocess.run([editor, str(p)])
            state.log("info", f"Edited {p.name}")
            state.refresh_modules()
        elif state.sub_mode == "order":
            state.sub_mode = None
            state.sort_modules("name")
            state.log("info", "Sorted by Modules")

    # ── p (edit path.yaml / filter char) ──
    @kb.add("p")
    def _key_p(event):
        if state.filter_active:
            state.filter_text += "p"
            state._jump_to_match()
            return
        if state.sub_mode != "edit":
            return
        p = state.current_module_dir()
        if not p:
            state.sub_mode = None
            return
        yaml_path = p / "path.yaml"
        state.sub_mode = None
        if yaml_path.exists():
            editor = os.environ.get("EDITOR", "vim")
            event.app.suspend_to_background()
            subprocess.run([editor, str(yaml_path)])
            state.log("info", f"Edited {yaml_path.name}")
            state.refresh_modules()
        else:
            state.log("warning", f"No path.yaml in {p.name}")

    # ── o (order sub-mode / filter char) ──
    @kb.add("o")
    def _order(_):
        if state.filter_active:
            state.filter_text += "o"
            state._jump_to_match()
            return
        if state.mode == "visual":
            return
        if state.sub_mode:
            return
        state.sub_mode = "order"

    # ── s (order by Status / filter char) ──
    @kb.add("s")
    def _key_s(_):
        if state.filter_active:
            state.filter_text += "s"
            state._jump_to_match()
            return
        if state.sub_mode != "order":
            return
        state.sub_mode = None
        state.sort_modules("status")
        state.log("info", "Sorted by Status")

    # ── d (order by Destination / filter char) ──
    @kb.add("d")
    def _key_d(_):
        if state.filter_active:
            state.filter_text += "d"
            state._jump_to_match()
            return
        if state.sub_mode != "order":
            return
        state.sub_mode = None
        state.sort_modules("destination")
        state.log("info", "Sorted by Destination")

    # ── l (order by Last backup / filter char) ──
    @kb.add("l")
    def _key_l(_):
        if state.filter_active:
            state.filter_text += "l"
            state._jump_to_match()
            return
        if state.sub_mode != "order":
            return
        state.sub_mode = None
        state.sort_modules("backup")
        state.log("info", "Sorted by Last backup")

    # ── / (filter) ──
    @kb.add("/")
    def _filter(_):
        if state.filter_active:
            state.filter_text += "/"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        if state.mode == "visual":
            return
        state.filter_active = True
        state.filter_text = ""

    # ── Backspace (filter delete char) ──
    @kb.add("backspace")
    def _filter_bs(_):
        if state.filter_active:
            state.filter_text = state.filter_text[:-1]
            state._jump_to_match()

    # ── b (backup / filter char) ──
    @kb.add("b")
    def _backup(event):
        if state.filter_active:
            state.filter_text += "b"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        _action_backup(event, state)

    # ── r (refresh / filter char) ──
    @kb.add("r")
    def _refresh(_):
        if state.filter_active:
            state.filter_text += "r"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        state.refresh_modules()
        state.refresh_backups()
        state.log("info", "Refreshed")

    # ── Escape ──
    @kb.add("escape")
    def _esc(_):
        if state.filter_active:
            state.filter_active = False
            state.filter_text = ""
            return
        if state.sub_mode:
            state.sub_mode = None
            return
        if state.mode == "visual":
            state.mode = "normal"
            state.selected_set.clear()
            state.log("info", "Normal mode")
            return
        get_app().exit()

    # ── q (quit / filter char) ──
    @kb.add("q")
    def _quit(_):
        if state.filter_active:
            state.filter_text += "q"
            state._jump_to_match()
            return
        if state.sub_mode:
            return
        get_app().exit()

    @kb.add("c-c")
    def _quit_cc(_):
        get_app().exit()

    # ── Remaining lowercase letters + digits for filter input ──
    # Letters not already handled above: c, f, g, h, i, n, t, u, v, w, x, y, z
    _filter_only_chars = "cfghintuvwxyz0123456789-_."
    for ch in _filter_only_chars:

        @kb.add(ch)
        def _filter_char(_, c=ch):
            if state.filter_active:
                state.filter_text += c
            state._jump_to_match()

    from prompt_toolkit.filters import Condition

    @Condition
    def is_searching():
        return state.filter_active

    @kb.add(Keys.Any, filter=is_searching)
    def _search_catch_all(event):
        char = event.key_sequence[0].data
        if len(char) == 1 and char.isprintable():
            state.filter_text += char
            state._jump_to_match()

    # ── Build Layout ──
    win_table = Window(
        content=FormattedTextControl(lambda: render_module_table(state)),
        always_hide_cursor=True,
    )
    win_filter = Window(
        content=FormattedTextControl(lambda: render_filter_line(state)),
        height=1,
        always_hide_cursor=True,
    )
    win_tab_bar = Window(
        content=FormattedTextControl(lambda: render_tab_bar(state)),
        height=2,
        always_hide_cursor=True,
    )
    win_tab_content = Window(
        content=FormattedTextControl(lambda: render_tab_content(state)),
        always_hide_cursor=True,
    )

    frame_table = rounded_frame(
        HSplit([win_table, win_filter]),
        active_fn=lambda: state.active_panel == TUIState.PANEL_TABLE,
    )
    frame_tabs = rounded_frame(
        HSplit([win_tab_bar, win_tab_content]),
        active_fn=lambda: state.active_panel == TUIState.PANEL_TABS,
    )

    win_footer = Window(
        content=FormattedTextControl(lambda: render_footer(state)),
        height=1,
        always_hide_cursor=True,
    )

    root = HSplit(
        [
            gap_h(1),
            HSplit([frame_table], height=D(weight=55)),
            gap_h(1),
            HSplit([frame_tabs], height=D(weight=45)),
            gap_h(1),
            win_footer,
            gap_h(1),
        ]
    )

    return Application(
        layout=Layout(root),
        key_bindings=kb,
        style=TUI_STYLE,
        full_screen=True,
    )


# ═══════════════════════════════════════════════════════════════════════
#  ACTIONS (link, unlink, backup)
# ═══════════════════════════════════════════════════════════════════════

import io
import re
import sys


def _capture_cmd(fn, *args, **kwargs) -> tuple[bool, list[str]]:
    """
    Execute a CLI command function capturing its stdout/stderr output.
    Returns (success: bool, lines: list[str]).
    Prevents CLI Rich output from corrupting the TUI layout.
    """
    buf = io.StringIO()
    old_stdout, old_stderr = sys.stdout, sys.stderr
    sys.stdout = sys.stderr = buf
    success = True
    try:
        fn(*args, **kwargs)
    except SystemExit:
        pass
    except Exception:
        success = False
    finally:
        sys.stdout, sys.stderr = old_stdout, old_stderr

    raw = buf.getvalue()
    ansi_escape = re.compile(r"\x1b\[[0-9;]*m")
    lines = [
        ansi_escape.sub("", line).strip()
        for line in raw.splitlines()
        if ansi_escape.sub("", line).strip()
    ]
    return success, lines


def _action_link(event, state: TUIState, names: list[str]):
    """Link modules."""
    from dots.commands.link import link_cmd

    success, lines = _capture_cmd(
        link_cmd, module=names, dry_run=False, force=False, interactive=False
    )
    for line in lines:
        level = (
            "error"
            if any(w in line.lower() for w in ("error", "fail", "✖"))
            else "info"
        )
        state.log(level, line)
    if success:
        state.log("success", f"Linked: {', '.join(names)}")
    else:
        state.log("error", f"Link failed for: {', '.join(names)}")
    state.refresh_modules()


def _action_unlink(event, state: TUIState, names: list[str]):
    """Unlink modules."""
    from dots.commands.unlink import unlink_cmd

    success, lines = _capture_cmd(
        unlink_cmd, module=names, dry_run=False, interactive=False
    )
    for line in lines:
        level = (
            "error"
            if any(w in line.lower() for w in ("error", "fail", "✖"))
            else "info"
        )
        state.log(level, line)
    if success:
        state.log("success", f"Unlinked: {', '.join(names)}")
    else:
        state.log("error", f"Unlink failed for: {', '.join(names)}")
    state.refresh_modules()


def _action_backup(event, state: TUIState):
    """Backup dotfiles."""
    from dots.commands.backup import run_backup, default_commit_message

    msg = default_commit_message()
    success, lines = _capture_cmd(run_backup, msg, state.config.repo_root)
    for line in lines:
        state.log("info", line)
    if success:
        state.log("success", f"Backup: {msg}")
    else:
        state.log("warning", "Backup: nothing to commit or failed")
    state.refresh_backups()


def _action_link_home_items(event, state: TUIState):
    """Link/unlink specific items selected in Home tab."""
    name = state.current_name()
    sts = state.current_statuses()
    if not name or not sts:
        return

    indices = (
        state.home_selected_set
        if state.home_selected_set
        else {state.home_selected_entry}
    )
    selected_sts = [sts[i] for i in indices if 0 <= i < len(sts)]

    if not selected_sts:
        return

    transaction = TransactionLog()
    linked = unlinked = 0

    try:
        for status in selected_sts:
            src = status.source
            dest = status.destination
            if status.state == "linked":
                if dest.is_symlink():
                    transaction.unlink(dest)
                    unlinked += 1
            else:
                if dest.is_symlink():
                    transaction.unlink(dest)
                elif dest.exists():
                    backup_path = dest.with_name(dest.name + "-backup")
                    transaction.backup(dest, backup_path)
                if not dest.parent.exists():
                    transaction.mkdir(dest.parent)
                transaction.symlink(dest, src.resolve())
                linked += 1

        transaction.commit()
        if linked > 0 or unlinked > 0:
            msg = []
            if linked:
                msg.append(f"{linked} linked")
            if unlinked:
                msg.append(f"{unlinked} unlinked")
            state.log("success", f"Home items: {', '.join(msg)}")
    except Exception as exc:
        transaction.rollback()
        state.log("error", f"Item action failed: {exc}")

    state.home_selected_set.clear()
    state.refresh_modules()


def _action_switch_variant(event, state: TUIState):
    """Switch the active variant for the module."""
    name = state.current_name()
    if not name:
        return

    vrs = state.module_variants.get(name, [])
    if not vrs or not (0 <= state.flavors_cursor < len(vrs)):
        return

    target_variant = vrs[state.flavors_cursor]
    active_variant = state.module_active_variant.get(name, "")

    if target_variant == active_variant:
        state.log("info", f"Variant {target_variant} is already active")
        return

    from dots.commands.link import link_cmd

    success, lines = _capture_cmd(
        link_cmd,
        module=[name],
        dry_run=False,
        force=True,
        interactive=False,
        variant=target_variant,
    )
    for line in lines:
        state.log("info", line)
    if success:
        state.log("success", f"Switched {name} → {target_variant}")
    else:
        state.log("error", f"Variant switch failed: {name} → {target_variant}")

    state.refresh_modules()


# ═══════════════════════════════════════════════════════════════════════
#  ENTRY POINT
# ═══════════════════════════════════════════════════════════════════════


def dashboard():
    config = DotsConfig.load()
    state = TUIState(config)
    _build_app(state).run()

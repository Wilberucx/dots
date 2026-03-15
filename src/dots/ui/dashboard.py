"""
Lazygit-style TUI dashboard for dots CLI.

4-panel layout with rounded-corner frames, proportional splits,
viewporting scroll, and a dimmed adaptive footer.
"""
from __future__ import annotations

import os
import subprocess
from datetime import datetime
from pathlib import Path
from typing import Optional, Callable

from prompt_toolkit import Application
from prompt_toolkit.application.current import get_app
from prompt_toolkit.key_binding import KeyBindings
from prompt_toolkit.layout import (
    Layout, HSplit, VSplit, Window, FormattedTextControl, D,
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
#  ROUNDED FRAME BUILDER
# ═══════════════════════════════════════════════════════════════════════

def rounded_frame(body: Window | HSplit | VSplit,
                  title: str = "",
                  active_fn: Callable[[], bool] = lambda: False,
                  scroll_hints: Callable[[], tuple[int, int]] = lambda: (0, 0)) -> HSplit:
    """Wrap body in a box with rounded corners and an optional title."""
    def border_style() -> str:
        return "class:border.active" if active_fn() else "class:border"

    def title_style() -> str:
        return "class:panel.title.active" if active_fn() else "class:panel.title"

    def get_top_border():
        top_n, below_n = scroll_hints()
        res = [(border_style(), "╭─")]
        if title:
            res.append((title_style(), f" {title} "))
            res.append((border_style(), "─"))
        if top_n > 0:
            res.append((title_style(), f" ↑ {top_n} "))
            res.append((border_style(), "─"))
        return res

    def get_bot_border():
        top_n, below_n = scroll_hints()
        res = [(border_style(), "╰")]
        if below_n > 0:
            res.append((border_style(), "──"))
            res.append((title_style(), f" ↓ {below_n} "))
        return res

    top_row = VSplit([
        Window(content=FormattedTextControl(get_top_border), height=1),
        Window(content=FormattedTextControl(lambda: [(border_style(), "─" * 200)]), height=1),
        Window(content=FormattedTextControl(lambda: [(border_style(), "╮")]), width=1, height=1),
    ])

    bot_row = VSplit([
        Window(content=FormattedTextControl(get_bot_border), height=1),
        Window(content=FormattedTextControl(lambda: [(border_style(), "─" * 200)]), height=1),
        Window(content=FormattedTextControl(lambda: [(border_style(), "╯")]), width=1, height=1),
    ])

    left_bar = Window(
        content=FormattedTextControl(lambda: [(border_style(), "│\n" * 200)]),
        width=1, always_hide_cursor=True, wrap_lines=False,
    )
    right_bar = Window(
        content=FormattedTextControl(lambda: [(border_style(), "│\n" * 200)]),
        width=1, always_hide_cursor=True, wrap_lines=False,
    )
    mid_row = VSplit([left_bar, body, right_bar])

    return HSplit([top_row, mid_row, bot_row])


def gap_h(rows: int = 1) -> Window:
    return Window(height=D(min=rows, max=rows, preferred=rows), always_hide_cursor=True)

def gap_v(cols: int = 1) -> Window:
    return Window(width=D(min=cols, max=cols, preferred=cols), always_hide_cursor=True)


# ═══════════════════════════════════════════════════════════════════════
#  STATE
# ═══════════════════════════════════════════════════════════════════════

class TUIState:
    PANEL_MODULES = 0
    PANEL_BACKUPS = 1
    PANEL_DETAILS = 2
    PANEL_TREE    = 3
    PANEL_LOG     = 4

    def __init__(self, config: DotsConfig):
        self.config = config
        self.active_panel: int = self.PANEL_MODULES

        # Modules
        self.module_names:    list[str]               = []
        self.module_statuses: dict[str, list[LinkStatus]] = {}
        self.module_health:   dict[str, str]           = {}
        self.module_has_changes: dict[str, bool]       = {}
        self.module_has_variants: dict[str, bool]      = {}
        self.module_variants:     dict[str, list[str]] = {}
        self.module_file_count:   dict[str, int]       = {}
        self.selected_module: int = 0
        self._mod_vt: int = 0
        self.mod_vis: int = 10

        # Tree
        self.tree_items:      list[dict] = []
        self.selected_tree:   int = 0
        self._tree_vt: int = 0
        self.tree_vis: int = 10

        # Backups
        self.backup_entries:  list[str] = []
        self.selected_backup: int = 0
        self._bkp_vt: int = 0
        self.bkp_vis: int = 6

        # Log
        self.log_entries: list[tuple[str, str, str]] = []

        self.refresh_modules()
        self.refresh_backups()
        self.log("info", "Dots TUI started")

    def current_name(self) -> Optional[str]:
        return self.module_names[self.selected_module] if self.module_names else None

    def current_statuses(self) -> list[LinkStatus]:
        n = self.current_name()
        return self.module_statuses.get(n, []) if n else []

    def current_module_dir(self) -> Optional[Path]:
        n = self.current_name()
        return self.config.repo_root / n if n else None

    def current_tree_path(self) -> Optional[Path]:
        if self.active_panel == self.PANEL_TREE and self.tree_items:
            return self.tree_items[self.selected_tree]["path"]
        return self.current_module_dir()

    def log(self, level: str, msg: str):
        ts = datetime.now().strftime("%H:%M:%S")
        self.log_entries.append((ts, level, msg))
        if len(self.log_entries) > 200:
            self.log_entries = self.log_entries[-200:]

    def _scroll_mod(self, d: int):
        if not self.module_names: return
        n = len(self.module_names)
        self.selected_module = max(0, min(n-1, self.selected_module + d))
        s, vt, v = self.selected_module, self._mod_vt, self.mod_vis
        if s < vt: self._mod_vt = s
        elif s >= vt + v: self._mod_vt = s - v + 1
        self._update_tree_data()

    def _scroll_tree(self, d: int):
        if not self.tree_items: return
        n = len(self.tree_items)
        self.selected_tree = max(0, min(n-1, self.selected_tree + d))
        s, vt, v = self.selected_tree, self._tree_vt, self.tree_vis
        if s < vt: self._tree_vt = s
        elif s >= vt + v: self._tree_vt = s - v + 1

    def _scroll_bkp(self, d: int):
        if not self.backup_entries: return
        n = len(self.backup_entries)
        self.selected_backup = max(0, min(n-1, self.selected_backup + d))
        s, vt, v = self.selected_backup, self._bkp_vt, self.bkp_vis
        if s < vt: self._bkp_vt = s
        elif s >= vt + v: self._bkp_vt = s - v + 1

    def _update_tree_data(self):
        self.tree_items = []
        self.selected_tree = 0
        self._tree_vt = 0
        name = self.current_name()
        if not name: return
        root = self.config.repo_root / name
        if not root.exists() or not root.is_dir(): return

        def walk(path: Path, prefix: str = "", depth: int = 0):
            if depth > 10: return
            try:
                items = sorted(
                    [p for p in path.iterdir() if not p.name.startswith(".") and p.name != "__pycache__"],
                    key=lambda p: (not p.is_dir(), p.name.lower())
                )
            except Exception: return
            for i, item in enumerate(items):
                is_last = i == len(items) - 1
                char = "└── " if is_last else "├── "
                self.tree_items.append({
                    "path": item, "name": item.name, "is_dir": item.is_dir(),
                    "prefix": prefix + char, "next_prefix": prefix + ("    " if is_last else "│   ")
                })
                if item.is_dir():
                    walk(item, prefix + ("    " if is_last else "│   "), depth + 1)
        walk(root)

    def refresh_modules(self):
        mods = resolve_modules(self.config)
        self.module_statuses = mods
        self.module_names = sorted(mods.keys())
        for d in self.config.get_module_dirs():
            if d.name not in self.module_names:
                self.module_names.append(d.name)
                self.module_statuses[d.name] = []
        self.module_names.sort()

        self.module_health, self.module_has_changes = {}, {}
        self.module_has_variants, self.module_variants, self.module_file_count = {}, {}, {}

        for name in self.module_names:
            sts = self.module_statuses.get(name, [])
            self.module_file_count[name] = len(sts)
            if not sts: self.module_health[name] = "healthy"
            elif any(s.state in ("conflict","unsafe") for s in sts): self.module_health[name] = "error"
            elif any(s.state == "pending" for s in sts): self.module_health[name] = "warning"
            else: self.module_health[name] = "healthy"

            try:
                r = subprocess.run(["git", "status", "--porcelain", name], cwd=self.config.repo_root, capture_output=True, text=True, timeout=2)
                self.module_has_changes[name] = bool(r.stdout.strip())
            except Exception: self.module_has_changes[name] = False

            try:
                vinfo = get_module_variant_info(self.config, name)
                if vinfo and vinfo.has_variants:
                    self.module_has_variants[name] = True
                    self.module_variants[name] = vinfo.variants
                else:
                    self.module_has_variants[name], self.module_variants[name] = False, []
            except Exception: self.module_has_variants[name] = False

        if self.module_names:
            self.selected_module = min(self.selected_module, len(self.module_names)-1)
        self._update_tree_data()

    def refresh_backups(self):
        try:
            r = subprocess.run(["git","log","--oneline","-30"], cwd=self.config.repo_root, capture_output=True, text=True, timeout=5)
            self.backup_entries = [l.strip() for l in r.stdout.strip().splitlines() if l.strip()] if r.returncode == 0 else ["(no git history)"]
        except Exception: self.backup_entries = ["(git unavailable)"]


# ═══════════════════════════════════════════════════════════════════════
#  RENDERERS
# ═══════════════════════════════════════════════════════════════════════

def render_modules(state: TUIState) -> FormattedText:
    try:
        h = get_app().output.get_size().rows
        usable = max(6, h - 8)
        # MODULES gets 70% of left column usable height
        state.mod_vis = max(2, int(usable * 0.7) - 2)
    except Exception: pass

    result: list[tuple[str,str]] = []
    if not state.module_names: return FormattedText([("class:det.dim", " (no modules)\n")])

    visible = state.module_names[state._mod_vt : state._mod_vt + state.mod_vis]
    for idx, name in enumerate(visible):
        abs_i = state._mod_vt + idx
        is_sel = abs_i == state.selected_module
        cls = f"class:mod.sel.{state.module_health.get(name,'healthy')}" if is_sel else f"class:mod.{state.module_health.get(name,'healthy')}"
        mark = " ▸ " if is_sel else "   "
        
        inds = []
        if (c := state.module_file_count.get(name, 0)) > 0: inds.append(f"󰈔 {c}")
        if state.module_has_variants.get(name): inds.append("󱔖")
        if state.module_has_changes.get(name): inds.append("󱗜")
        
        line = f"{mark}{name:<18}  {'  '.join(inds)}\n"
        result.append((cls, line))
    return FormattedText(result)


def render_details(state: TUIState) -> FormattedText:
    result: list[tuple[str,str]] = []
    name = state.current_name()
    if not name: return FormattedText([("class:det.dim", " select a module\n")])

    health = state.module_health.get(name, "healthy")
    hlth_cls = {"healthy":"class:det.linked","warning":"class:det.pending","error":"class:det.conflict"}.get(health,"class:det.dim")
    result.append(("class:det.label", f" {name}  "))
    result.append((hlth_cls, f"[{health}]\n"))

    if vars := state.module_variants.get(name):
        result.append(("class:det.dim", " Variants: "))
        for i, v in enumerate(vars):
            if i > 0: result.append(("", ", "))
            result.append(("class:det.pending", v))
        result.append(("", "\n"))

    statuses = state.current_statuses()
    if not statuses: return FormattedText(result + [("class:det.dim", "\n (no mappings)\n")])

    home = str(Path.home())
    sh = lambda p: "~" + str(p)[len(home):] if str(p).startswith(home) else str(p)
    W1, W2, W3 = 14, 28, 8
    result.append(("class:det.dim", f"\n {'Source':<{W1}} {'Destination':<{W2}} State\n"))
    result.append(("class:border",  " " + "─"*(W1+W2+W3+2) + "\n"))

    st_cls = {"linked":"class:det.linked","pending":"class:det.pending","conflict":"class:det.conflict","unsafe":"class:det.unsafe","missing":"class:det.missing"}
    for s in statuses:
        result.append(("class:det.dim",  f" {s.source.name[:W1-1]:<{W1}} "))
        result.append(("class:det.dim",  f"{sh(s.destination)[:W2-1]:<{W2}} "))
        result.append((st_cls.get(s.state,"class:det.dim"), f"{s.state:<{W3}}\n"))
    return FormattedText(result)


def render_tree(state: TUIState) -> FormattedText:
    try:
        h = get_app().output.get_size().rows
        usable = max(6, h - 8)
        # TREE gets 35% of right column usable height
        state.tree_vis = max(2, int(usable * 0.35) - 2)
    except Exception: pass

    result: list[tuple[str,str]] = []
    if not state.tree_items: return FormattedText([("class:det.dim", " (no tree)\n")])

    visible = state.tree_items[state._tree_vt : state._tree_vt + state.tree_vis]
    act = state.active_panel == TUIState.PANEL_TREE
    for idx, item in enumerate(visible):
        abs_i = state._tree_vt + idx
        is_sel = abs_i == state.selected_tree and act
        cls = "class:panel.title.active" if is_sel else ("class:det.label" if item["is_dir"] else "")
        icon = ("󰉋 " if item["is_dir"] else "󰈔 ")
        result.append(("class:border", item["prefix"]))
        result.append((cls, icon + item["name"] + ("/" if item["is_dir"] else "") + "\n"))
    return FormattedText(result)


def render_backups(state: TUIState) -> FormattedText:
    try:
        h = get_app().output.get_size().rows
        usable = max(6, h - 8)
        # BACKUPS gets 30% of left column usable height
        state.bkp_vis = max(2, int(usable * 0.3) - 2)
    except Exception: pass

    result: list[tuple[str,str]] = []
    if not state.backup_entries: return FormattedText([("class:det.dim", " (no backups)\n")])
    visible = state.backup_entries[state._bkp_vt : state._bkp_vt + state.bkp_vis]
    act = state.active_panel == TUIState.PANEL_BACKUPS
    for idx, entry in enumerate(visible):
        abs_i = state._bkp_vt + idx
        is_sel = abs_i == state.selected_backup and act
        label = entry.split(" ", 1)[1] if " " in entry else entry
        result.append(("class:bkp.sel" if is_sel else "class:bkp.item", f"{' ▸ ' if is_sel else '   '}{label}\n"))
    return FormattedText(result)


def render_log(state: TUIState) -> FormattedText:
    if not state.log_entries: return FormattedText([("class:log.ts", " —\n")])
    level_cls = {"info":"class:log.info","success":"class:log.success","warning":"class:log.warning","error":"class:log.error"}
    result = []
    for ts, level, msg in state.log_entries[-8:]:
        result.extend([("class:log.ts", f" {ts}  "), (level_cls.get(level,"class:log.info"), f"{msg}\n")])
    return FormattedText(result)


def render_footer(state: TUIState) -> FormattedText:
    KEYS = [("↑↓/jk","nav"),("Tab","panel"),("l","link"),("b","backup"),("r","refresh"),("e","edit"),("q","quit")]
    LEGEND = [("󰈔","Files"),("󱔖","Vars"),("󱗜","Dirty")]
    result = []
    for i, (k, d) in enumerate(KEYS):
        if i > 0: result.append(("class:footer.sep", "  "))
        result.extend([("class:footer.key", k), ("class:footer", f":{d}")])
    result.append(("", " " * 4))
    for i, (icon, d) in enumerate(LEGEND):
        if i > 0: result.append(("class:footer.sep", " "))
        result.extend([("class:footer.key", icon), ("class:footer", f":{d}")])
    return FormattedText(result)


# ═══════════════════════════════════════════════════════════════════════
#  APPLICATION BUILDER
# ═══════════════════════════════════════════════════════════════════════

def _build_app(state: TUIState) -> Application:
    kb = KeyBindings()

    # ── Numeric Panel Switching (1-5) ──
    for _num, _panel in [(1, TUIState.PANEL_MODULES), (2, TUIState.PANEL_BACKUPS),
                         (3, TUIState.PANEL_DETAILS), (4, TUIState.PANEL_TREE),
                         (5, TUIState.PANEL_LOG)]:
        @kb.add(str(_num))
        def _sw(event, p=_panel): state.active_panel = p

    @kb.add("j")
    @kb.add("down")
    def _dn(_):
        if state.active_panel == TUIState.PANEL_MODULES: state._scroll_mod(+1)
        elif state.active_panel == TUIState.PANEL_TREE: state._scroll_tree(+1)
        elif state.active_panel == TUIState.PANEL_BACKUPS: state._scroll_bkp(+1)

    @kb.add("k")
    @kb.add("up")
    def _up(_):
        if state.active_panel == TUIState.PANEL_MODULES: state._scroll_mod(-1)
        elif state.active_panel == TUIState.PANEL_TREE: state._scroll_tree(-1)
        elif state.active_panel == TUIState.PANEL_BACKUPS: state._scroll_bkp(-1)

    @kb.add("tab")
    def _cyc(_): state.active_panel = (state.active_panel + 1) % 5
    @kb.add("s-tab")
    def _cyc_b(_): state.active_panel = (state.active_panel - 1) % 5

    @kb.add("l")
    def _lk(_): state.action_link()
    @kb.add("b")
    def _bk(_): state.action_backup()
    @kb.add("r")
    def _rf(_): state.refresh_modules(); state.refresh_backups(); state.log("info", "Refreshed")

    @kb.add("e")
    def _ed(event):
        p = state.current_tree_path()
        if not p: return
        event.app.suspend_to_background()
        subprocess.run([os.environ.get("EDITOR","vim"), str(p)])
        state.log("info", f"Edited {p.name}")

    @kb.add("q")
    @kb.add("c-c")
    def _quit(_): get_app().exit()

    win_mod = Window(content=FormattedTextControl(lambda: render_modules(state)), always_hide_cursor=True)
    win_bkp = Window(content=FormattedTextControl(lambda: render_backups(state)), always_hide_cursor=True)
    win_det = Window(content=FormattedTextControl(lambda: render_details(state)), always_hide_cursor=True)
    win_tree = Window(content=FormattedTextControl(lambda: render_tree(state)), always_hide_cursor=True)
    win_log = Window(content=FormattedTextControl(lambda: render_log(state)), always_hide_cursor=True)

    frame_mod = rounded_frame(win_mod, "1 MODULES", lambda: state.active_panel == TUIState.PANEL_MODULES, lambda: (state._mod_vt, len(state.module_names)-(state._mod_vt+state.mod_vis)))
    frame_bkp = rounded_frame(win_bkp, "2 BACKUPS", lambda: state.active_panel == TUIState.PANEL_BACKUPS, lambda: (state._bkp_vt, len(state.backup_entries)-(state._bkp_vt+state.bkp_vis)))
    frame_det = rounded_frame(win_det, "3 DETAILS", lambda: state.active_panel == TUIState.PANEL_DETAILS)
    frame_tree = rounded_frame(win_tree, "4 TREE", lambda: state.active_panel == TUIState.PANEL_TREE, lambda: (state._tree_vt, len(state.tree_items)-(state._tree_vt+state.tree_vis)))
    frame_log = rounded_frame(win_log, "5 LOG", lambda: state.active_panel == TUIState.PANEL_LOG)

    # Definimos los pesos explícitos para que prompt_toolkit divida bien el espacio
    left_col = HSplit([
        HSplit([frame_mod], height=D(weight=70)),
        gap_h(1),
        HSplit([frame_bkp], height=D(weight=30)),
    ])

    right_col = HSplit([
        HSplit([frame_det], height=D(weight=45)),
        gap_h(1),
        HSplit([frame_tree], height=D(weight=35)),
        gap_h(1),
        HSplit([frame_log], height=D(weight=20)),
    ])

    body = VSplit([
        HSplit([left_col], width=D(weight=40)),
        gap_v(1),
        HSplit([right_col], width=D(weight=60))
    ])

    root = HSplit([gap_h(1), VSplit([gap_v(2), body, gap_v(2)]), gap_h(1), Window(content=FormattedTextControl(lambda: render_footer(state)), height=1), gap_h(1)])

    return Application(layout=Layout(root), key_bindings=kb, style=TUI_STYLE, full_screen=True)

def dashboard():
    config = DotsConfig.load()
    state = TUIState(config)
    _build_app(state).run()

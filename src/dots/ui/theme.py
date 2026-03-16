"""
Centralized theme for the dots CLI TUI and terminal output.
Uses a professional 'Gentleman' palette (inspired by Kanagawa/Ghostty).
"""

from prompt_toolkit.styles import Style
from InquirerPy.utils import get_style

# ── Color Palette (HEX) ──
# Main tones (Gentleman / Kanagawa inspired)
# ACCENT     = "#7fb4ca"  # Muted blue/cyan (Kanagawa: Blue)
# SUCCESS    = "#b7cc85"  # Muted green (Kanagawa: Green)
# WARNING    = "#ffe066"  # Soft yellow (Kanagawa: Yellow)
# ERROR      = "#cb7c94"  # Muted red/pink (Kanagawa: Red/Pink)
# DIM        = "#8a8fa3"  # Blueish gray (Kanagawa: Gray)
# FG         = "#f3f6f9"  # Off-white / foreground
# BG         = "#06080f"  # Deep background
# SEL_BG     = "#263356"  # Selection / active background
# BORDER_ACT = ACCENT
# BORDER_DIM = "#2A2A37"  # Dark border (Kanagawa: Sumi Ink)

ACCENT = "#b4befe"  # Lavender
SUCCESS = "#a6e3a1"  # Green
WARNING = "#f9e2af"  # Yellow
ERROR = "#f38ba8"  # Red
DIM = "#9399b2"  # Overlay
FG = "#cdd6f4"  # Text
BG = "#1e1e2e"  # Base
SEL_BG = "#45475a"  # Surface1
BORDER_ACT = ACCENT
BORDER_DIM = "#313244"  # Surface0


# ── Rich Theme Styles (for terminal output and status panels) ──
RICH_THEME_DICT = {
    "info": f"{ACCENT}",
    "warning": f"bold {WARNING}",
    "error": f"bold {ERROR}",
    "success": f"bold {SUCCESS}",
    "accent": ACCENT,
    "dim": f"{DIM}",
    "dim.italic": f"italic {DIM}",
    "menu.active": f"bold {FG} on {SEL_BG}",
    "menu.normal": FG,
    "menu.hint": f"dim {ACCENT}",
    "table.border": f"{BORDER_DIM}",
}

# ── Prompt Toolkit Styles (for the dashboard) ──
TUI_STYLE_DICT = {
    "": FG,
    # Frame borders
    "border": BORDER_DIM,
    "border.active": f"bold {BORDER_ACT}",
    "panel.title": f"bold {FG}",
    "panel.title.active": f"bold {ACCENT}",

    # Module table rows
    "row": FG,
    "row.sel": f"fg:{ACCENT} bold",

    # Column header
    "col.header": f"bold {ACCENT}",

    # Status colors
    "status.linked": f"bold {SUCCESS}",
    "status.unlinked": DIM,
    "status.broken": f"bold {ERROR}",

    # Selection circles (Visual mode)
    "select.on": f"bold {ACCENT}",
    "select.off": DIM,

    # Tab bar
    "tab.active": f"bold {ACCENT}",
    "tab.inactive": DIM,

    # Details (kept for tab content)
    "det.label": f"bold {FG}",
    "det.dim": DIM,
    "det.linked": SUCCESS,
    "det.pending": WARNING,
    "det.conflict": ERROR,
    "det.unsafe": ERROR,
    "det.missing": DIM,

    # Backups
    "bkp.item": FG,
    "bkp.sel": f"bold {ACCENT}",

    # Log
    "log.ts": DIM,
    "log.info": ACCENT,
    "log.success": SUCCESS,
    "log.warning": WARNING,
    "log.error": ERROR,

    # Footer
    "footer": f"{DIM}",
    "footer.key": f"bold {ACCENT}",
    "footer.sep": BORDER_DIM,

    # Sub-binding overlay (edit/order sub-modes)
    "sub.key": f"bold {ACCENT}",
    "sub.label": DIM,

    # Mode indicator
    "mode.normal": f"bold {SUCCESS}",
    "mode.visual": f"bold {WARNING}",

    # Inline filter
    "filter.prompt": f"bold {ACCENT}",
    "filter.text": FG,
}

# ── InquirerPy Styles (shared for all prompts) ──
PROMPT_STYLE_DICT = {
    "questionmark": f"bold {ACCENT}",
    "answermark": f"bold {SUCCESS}",
    "answer": f"bold {FG}",
    "input": f"{FG}",
    "question": f"bold {FG}",
    "instruction": f"italic {DIM}",
    "pointer": f"bold {ACCENT}",
    "checkbox": f"{ACCENT}",
    "separator": f"{BORDER_DIM}",
    "marker": f"bold {SUCCESS}",
    "fuzzy_prompt": f"bold {ACCENT}",
    "fuzzy_info": f"{DIM}",
}

# InquirerPy requires its own style object wrapper to avoid AttributeError
PROMPT_STYLE = get_style(PROMPT_STYLE_DICT, style_override=True)

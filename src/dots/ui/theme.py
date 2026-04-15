"""
Centralized theme for the dots CLI terminal output.
Inspired by Kanagawa/Ghostty palette.
"""

from InquirerPy.utils import get_style

# ── Color Palette (HEX) ──
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

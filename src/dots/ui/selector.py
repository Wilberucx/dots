"""
Reusable interactive selection helpers for dots CLI.
Built on InquirerPy — consistent keybindings and style across all commands.
"""
from typing import List, Optional
from InquirerPy import inquirer
from InquirerPy.base.control import Choice
from dots.core.config import DotsConfig
from dots.ui.theme import PROMPT_STYLE


def _checkbox(
    message: str,
    choices: List[Choice],
    preselect_all: bool = True,
) -> List[str]:
    """
    Base checkbox selector — shared keybindings and style.
    Returns list of selected values.
    """
    if not choices:
        return []

    selected = inquirer.checkbox(
        message=message,
        choices=choices,
        style=PROMPT_STYLE,
        keybindings={
            "toggle":     [{"key": "space"}, {"key": "tab"}],
            "down":       [{"key": "down"},  {"key": "j"}],
            "up":         [{"key": "up"},    {"key": "k"}],
            "toggle-all": [{"key": "a"}],
        },
        instruction="(↑↓/jk navigate · Space/Tab toggle · a toggle-all · Enter confirm)",
    ).execute()

    return selected if selected else []


def select_modules(
    config: DotsConfig,
    preselect_all: bool = True,
    modules: Optional[List[str]] = None,
    types: Optional[List[str]] = None,
) -> List[str]:
    """
    Interactive module selector.

    Args:
        config: DotsConfig instance
        preselect_all: All modules selected by default if True
        modules: Optional pre-filter by module name
        types: Optional pre-filter by module type

    Returns:
        List of selected module names
    """
    dirs = config.get_module_dirs(modules=modules, types=types)

    if not dirs:
        return []

    choices = [
        Choice(value=d.name, name=d.name, enabled=preselect_all)
        for d in dirs
    ]

    return _checkbox(
        message="Select modules (Space/Tab to toggle, Enter to confirm):",
        choices=choices,
        preselect_all=preselect_all,
    )


def select_variant(variants: List[str], current: Optional[str] = None) -> Optional[str]:
    """
    Interactive single-choice variant selector.

    Args:
        variants: List of available variant source names
        current: Currently active variant (shown as default)

    Returns:
        Selected variant name, or None if cancelled
    """
    if not variants:
        return None

    choices = [
        Choice(
            value=v,
            name=f"{v}  ← active" if v == current else v,
            enabled=(v == current),
        )
        for v in variants
    ]

    selected = inquirer.select(
        message="Select variant to activate:",
        choices=choices,
        style=PROMPT_STYLE,
        keybindings={
            "down": [{"key": "down"}, {"key": "j"}],
            "up":   [{"key": "up"},   {"key": "k"}],
        },
        instruction="(↑↓/jk navigate · Enter confirm · Ctrl+C cancel)",
    ).execute()

    return selected


def confirm(message: str, default: bool = False) -> bool:
    """
    Simple yes/no confirmation prompt.
    Used by adopt and other commands that need user confirmation.
    """
    return inquirer.confirm(
        message=message,
        default=default,
        style=PROMPT_STYLE,
    ).execute()

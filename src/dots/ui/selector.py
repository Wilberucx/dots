"""
Interactive TUI for selecting dotfiles modules.
"""
from typing import List
from InquirerPy import inquirer
from InquirerPy.base.control import Choice
from dots.core.config import DotsConfig
from dots.ui.theme import PROMPT_STYLE


def select_modules(config: DotsConfig, preselect_all: bool = True) -> List[str]:
    """
    Show interactive TUI to select which modules to link.
    
    Args:
        config: DotsConfig instance
        preselect_all: If True, all modules are selected by default
    
    Returns:
        List of selected module names
    """
    gentleman_dirs = config.get_module_dirs()
    
    if not gentleman_dirs:
        return []
    
    # Create choices with module names
    choices = [
        Choice(value=d.name, name=d.name, enabled=preselect_all)
        for d in gentleman_dirs
    ]
    
    selected = inquirer.checkbox(
        message="Select modules to link (Space/Tab to select, Enter to confirm):",
        choices=choices,
        style=PROMPT_STYLE,
        keybindings={
            "toggle": [{"key": "space"}, {"key": "tab"}],
            "down": [{"key": "down"}, {"key": "j"}],
            "up": [{"key": "up"}, {"key": "k"}],
            "toggle-all": [{"key": "a"}],
        },
        instruction="(↑↓/jk to navigate, Space/Tab to toggle, 'a' to toggle all, Enter to confirm)",
    ).execute()
    
    return selected if selected else []

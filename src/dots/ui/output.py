from rich.console import Console
from rich.theme import Theme
from dots.ui.theme import ACCENT, RICH_THEME_DICT

# Define custom theme for consistent output
custom_theme = Theme(RICH_THEME_DICT)

console = Console(theme=custom_theme)

def print_header(text: str):
    # Using hex accent from theme
    console.print(f"[bold {ACCENT}]==== {text} ====[/bold {ACCENT}]")

def print_success(text: str):
    console.print(f"[success]✔ {text}[/success]")

def print_error(text: str):
    console.print(f"[error]✖ {text}[/error]")

def print_warning(text: str):
    console.print(f"[warning]⚠ {text}[/warning]")

def print_info(text: str):
    console.print(f"[info]ℹ {text}[/info]")

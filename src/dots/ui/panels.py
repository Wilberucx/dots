"""
Interactive TUI panels for each dots command.

Each panel wraps the core logic of a command in an interactive presentation
suitable for the dashboard. These do NOT replace the inline CLI commands.
"""
import shutil
import subprocess
from pathlib import Path
from typing import Optional

from InquirerPy import inquirer
from InquirerPy.base.control import Choice
from rich.panel import Panel
from rich.table import Table
from rich.text import Text
from rich.prompt import Confirm

from dots.ui.output import console
from dots.ui.theme import ACCENT, FG, PROMPT_STYLE
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules


def _pause(msg: str = "Press Enter to return..."):
    """Wait for user input before returning to dashboard."""
    console.print()
    console.input(f"[dim]{msg}[/dim]")


def _clear():
    """Clear the terminal screen."""
    console.clear()


# ═══════════════════════════════════════════════════════════════════
#  STATUS PANEL
# ═══════════════════════════════════════════════════════════════════

def status_panel():
    """Rich table-based status panel for all modules."""
    _clear()
    config = DotsConfig.load()
    modules = resolve_modules(config)

    if not modules:
        console.print(Panel("[yellow]No modules found.[/yellow]", title="Status"))
        _pause()
        return

    table = Table(
        title=f"[bold {ACCENT}]Dotfiles Status[/bold {ACCENT}]",
        border_style="table.border",
        show_lines=False,
        pad_edge=True,
        expand=True,
    )
    table.add_column("Module", style=f"bold {FG}", min_width=14)
    table.add_column("Files", justify="center", style="dim", min_width=6)
    table.add_column("State", justify="center", min_width=12)
    table.add_column("Details", style="dim", ratio=1)

    totals = {"linked": 0, "pending": 0, "conflict": 0}

    for module_name, statuses in sorted(modules.items()):
        linked = sum(1 for s in statuses if s.state == "linked")
        pending = sum(1 for s in statuses if s.state == "pending")
        conflicts = sum(1 for s in statuses if s.state in ("conflict", "unsafe"))

        totals["linked"] += linked
        totals["pending"] += pending
        totals["conflict"] += conflicts

        total_files = len(statuses)

        # State badge
        if conflicts > 0:
            state = Text("● conflict", style="error")
            details = f"{conflicts} conflict{'s' if conflicts > 1 else ''}"
        elif pending > 0:
            state = Text("● pending", style="warning")
            details = f"{pending} to link"
        elif linked > 0:
            state = Text("● linked", style="success")
            details = "all synced"
        else:
            state = Text("○ empty", style="dim")
            details = "no files"

        table.add_row(module_name, str(total_files), state, details)

    console.print()
    console.print(table)

    # Summary bar
    summary_parts = []
    if totals["linked"]:
        summary_parts.append(f"[success]{totals['linked']} linked[/success]")
    if totals["pending"]:
        summary_parts.append(f"[warning]{totals['pending']} pending[/warning]")
    if totals["conflict"]:
        summary_parts.append(f"[error]{totals['conflict']} conflicts[/error]")

    console.print()
    console.print(
        Panel(
            " · ".join(summary_parts),
            title=f"[bold {ACCENT}]Summary[/bold {ACCENT}]",
            border_style="table.border",
            expand=True,
        )
    )
    _pause()


# ═══════════════════════════════════════════════════════════════════
#  LINK PANEL
# ═══════════════════════════════════════════════════════════════════

def link_panel():
    """Interactive module selector → link."""
    _clear()
    config = DotsConfig.load()
    module_dirs = config.get_module_dirs()

    if not module_dirs:
        console.print("[yellow]No modules found.[/yellow]")
        _pause()
        return

    choices = [
        Choice(value=d.name, name=d.name, enabled=True)
        for d in module_dirs
    ]

    selected = inquirer.checkbox(
        message="Select modules to link (Space to toggle, Enter to confirm):",
        choices=choices,
        style=PROMPT_STYLE,
        keybindings={
            "toggle": [{"key": "space"}, {"key": "tab"}],
            "down": [{"key": "down"}, {"key": "j"}],
            "up": [{"key": "up"}, {"key": "k"}],
            "toggle-all": [{"key": "a"}],
        },
        instruction="(↑↓/jk navigate, Space toggle, 'a' all, Enter confirm, Ctrl+C cancel)",
    ).execute()

    if not selected:
        console.print("[dim]No modules selected.[/dim]")
        _pause()
        return

    # Import and run link logic
    from dots.commands.link import link_cmd
    try:
        link_cmd(modules_args=selected, dry_run=False, force=False, interactive=False)
    except SystemExit:
        pass
    _pause()


# ═══════════════════════════════════════════════════════════════════
#  UNLINK PANEL
# ═══════════════════════════════════════════════════════════════════

def unlink_panel():
    """Interactive module selector → unlink."""
    _clear()
    config = DotsConfig.load()
    modules = resolve_modules(config)

    # Only show modules that have linked files
    linked_modules = {
        name: statuses for name, statuses in modules.items()
        if any(s.state == "linked" for s in statuses)
    }

    if not linked_modules:
        console.print("[yellow]No linked modules to unlink.[/yellow]")
        _pause()
        return

    choices = [
        Choice(
            value=name,
            name=f"{name} ({sum(1 for s in statuses if s.state == 'linked')} files)",
            enabled=False,
        )
        for name, statuses in sorted(linked_modules.items())
    ]

    selected = inquirer.checkbox(
        message="Select modules to unlink (Space to toggle, Enter to confirm):",
        choices=choices,
        style=PROMPT_STYLE,
        keybindings={
            "toggle": [{"key": "space"}, {"key": "tab"}],
            "down": [{"key": "down"}, {"key": "j"}],
            "up": [{"key": "up"}, {"key": "k"}],
            "toggle-all": [{"key": "a"}],
        },
        instruction="(↑↓/jk navigate, Space toggle, 'a' all, Enter confirm, Ctrl+C cancel)",
    ).execute()

    if not selected:
        console.print("[dim]No modules selected.[/dim]")
        _pause()
        return

    if not Confirm.ask(f"[warning]Unlink {len(selected)} module(s)?[/warning]", console=console):
        console.print("[dim]Cancelled.[/dim]")
        _pause()
        return

    from dots.commands.unlink import unlink_cmd
    try:
        unlink_cmd(modules_args=selected, dry_run=False, interactive=False)
    except SystemExit:
        pass
    _pause()


# ═══════════════════════════════════════════════════════════════════
#  INSTALL PANEL
# ═══════════════════════════════════════════════════════════════════

def install_panel():
    """Run install command (already interactive)."""
    _clear()
    from dots.commands.install import install_cmd
    try:
        install_cmd(dry_run=False, tools=None)
    except SystemExit:
        pass
    _pause()


# ═══════════════════════════════════════════════════════════════════
#  ADOPT PANEL
# ═══════════════════════════════════════════════════════════════════

def _fzf_pick_path() -> Optional[str]:
    """Use fzf to pick a file/directory. Returns path string or None."""
    try:
        result = subprocess.run(
            ["fzf", "--prompt=Select file to adopt: ", "--height=40%", "--reverse"],
            capture_output=True,
            text=True,
        )
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
    except FileNotFoundError:
        pass
    return None


def _inquirer_pick_path() -> Optional[str]:
    """Fallback fuzzy file picker using InquirerPy."""
    home = str(Path.home())
    result = inquirer.filepath(
        message="Select file or directory to adopt:",
        default=home + "/",
        style=PROMPT_STYLE,
        keybindings={
            "down": [{"key": "down"}, {"key": "j"}],
            "up": [{"key": "up"}, {"key": "k"}],
        },
    ).execute()
    return result if result else None


def adopt_panel():
    """File picker → adopt into dotfiles."""
    _clear()
    console.print(f"[bold {ACCENT}]Adopt Configuration[/bold {ACCENT}]")
    console.print()

    # Try fzf first, fall back to InquirerPy
    if shutil.which("fzf"):
        console.print("[dim]Using fzf for file selection...[/dim]")
        console.print()
        picked = _fzf_pick_path()
    else:
        picked = _inquirer_pick_path()

    if not picked:
        console.print("[dim]No file selected.[/dim]")
        _pause()
        return

    path = Path(picked).resolve()
    if not path.exists():
        console.print(f"[red]Path does not exist: {path}[/red]")
        _pause()
        return

    console.print(f"  Selected: [bold]{path}[/bold]")
    console.print()

    # Ask for module name
    default_name = path.name.capitalize()
    module_name = inquirer.text(
        message="Module name:",
        default=default_name,
        style=PROMPT_STYLE,
    ).execute()

    if not module_name:
        console.print("[dim]Cancelled.[/dim]")
        _pause()
        return

    from dots.commands.adopt import adopt_cmd
    try:
        adopt_cmd(path=path, name=module_name, dry_run=False)
    except SystemExit:
        pass
    _pause()


# ═══════════════════════════════════════════════════════════════════
#  BACKUP PANEL
# ═══════════════════════════════════════════════════════════════════

def backup_panel():
    """Interactive backup with editable commit message."""
    _clear()
    from dots.commands.backup import run_backup, default_commit_message

    config = DotsConfig.load()
    dots_dir = config.repo_root

    default_msg = default_commit_message()

    console.print(f"[bold {ACCENT}]Backup Dotfiles[/bold {ACCENT}]")
    console.print()

    commit_msg = inquirer.text(
        message="Commit message:",
        default=default_msg,
        style=PROMPT_STYLE,
        long_instruction="(Edit or press Enter for default)",
    ).execute()

    if not commit_msg:
        commit_msg = default_msg

    console.print()
    success = run_backup(commit_msg, dots_dir)

    if success:
        push = Confirm.ask("[accent]Push to remote?[/accent]", console=console)
        if push:
            run_backup_push(dots_dir)

    _pause()


def run_backup_push(dots_dir: Path):
    """Push to remote after backup."""
    try:
        subprocess.run(
            ["git", "push"],
            cwd=dots_dir,
            check=True,
            capture_output=True,
        )
        console.print("[green]✔[/green] git push")
    except subprocess.CalledProcessError as e:
        console.print(f"[red]✘ git push failed:[/red] {e.stderr.decode()}")

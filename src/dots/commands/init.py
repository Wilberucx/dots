import os
import typer
from pathlib import Path
from rich.console import Console

from dots.core.config import MARKER_DIR, MARKER_CONFIG, LEGACY_MARKER, is_dotfiles_repo

console = Console()


def _detect_shell() -> tuple[str, Path]:
    """Detect user's shell and its config file."""
    # Check $SHELL first
    shell = os.environ.get("SHELL", "")
    if "zsh" in shell:
        return ("zsh", Path.home() / ".zshrc")
    if "bash" in shell:
        return ("bash", Path.home() / ".bashrc")
    if "fish" in shell:
        return ("fish", Path.home() / ".config" / "fish" / "config.fish")

    # Fallback: check which config files exist
    home = Path.home()
    if (home / ".zshrc").exists():
        return ("zsh", home / ".zshrc")
    if (home / ".bashrc").exists():
        return ("bash", home / ".bashrc")
    if (home / ".config" / "fish" / "config.fish").exists():
        return ("fish", home / ".config" / "fish" / "config.fish")

    return ("unknown", Path.home() / ".zshrc")  # Default guess


def _add_to_shell_config(config_path: Path, export_line: str):
    """Add export line to shell config file."""
    try:
        with open(config_path, "a") as f:
            f.write(f"\n# dots: dotfiles repository\n{export_line}\n")
    except Exception as e:
        console.print(f"[red]Failed to write to {config_path}: {e}[/red]")


def init_cmd():
    """
    Initialize a new dotfiles repository by creating .dots/config.yaml.

    This command should be run at the root of your dotfiles repository.
    It tells the dots CLI that this directory is managed by it.

    Backward compatibility: legacy dots.toml is still recognized.
    """
    cwd = Path.cwd()
    marker_dir = cwd / MARKER_DIR
    marker_path = marker_dir / MARKER_CONFIG
    legacy_path = cwd / LEGACY_MARKER

    # Check if already initialized (new or legacy format)
    if is_dotfiles_repo(cwd):
        if marker_path.exists():
            console.print(f"[yellow]Dotfiles already initialized in {cwd}[/yellow]")
        else:
            console.print(f"[yellow]Legacy marker '{LEGACY_MARKER}' found — migrating to new format...[/yellow]")
            _migrate_from_legacy(cwd, marker_dir, marker_path)
        return

    try:
        # Create .dots directory and config.yaml
        marker_dir.mkdir(exist_ok=True)
        with open(marker_path, "w") as f:
            f.write(f"# {MARKER_DIR}/{MARKER_CONFIG} — marker for the dots CLI\n")
            f.write("# This file identifies this directory as a dotfiles repository managed by dots.\n")
            f.write("# See: https://github.com/cantoarch/dots\n\n")
            f.write("[dots]\nversion = \"1\"\n")
    except Exception as e:
        console.print(f"[red]Error creating marker file: {e}[/red]")
        raise typer.Exit(code=1)

    console.print(f"[green]Successfully initialized dotfiles repository in {cwd}[/green]")
    console.print(f"Created '{MARKER_DIR}/{MARKER_CONFIG}'.")

    # Ask about DOTS_REPO (outside main try to avoid masking marker creation errors)
    shell_name, shell_config = _detect_shell()
    export_line = f'export DOTS_REPO="{cwd.resolve()}"'

    console.print(f"\n[bold]DOTS_REPO[/bold] tells dots where to find your dotfiles.")
    console.print(f"Without it, you must be inside your dotfiles directory or use [green]--path[/green].")

    try:
        add_repo = typer.confirm(f"Add DOTS_REPO to your {shell_config.name}?")
    except Exception:
        add_repo = False  # No TTY available

    if add_repo:
        _add_to_shell_config(shell_config, export_line)
        console.print(f"[green]Added to {shell_config}[/green]")
        console.print(f"[dim]Run [italic]source {shell_config}[/italic] or restart your terminal.[/dim]")
    else:
        console.print(f"\n[dim]You can add it manually:[/dim]")
        console.print(f"  [green]{export_line}[/green]")
        console.print(f"\n[dim]Or use [green]dots --path /path/to/dotfiles <command>[/green] instead.[/dim]")


def _migrate_from_legacy(cwd: Path, marker_dir: Path, marker_path: Path):
    """Migrate from legacy dots.toml to new .dots/config.yaml format."""
    legacy_path = cwd / LEGACY_MARKER
    try:
        # Read legacy content
        legacy_content = legacy_path.read_text()

        # Create new config
        marker_dir.mkdir(exist_ok=True)
        with open(marker_path, "w") as f:
            f.write(f"# {MARKER_DIR}/{MARKER_CONFIG} — marker for the dots CLI\n")
            f.write("# Migrated from legacy dots.toml\n\n")
            f.write(legacy_content)

        console.print(f"[green]Migrated {LEGACY_MARKER} to {MARKER_DIR}/{MARKER_CONFIG}[/green]")
        console.print(f"[dim]You can safely delete {LEGACY_MARKER} if desired.[/dim]")
    except Exception as e:
        console.print(f"[yellow]Migration failed: {e}[/yellow]")

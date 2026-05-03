import typer
from pathlib import Path
from rich.console import Console

from dots.core.config import MARKER_DIR, MARKER_CONFIG, LEGACY_MARKER, is_dotfiles_repo

console = Console()


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

        console.print(f"[green]Successfully initialized dotfiles repository in {cwd}[/green]")
        console.print(f"Created '{MARKER_DIR}/{MARKER_CONFIG}'.")

        # Suggest adding DOTS_REPO to shell profile
        console.print(f"\n[bold]Optional:[/bold] To auto-detect this repo,")
        console.print(f"add to your shell profile (e.g. ~/.zshrc):")
        console.print(f"\n  [green]export DOTS_REPO=\"{cwd.resolve()}\"[/green]\n")

    except Exception as e:
        console.print(f"[red]Error creating marker file: {e}[/red]")
        raise typer.Exit(code=1)


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

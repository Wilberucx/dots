import typer
from pathlib import Path
from rich.console import Console

from dots.core.config import MARKER_FILE

console = Console()

def init_cmd():
    """
    Initialize a new dotfiles repository by creating a dots.toml marker file.
    
    This command should be run at the root of your dotfiles repository.
    It tells the dots CLI that this directory is managed by it.
    """
    cwd = Path.cwd()
    marker_path = cwd / MARKER_FILE
    dotsrc_path = Path.home() / ".dotsrc"
    
    if marker_path.exists():
        console.print(f"[yellow]Marker file '{MARKER_FILE}' already exists in {cwd}[/yellow]")
        # Even if marker exists, ensure .dotsrc points here
        _update_dotsrc(dotsrc_path, cwd)
        return
        
    try:
        with open(marker_path, "w") as f:
            f.write(f"# {MARKER_FILE} \u2014 marker file for the dots CLI\n")
            f.write("# This file identifies this directory as a dotfiles repository managed by dots.\n")
            f.write("# See: https://github.com/cantoarch/dots\n\n")
            f.write("[dots]\nversion = \"1\"\n")
            
        console.print(f"[green]Successfully initialized dotfiles repository in {cwd}[/green]")
        console.print(f"Created '{MARKER_FILE}' marker file.")
        _update_dotsrc(dotsrc_path, cwd)
    except Exception as e:
        console.print(f"[red]Error creating marker file: {e}[/red]")
        raise typer.Exit(code=1)

def _update_dotsrc(dotsrc_path: Path, repo_path: Path):
    """Update or create the ~/.dotsrc file with the repository path."""
    try:
        with open(dotsrc_path, "w") as f:
            f.write(f"DOTS_REPO=\"{repo_path.resolve()}\"\n")
        
        console.print(f"\n[bold]➤ Global path registered[/bold]")
        console.print(f"[dim]The file {dotsrc_path} was created so this repo can be found automatically.[/dim]")
        
        console.print(f"\n[bold cyan]Optional (for advanced users):[/bold cyan]")
        console.print(f"If you prefer not to have a `.dotsrc` file in your home directory,")
        console.print(f"you can export the path directly in your shell profile (e.g. `~/.zshrc` or `~/.bashrc`):")
        console.print(f"\n[green]  export DOTS_REPO=\"{repo_path.resolve()}\"[/green]")
        console.print(f"\nOr run this to append it automatically to your zsh profile:")
        console.print(f"[bold]  echo 'export DOTS_REPO=\"{repo_path.resolve()}\"' >> ~/.zshrc && source ~/.zshrc[/bold]\n")
        
    except Exception as e:
        console.print(f"[red]Failed to update global {dotsrc_path}: {e}[/red]")

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
    
    if marker_path.exists():
        console.print(f"[yellow]Marker file '{MARKER_FILE}' already exists in {cwd}[/yellow]")
        return
        
    try:
        with open(marker_path, "w") as f:
            f.write(f"# {MARKER_FILE} \u2014 marker file for the dots CLI\n")
            f.write("# This file identifies this directory as a dotfiles repository managed by dots.\n")
            f.write("# See: https://github.com/cantoarch/dots\n\n")
            f.write("[dots]\nversion = \"1\"\n")
            
        console.print(f"[green]Successfully initialized dotfiles repository in {cwd}[/green]")
        console.print(f"Created '{MARKER_FILE}' marker file.")
    except Exception as e:
        console.print(f"[red]Error creating marker file: {e}[/red]")
        raise typer.Exit(code=1)

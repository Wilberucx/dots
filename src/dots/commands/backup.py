import subprocess
import typer
from datetime import datetime
from pathlib import Path
from dots.ui.output import console, print_header
from rich.prompt import Confirm


def default_commit_message() -> str:
    """Generate the default timestamped commit message."""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    return f"backup: {timestamp}"


def run_backup(commit_msg: str, dots_dir: Path, push: bool = False) -> bool:
    """
    Core backup logic: git add, commit, optionally push.

    Args:
        commit_msg: Commit message to use
        dots_dir: Path to the dotfiles repository root
        push: Whether to push after committing

    Returns:
        True if backup was successful, False otherwise
    """
    try:
        subprocess.run(
            ["git", "add", "."],
            cwd=dots_dir,
            check=True,
            capture_output=True,
        )
        console.print("[green]✔[/green] git add .")
    except subprocess.CalledProcessError as e:
        console.print(f"[red]✘ git add failed:[/red] {e.stderr.decode()}")
        return False

    try:
        result = subprocess.run(
            ["git", "diff", "--cached", "--quiet"],
            cwd=dots_dir,
            capture_output=True,
        )

        if result.returncode == 0:
            console.print("[yellow]ℹ[/yellow] No changes to commit")
            return True

        subprocess.run(
            ["git", "commit", "-m", commit_msg],
            cwd=dots_dir,
            check=True,
            capture_output=True,
        )
        console.print(f'[green]✔[/green] git commit -m "{commit_msg}"')
    except subprocess.CalledProcessError as e:
        console.print(f"[red]✘ git commit failed:[/red] {e.stderr.decode()}")
        return False

    if push:
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
            return False

    return True


def backup_cmd():
    """
    Backup dotfiles with git commit and optional push.
    """
    print_header("Dots Backup")

    dots_dir = Path(__file__).parent.parent.parent.parent
    commit_msg = default_commit_message()

    push = False
    success = run_backup(commit_msg, dots_dir)

    if success:
        if Confirm.ask("[cyan]Push to remote?[/cyan]"):
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
                raise typer.Exit(1)
    else:
        raise typer.Exit(1)
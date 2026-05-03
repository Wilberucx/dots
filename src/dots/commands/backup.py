import subprocess
import typer
from datetime import datetime
from pathlib import Path
from dots.ui.output import console, print_header
from dots.core.config import DotsConfig


backup_app = typer.Typer(help="Backup dotfiles repository")


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


@backup_app.command(name="run")
def backup_cmd(
    push: bool = typer.Option(False, "--push", "-p", help="Push to remote after commit"),
    message: str = typer.Option(None, "--message", "-m", help="Commit message (default: timestamp)"),
):
    """
    Backup dotfiles with git commit and optional push.
    """
    print_header("Dots Backup")

    config = DotsConfig.load()
    dots_dir = config.repo_root
    commit_msg = message if message else default_commit_message()

    success = run_backup(commit_msg, dots_dir, push=push)

    if not success:
        raise typer.Exit(1)


@backup_app.command(name="list")
def list_cmd(
    limit: int = typer.Option(10, "--limit", "-n", help="Cantidad de backups a mostrar"),
):
    """Lista los últimos backups."""
    print_header("Backup History")

    config = DotsConfig.load()
    dots_dir = config.repo_root

    try:
        result = subprocess.run(
            ["git", "log", f"-{limit}", "--format=%h|%ar|%s"],
            cwd=dots_dir,
            check=True,
            capture_output=True,
            text=True,
        )

        lines = result.stdout.strip().split("\n")
        if not lines or lines == [""]:
            console.print("[yellow]ℹ[/yellow] No hay backups en el historial")
            return

        for line in lines:
            if "|" in line:
                hash_part, rest = line.split("|", 1)
                console.print(f"[yellow]{hash_part}[/yellow] {rest}")

    except subprocess.CalledProcessError as e:
        console.print(f"[red]✘ git log failed:[/red] {e.stderr}")
        raise typer.Exit(1)


@backup_app.command(name="diff")
def diff_cmd(
    ref: str = typer.Argument("HEAD~1", help="Commit o ref para comparar contra HEAD"),
):
    """Muestra qué cambió desde el último backup o un ref específico."""
    print_header(f"Diff: {ref} → HEAD")

    config = DotsConfig.load()
    dots_dir = config.repo_root

    try:
        subprocess.run(
            ["git", "diff", "--stat", ref, "HEAD"],
            cwd=dots_dir,
            check=True,
        )
    except subprocess.CalledProcessError as e:
        console.print(f"[red]✘ git diff failed:[/red] {e.stderr}")
        raise typer.Exit(1)
import os
import subprocess
import typer
from datetime import datetime
from pathlib import Path
from dots.ui.output import console, print_header
from dots.core.config import DotsConfig


# ─── Funciones de sync ────────────────────────────────────────────────────────


def get_upstream_branch(dots_dir: Path) -> str | None:
    """Return upstream branch (e.g. 'origin/main') or None if not configured."""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"],
            cwd=dots_dir,
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip()
        return None
    except Exception:
        return None


def get_remote_ahead_count(dots_dir: Path, upstream: str) -> int:
    """Return how many commits the remote has that local does not (HEAD..upstream)."""
    try:
        result = subprocess.run(
            ["git", "rev-list", "--count", f"HEAD..{upstream}"],
            cwd=dots_dir,
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            return int(result.stdout.strip())
        return 0
    except Exception:
        return 0


def get_conflict_files(dots_dir: Path) -> list[str]:
    """Return list of files with unresolved merge conflicts."""
    result = subprocess.run(
        ["git", "diff", "--name-only", "--diff-filter=U"],
        cwd=dots_dir,
        capture_output=True,
        text=True,
    )
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def sync_from_remote(dots_dir: Path) -> dict:
    """
    Fetch remote and rebase local commits on top if remote is ahead.

    Returns a dict with keys:
        status   : "clean" | "pulled" | "conflicts" | "no_upstream" | "error"
        conflicts: list[str]   — populated only when status == "conflicts"
        ahead    : int         — commits pulled from remote
    """
    upstream = get_upstream_branch(dots_dir)

    if not upstream:
        return {"status": "no_upstream", "conflicts": [], "ahead": 0}

    subprocess.run(
        ["git", "fetch", "--quiet", "origin"],
        cwd=dots_dir,
        capture_output=True,
        timeout=10,
    )

    ahead = get_remote_ahead_count(dots_dir, upstream)

    if ahead == 0:
        return {"status": "clean", "conflicts": [], "ahead": 0}

    console.print(f"[yellow]⚠[/yellow] Remote has {ahead} new commit(s) — pulling...")

    result = subprocess.run(
        ["git", "pull", "--autostash", "--rebase"],
        cwd=dots_dir,
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        conflicts = get_conflict_files(dots_dir)
        if conflicts:
            return {"status": "conflicts", "conflicts": conflicts, "ahead": ahead}
        console.print(f"[red]✘[/red] Pull failed: {result.stderr}")
        return {"status": "error", "conflicts": [], "ahead": 0}

    return {"status": "pulled", "conflicts": [], "ahead": ahead}


def resolve_conflicts_interactive(dots_dir: Path, conflicts: list[str]) -> bool:
    """
    Offer interactive conflict resolution via $EDITOR, then continue rebase.

    Returns True if the rebase was completed successfully and backup can continue.
    Returns False if the user aborted or something failed.
    """
    console.print("[red]✘[/red] Conflicts in:")
    for f in conflicts:
        console.print(f"   • {f}")

    if not typer.confirm("Do you want to resolve them in your $EDITOR?"):
        subprocess.run(["git", "rebase", "--abort"], cwd=dots_dir, capture_output=True)
        console.print("[dim]Rebase aborted — resolve manually then re-run[/dim]")
        return False

    editor = os.environ.get("EDITOR", "vim")
    try:
        subprocess.run([editor, *conflicts], cwd=dots_dir, check=True)
    except Exception as e:
        console.print(f"[red]✘[/red] Failed to open editor: {e}")
        subprocess.run(["git", "rebase", "--abort"], cwd=dots_dir, capture_output=True)
        return False

    # Verificar que no queden conflictos sin resolver antes de git add
    remaining = get_conflict_files(dots_dir)
    if remaining:
        console.print("[red]✘[/red] Still unresolved after editing:")
        for f in remaining:
            console.print(f"   • {f}")
        subprocess.run(["git", "rebase", "--abort"], cwd=dots_dir, capture_output=True)
        console.print("[dim]Rebase aborted — fix all conflicts then re-run[/dim]")
        return False

    subprocess.run(["git", "add", *conflicts], cwd=dots_dir, check=True)

    if not typer.confirm("Continue with rebase?"):
        subprocess.run(["git", "rebase", "--abort"], cwd=dots_dir, capture_output=True)
        console.print("[dim]Rebase aborted — resolve manually then re-run[/dim]")
        return False

    result = subprocess.run(
        ["git", "rebase", "--continue"],
        cwd=dots_dir,
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        console.print(f"[red]✘[/red] Failed to continue rebase: {result.stderr}")
        return False

    return True


backup_app = typer.Typer(help="Backup dotfiles repository")


def default_commit_message() -> str:
    """Generate the default timestamped commit message."""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    return f"backup: {timestamp}"


def run_backup(
    commit_msg: str, dots_dir: Path, push: bool = True, no_sync: bool = False
) -> bool:
    """
    Core backup logic: git add, commit, optionally push.

    Args:
        commit_msg: Commit message to use
        dots_dir: Path to the dotfiles repository root
        push: Whether to push after committing (default: True)
        no_sync: Skip remote sync check and push directly

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

    # Commit first, then sync. This is intentional: --rebase places local commits
    # on top of remote history, so your current system state always ends up as HEAD.
    if push:
        if not no_sync:
            sync_result = sync_from_remote(dots_dir)

            if sync_result["status"] == "no_upstream":
                console.print(
                    "[dim]ℹ No upstream branch configured — skipping sync[/dim]"
                )

            elif sync_result["status"] == "pulled":
                console.print(
                    f"[green]✔[/green] Pulled {sync_result['ahead']} commit(s) from remote"
                )

            elif sync_result["status"] == "conflicts":
                resolved = resolve_conflicts_interactive(
                    dots_dir, sync_result["conflicts"]
                )
                if not resolved:
                    return False

            elif sync_result["status"] == "error":
                return False

            # "clean" — no output, continuar silenciosamente

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
    push: bool = typer.Option(
        True, "--push/--no-push", help="Push to remote after commit (default: on)"
    ),
    no_sync: bool = typer.Option(
        False, "--no-sync", help="Skip remote sync check, push directly"
    ),
    message: str = typer.Option(
        None, "--message", "-m", help="Commit message (default: timestamp)"
    ),
):
    """
    Backup dotfiles with git commit and optional push.
    """
    print_header("Dots Backup")

    config = DotsConfig.load()
    dots_dir = config.repo_root
    commit_msg = message if message else default_commit_message()

    success = run_backup(commit_msg, dots_dir, push=push, no_sync=no_sync)

    if not success:
        raise typer.Exit(1)


@backup_app.command(name="list")
def list_cmd(
    limit: int = typer.Option(
        10, "--limit", "-n", help="Cantidad de backups a mostrar"
    ),
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


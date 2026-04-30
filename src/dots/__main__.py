import typer
from pathlib import Path
from importlib.metadata import version as get_version
from dots.ui.output import console

app = typer.Typer(
    name="dots",
    help="dots — dotfile manager",
    add_completion=False,
)

def show_banner():
    """Display ASCII banner from assets/ASCII file."""
    banner_path = Path(__file__).parent.parent / "ASCII"
    if banner_path.exists():
        with open(banner_path, 'r') as f:
            banner = f.read()
            console.print(f"[bold cyan]{banner}[/bold cyan]")
            console.print()  # Empty line after banner

def version_callback(value: bool):
    if value:
        ver = get_version("dots")
        typer.echo(f"dots v{ver}")
        raise typer.Exit()

@app.callback(invoke_without_command=True)
def main_callback(
    ctx: typer.Context,
    version: bool = typer.Option(
        None, "--version", "-v",
        callback=version_callback,
        is_eager=True,
        help="Show version and exit."
    )
):
    """
    Unified CLI for managing dotfiles on Linux, macOS, and Windows.
    """
    if ctx.invoked_subcommand is None:
        # Show help message by default
        typer.echo(ctx.get_help())


from dots.commands import link, install, adopt, status, unlink, backup, init, edit, list as list_mod

app.command(name="init")(init.init_cmd)
app.command(name="link")(link.link_cmd)
app.command(name="unlink")(unlink.unlink_cmd)
app.command(name="install")(install.install_cmd)
app.command(name="status")(status.status_cmd)
app.command(name="adopt")(adopt.adopt_cmd)
app.command(name="backup")(backup.backup_cmd)
app.command(name="edit")(edit.edit_cmd)
app.command(name="list")(list_mod.list_cmd)
app.command(name="ls", hidden=True)(list_mod.list_cmd)


if __name__ == "__main__":
    app()

def main():
    app()

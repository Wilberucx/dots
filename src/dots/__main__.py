import typer
from pathlib import Path
from dots.ui.output import console

app = typer.Typer(
    name="dots",
    help="Gentleman Dotfiles Manager",
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

@app.callback(invoke_without_command=True)
def main_callback(ctx: typer.Context):
    """
    Unified CLI for managing dotfiles on Linux, macOS, and Windows.
    """
    if ctx.invoked_subcommand is None:
        # Launch interactive dashboard
        from dots.ui.dashboard import dashboard
        dashboard()


from dots.commands import link, install, adopt, status, unlink, backup

app.command(name="link")(link.link_cmd)
app.command(name="unlink")(unlink.unlink_cmd)
app.command(name="install")(install.install_cmd)
app.command(name="status")(status.status_cmd)
app.command(name="adopt")(adopt.adopt_cmd)
app.command(name="backup")(backup.backup_cmd)

if __name__ == "__main__":
    app()

def main():
    app()

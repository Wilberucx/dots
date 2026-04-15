import os
import shlex
import subprocess
import typer
from pathlib import Path
from dots.core.config import DotsConfig
from dots.ui.output import print_error, print_info

def edit_cmd(
    module: str = typer.Argument(..., help="Name of the module to edit"),
    config_file: bool = typer.Option(
        False, "--config", "-c", help="Edit the path.yaml configuration file"
    ),
):
    """
    Open a module folder or its path.yaml in your $EDITOR.
    """
    config = DotsConfig.load()
    module_path = config.repo_root / module

    if not module_path.exists():
        print_error(f"Module '{module}' not found in {config.repo_root}")
        raise typer.Exit(1)

    target = module_path
    if config_file:
        yaml_path = module_path / "path.yaml"
        if not yaml_path.exists():
            print_error(f"Module '{module}' does not have a path.yaml file.")
            raise typer.Exit(1)
        target = yaml_path

    editor_env = os.environ.get("EDITOR", "vim")
    print_info(f"Opening {target} with {editor_env}...")

    cmd = shlex.split(editor_env) + [str(target)]
    try:
        subprocess.run(cmd, check=True)
    except Exception as e:
        print_error(f"Could not open editor: {e}")
        raise typer.Exit(1)

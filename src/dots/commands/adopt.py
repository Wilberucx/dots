import typer
import shutil
import os
from pathlib import Path
from typing import Optional
import yaml


from dots.ui.output import (
    console, print_header, print_success,
    print_error, print_warning, print_info,
)
from dots.ui.selector import confirm
from dots.core.config import DotsConfig
from dots.core.transaction import TransactionLog




def _destination_str(abs_path: Path, home_dir: Path) -> str:
    """Return ~-relative destination string."""
    try:
        return f"~/{abs_path.relative_to(home_dir)}"
    except ValueError:
        return str(abs_path)


def _load_yaml(yaml_path: Path) -> dict:
    """Load path.yaml safely, return empty structure on error."""
    if not yaml_path.exists():
        return {"files": []}
    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
            return data if data and isinstance(data, dict) else {"files": []}
    except yaml.YAMLError:
        return {"files": []}


def _destination_already_declared(data: dict, destination: str) -> bool:
    """Return True if destination is already in path.yaml files list."""
    for entry in data.get("files", []):
        if isinstance(entry, dict):
            declared = (
                entry.get("destination")
                or entry.get("destination-linux")
                or entry.get("destination-mac")
            )
            if declared == destination:
                return True
    return False


def adopt_cmd(
    path: Path = typer.Argument(
        ..., help="Path to the file or directory to adopt", exists=True
    ),
    name: str = typer.Option(
        None, "--name", "-n", help="Module name (e.g. Zsh)"
    ),
    dry_run: bool = typer.Option(
        False, "--dry-run", help="Show what would be done without executing"
    ),
):
    """
    Import a config file into the dotfiles repo and register it in path.yaml.

    If the module already exists and the destination is already declared,
    offers to create a new variant instead of overwriting.
    """
    print_header("Adopting Configuration")


    config = DotsConfig.load()
    abs_path = path.resolve()

    # Safety check — outside HOME
    if not str(abs_path).startswith(str(config.home_dir)):
        print_warning(f"{abs_path} is outside HOME.")
        if not confirm("Proceed anyway?", default=False):
            raise typer.Abort()

    # Determine module name
    if not name:
        from InquirerPy import inquirer
        from dots.ui.theme import PROMPT_STYLE
        name = inquirer.text(
            message="Module name:",
            default=path.name.capitalize(),
            style=PROMPT_STYLE,
        ).execute()

    module_dir = config.repo_root / name
    yaml_path = module_dir / "path.yaml"
    destination = _destination_str(abs_path, config.home_dir)
    data = _load_yaml(yaml_path)

    # ── Case 1: module exists + destination already declared → variant ──
    if module_dir.exists() and _destination_already_declared(data, destination):
        print_info(
            f"Module [bold]{name}[/bold] already declares "
            f"[dim]{destination}[/dim] as a destination."
        )

        if not confirm(
            f"Create a new variant in '{name}' for this file?",
            default=True,
        ):
            print_info("Adoption cancelled.")
            raise typer.Abort()

        # Ask for variant folder name
        from InquirerPy import inquirer
        from dots.ui.theme import PROMPT_STYLE
        variant_name = inquirer.text(
            message="Variant name (will be used as subfolder):",
            default=path.stem,
            style=PROMPT_STYLE,
        ).execute()

        variant_dir = module_dir / variant_name
        target_file = variant_dir / path.name
        variant_source = f"{variant_name}/{path.name}"

        if dry_run:
            print_info(f"[DRY] Would create: {variant_dir}/")
            print_info(f"[DRY] Would move {abs_path} → {target_file}")
            print_info(
                f"[DRY] Would add variant entry to {yaml_path}: "
                f"source={variant_source}, destination={destination}"
            )
            return

        transaction = TransactionLog()
        try:
            variant_dir.mkdir(parents=True, exist_ok=True)
            if target_file.exists():
                print_error(f"{target_file} already exists in repo.")
                raise typer.Exit(1)

            transaction.backup(abs_path, target_file)
            print_success(f"Moved {path.name} → {variant_dir}/")

            # Append variant entry to path.yaml
            new_entry = {
                "source": variant_source,
                "destination": destination,
            }
            data["files"].append(new_entry)
            with open(yaml_path, "w") as f:
                yaml.dump(data, f, sort_keys=False)

            print_success(f"Added variant '{variant_name}' to {yaml_path}")
            transaction.commit()

        except Exception as e:
            transaction.rollback()
            print_error(f"Adopt failed: {e}. Changes rolled back.")
            raise typer.Exit(1)

        print_info(
            f"Run [bold]dots link -m {name} --variant {variant_source}[/bold] "
            f"to activate this variant."
        )
        return

    # ── Case 2: module doesn't exist or destination is new → normal adopt ──
    target_file = module_dir / path.name

    if dry_run:
        print_info(f"[DRY] Would create directory: {module_dir}")
        print_info(f"[DRY] Would move {abs_path} → {target_file}")
        print_info(
            f"[DRY] Would {'update' if yaml_path.exists() else 'create'} "
            f"path.yaml with destination={destination}"
        )
        return

    transaction = TransactionLog()
    try:
        module_dir.mkdir(parents=True, exist_ok=True)

        if target_file.exists():
            print_error(f"{target_file} already exists in repo.")
            raise typer.Exit(1)

        transaction.backup(abs_path, target_file)
        print_success(f"Moved {path.name} → {module_dir}/")

        # Append to files list
        new_entry = {
            "source": path.name,
            "destination": destination,
        }
        data["files"].append(new_entry)

        with open(yaml_path, "w") as f:
            yaml.dump(data, f, sort_keys=False)

        print_success(f"{'Updated' if module_dir.exists() else 'Created'} {yaml_path}")
        transaction.commit()

    except Exception as e:
        transaction.rollback()
        print_error(f"Adopt failed: {e}. Changes rolled back.")
        raise typer.Exit(1)

    print_info(f"Run [bold]dots link -m {name}[/bold] to create the symlink.")

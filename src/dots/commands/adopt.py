import typer
from pathlib import Path

from dots.ui.output import (
    console, print_header, print_success,
    print_error, print_warning, print_info,
)
from dots.ui.selector import confirm
from dots.core.config import DotsConfig
from dots.core.transaction import TransactionLog
from dots.core.module_writer import (
    destination_str,
    load_module_data,
    is_destination_declared,
    append_file_entry,
)


def _prompt_module_name(path: Path) -> str:
    from InquirerPy import inquirer
    from dots.ui.theme import PROMPT_STYLE
    return inquirer.text(
        message="Module name:",
        default=path.name.capitalize(),
        style=PROMPT_STYLE,
    ).execute()


def _prompt_variant_name(path: Path) -> str:
    from InquirerPy import inquirer
    from dots.ui.theme import PROMPT_STYLE
    return inquirer.text(
        message="Variant name (will be used as subfolder):",
        default=path.stem,
        style=PROMPT_STYLE,
    ).execute()


def _do_adopt(
    abs_path: Path,
    target_file: Path,
    yaml_path: Path,
    entry: dict,
    transaction: TransactionLog,
    dry_run: bool,
    label: str,
) -> None:
    """
    Lógica común para adopt normal y adopt con variant.
    Mueve el archivo, actualiza path.yaml.
    """
    if dry_run:
        print_info(f"[DRY] Would move {abs_path} → {target_file}")
        print_info(f"[DRY] Would write entry to {yaml_path}: {entry}")
        return

    if target_file.exists():
        print_error(f"{target_file} already exists in repo.")
        raise typer.Exit(1)

    transaction.move(abs_path, target_file)
    append_file_entry(yaml_path, entry)
    print_success(f"Moved {abs_path.name} → {target_file.parent}/")
    print_success(f"{label} {yaml_path}")


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
    name = name or _prompt_module_name(path)

    module_dir = config.repo_root / name
    yaml_path = module_dir / "path.yaml"
    destination = destination_str(abs_path, config.home_dir)
    data = load_module_data(yaml_path)

    transaction = TransactionLog()
    try:
        # Case 1: variant
        if module_dir.exists() and is_destination_declared(data, destination):
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

            variant_name = _prompt_variant_name(path)
            variant_dir = module_dir / variant_name
            entry = {"source": f"{variant_name}/{path.name}", "destination": destination}
            
            _do_adopt(abs_path, variant_dir / path.name, yaml_path, entry, transaction, dry_run, "Updated")
            
            if not dry_run:
                transaction.commit()
                print_info(f"Run [bold]dots link -m {name} --variant {variant_name}[/bold]")
        
        # Case 2: adopt normal
        else:
            entry = {"source": path.name, "destination": destination}
            _do_adopt(abs_path, module_dir / path.name, yaml_path, entry, transaction, dry_run, "Created")
            
            if not dry_run:
                transaction.commit()
                print_info(f"Run [bold]dots link -m {name}[/bold]")

    except Exception as e:
        transaction.rollback()
        print_error(f"Adopt failed: {e}. Changes rolled back.")
        raise typer.Exit(1)

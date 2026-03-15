import typer
import shutil
import os
from pathlib import Path
from rich.prompt import Prompt, Confirm
import yaml
from dots.ui.output import console, print_header, print_success, print_error, print_warning, print_info
from dots.core.config import DotsConfig
from dots.core.transaction import TransactionLog

def adopt_cmd(
    path: Path = typer.Argument(..., help="Path to the file or directory to adopt", exists=True),
    name: str = typer.Option(None, "--name", "-n", help="Name of the module (e.g. Zsh)"),
    dry_run: bool = typer.Option(False, "--dry-run", help="Show what would be done"),
):
    """
    Import a config file into the Dotfiles repo and create a path.yaml.
    """
    print_header("Adopting Configuration")
    
    config = DotsConfig.load()
    abs_path = path.resolve()
    
    if not str(abs_path).startswith(str(config.home_dir)):
        print_warning(f"File {abs_path} is outside HOME. Are you sure?")
        if not Confirm.ask("Proceed?"):
            raise typer.Abort()

    # Determine Module Name
    if not name:
        default_name = path.name.capitalize()
        name = Prompt.ask("Module Name", default=default_name)

    module_dir = config.repo_root / name
    
    print_info(f"Target Module: {module_dir}")

    # Calculate relative destination path (path relative to HOME)
    # destination in yaml usually uses ~
    try:
        rel_to_home = abs_path.relative_to(config.home_dir)
        destination_str = f"~/{rel_to_home}"
    except ValueError:
        destination_str = str(abs_path)

    # Prepare file move
    target_file = module_dir / path.name
    
    # Check if module exists
    if module_dir.exists():
        print_info(f"Module {name} already exists.")
    else:
        print_info(f"Creating module {name}...")

    if dry_run:
        print_info(f"[DRY] Would create directory: {module_dir}")
        print_info(f"[DRY] Would move {abs_path} -> {target_file}")
        print_info(f"[DRY] Would create path.yaml pointing to {destination_str}")
        return

    # Execute
    transaction = TransactionLog()
    
    try:
        module_dir.mkdir(parents=True, exist_ok=True)
        
        if target_file.exists():
            print_error(f"File {target_file} already exists in repo.")
            raise typer.Exit(1)
            
        # Move file (record as backup for rollback)
        transaction.backup(abs_path, target_file)
        print_success(f"Moved {path.name} to {module_dir}")
        
        # Create/Update path.yaml
        yaml_path = module_dir / "path.yaml"
        
        new_entry = {
            "source": path.name,
            "destination": destination_str
        }
        
        data = {"files": []}
        if yaml_path.exists():
            with open(yaml_path, 'r') as f:
                try:
                    loaded = yaml.safe_load(f)
                    if loaded and 'files' in loaded:
                        data = loaded
                except yaml.YAMLError:
                    pass
        
        data['files'].append(new_entry)
        
        with open(yaml_path, 'w') as f:
            yaml.dump(data, f, sort_keys=False)
            
        print_success(f"Updated {yaml_path}")
        transaction.commit()
        
    except Exception as e:
        transaction.rollback()
        print_error(f"Adopt failed: {e}. Changes rolled back.")
        raise typer.Exit(1)
    
    # Suggest running dot link
    print_info("Run 'dots link' to create the symlink back.")

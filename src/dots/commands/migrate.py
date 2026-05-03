"""Migrate path.yaml files from v2 to v3 format."""

import typer
from pathlib import Path
from typing import List
import yaml

from dots.core.config import DotsConfig
from dots.ui.output import console

# Mapeo de campos dependencies: v2 -> v3
DEPENDENCY_FIELD_MAP = {
    "source": "url",
    "target": "dest",
    "extract-path": "extract",
    "arch_map": "arch",
    "package-managers": "managers",
}

# Mapeo de campos files: v2 -> v3
FILE_FIELD_MAP = {
    "destination-linux": "per-os",
    "destination-mac": "per-os",
    "destination-override": "per-os",
}


def find_all_path_yaml(repo_root: Path) -> List[Path]:
    """Find all path.yaml files in the repository."""
    return list(repo_root.rglob("path.yaml"))


def migrate_file(file_path: Path, dry_run: bool = False) -> bool:
    """
    Migrate a single path.yaml file from v2 to v3.
    Returns True if file was modified, False otherwise.
    """
    if not file_path.exists():
        return False

    try:
        with open(file_path, "r") as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError as e:
        console.print(f"[red]Error parsing {file_path}: {e}[/red]")
        return False

    if not data or not isinstance(data, dict):
        return False

    modified = False

    # Migrate dependencies
    dependencies = data.get("dependencies", [])
    if isinstance(dependencies, list):
        new_dependencies = []
        for dep in dependencies:
            if isinstance(dep, dict):
                migrated_dep = _migrate_dependency(dep)
                if migrated_dep != dep:
                    modified = True
                new_dependencies.append(migrated_dep)
            else:
                new_dependencies.append(dep)

        if modified:
            data["dependencies"] = new_dependencies

    # Migrate files
    files = data.get("files", [])
    if isinstance(files, list):
        new_files = []
        for f in files:
            if isinstance(f, dict):
                migrated_file = _migrate_file(f)
                if migrated_file != f:
                    modified = True
                new_files.append(migrated_file)
            else:
                new_files.append(f)

        if modified:
            data["files"] = new_files

    if modified and not dry_run:
        # Write back with same formatting style
        with open(file_path, "w") as f:
            yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False, allow_unicode=True)

    return modified


def _migrate_dependency(dep: dict) -> dict:
    """Migrate a single dependency from v2 to v3."""
    result = dep.copy()

    # Migrate field names
    for old_field, new_field in DEPENDENCY_FIELD_MAP.items():
        if old_field in result and new_field not in result:
            result[new_field] = result.pop(old_field)

    # Migrate type: system -> package
    dep_type = result.get("type", "")
    if dep_type == "system":
        result["type"] = "package"

    return result


def _migrate_file(file_entry: dict) -> dict:
    """Migrate a single file entry from v2 to v3."""
    result = file_entry.copy()

    # Migrate destination-linux/destination-mac to per-os
    dest_linux = result.pop("destination-linux", None)
    dest_mac = result.pop("destination-mac", None)

    if dest_linux or dest_mac:
        per_os = result.get("per-os", {})
        if not isinstance(per_os, dict):
            per_os = {}

        if dest_linux:
            per_os["linux"] = dest_linux
        if dest_mac:
            per_os["mac"] = dest_mac

        result["per-os"] = per_os

    # Migrate destination-override to per-os
    dest_override = result.pop("destination-override", None)
    if dest_override:
        per_os = result.get("per-os", {})
        if not isinstance(per_os, dict):
            per_os = {}

        # destination-override applies to all OS, merge in
        if isinstance(dest_override, dict):
            for os_name, dest in dest_override.items():
                per_os[os_name] = dest
        else:
            # String value means override all
            per_os["linux"] = dest_override
            per_os["mac"] = dest_override

        result["per-os"] = per_os

    return result


def migrate_cmd(
    dry_run: bool = typer.Option(False, "--dry-run", help="Preview changes without modifying files"),
    yes: bool = typer.Option(False, "--yes", "-y", help="Skip confirmation prompt"),
):
    """
    Migrate all path.yaml files in the repository from v2 to v3 format.

    This command automatically converts:
    - dependency field names (source->url, target->dest, etc.)
    - type: system -> type: package
    - file destinations (destination-linux/mac -> per-os)
    """
    config = DotsConfig.load()
    repo_root = config.repo_root

    console.print(f"[cyan]Scanning {repo_root} for path.yaml files...[/cyan]")

    path_yaml_files = find_all_path_yaml(repo_root)

    if not path_yaml_files:
        console.print("[yellow]No path.yaml files found.[/yellow]")
        return

    console.print(f"[green]Found {len(path_yaml_files)} path.yaml file(s).[/green]")

    # Preview phase - show what would change
    files_to_migrate = []
    for file_path in path_yaml_files:
        modified = migrate_file(file_path, dry_run=True)
        rel_path = file_path.relative_to(repo_root)
        if modified:
            console.print(f"  [yellow]→ {rel_path}[/yellow] (needs migration)")
            files_to_migrate.append(file_path)
        else:
            console.print(f"  [dim]→ {rel_path}[/dim] (already v3)")

    if not files_to_migrate:
        console.print("[green]All files already at v3 format.[/green]")
        return

    if dry_run:
        console.print(f"\n[yellow]--dry-run: no files were modified.[/yellow]")
        return

    # Confirmation prompt unless -y is passed
    if not yes:
        console.print(f"\n[cyan]Migrate {len(files_to_migrate)} file(s)?[/cyan]")
        confirm = typer.prompt("Press Enter to proceed or 'n' to cancel", default="y")
        if confirm.lower() != "y":
            console.print("[yellow]Aborted.[/yellow]")
            return

    # Apply migration
    for file_path in files_to_migrate:
        migrate_file(file_path, dry_run=False)
        rel_path = file_path.relative_to(repo_root)
        console.print(f"[green]✓ {rel_path}[/green]")

    console.print(f"\n[bold green]Migration complete: {len(files_to_migrate)} file(s) updated.[/bold green]")
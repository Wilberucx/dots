import typer
import subprocess
import shutil
import os
import platform
import tarfile
import zipfile
import requests
from pathlib import Path
from typing import List, Dict, Optional
from rich.prompt import Confirm
from rich.table import Table
from rich.progress import Progress, SpinnerColumn, TextColumn
from dots.ui.output import console, print_header, print_success, print_error, print_warning, print_info
from dots.plugins.managers import get_package_manager, PackageManager
from dots.core.config import DotsConfig
from dots.core.yaml_parser import parse_dependencies, Dependency

def get_system_arch() -> str:
    """Detect system architecture (x86_64, aarch64, etc)."""
    machine = platform.machine().lower()
    if machine in ['x86_64', 'amd64']:
        return 'x86_64'
    elif machine in ['aarch64', 'arm64']:
        return 'aarch64'
    return machine

def expand_path(path_str: str) -> Path:
    """Expand ~ and env vars in path."""
    return Path(os.path.expandvars(os.path.expanduser(path_str)))

def install_git_dep(dep: Dependency, dry_run: bool):
    """Handle git repository cloning."""
    if not dep.source or not dep.target:
        print_warning(f"Skipping git dependency '{dep.name}': missing source or target.")
        return

    dest = expand_path(dep.target)
    if dest.exists():
        print_info(f"  [skip] {dep.name} already exists at {dest}")
        return

    print_info(f"  [git] Cloning {dep.name} to {dest}...")
    if not dry_run:
        try:
            subprocess.run(
                ["git", "clone", dep.source, str(dest)],
                check=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )
            if dep.ref:
                print_info(f"  [git] Checking out ref: {dep.ref}")
                subprocess.run(
                    ["git", "-C", str(dest), "checkout", dep.ref],
                    check=True,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL
                )
            print_success(f"  Installed {dep.name}")
        except subprocess.CalledProcessError:
            print_error(f"  Failed to install {dep.name}")

def install_package_dep(dep: Dependency, manager: PackageManager, dry_run: bool):
    """
    Handle type: package dependencies with per-manager name mapping.
    
    Looks up the package name for the current manager in dep.package_managers.
    If the current manager is not listed, skips with a warning.
    If already installed (shutil.which), skips silently.
    """
    if not dep.package_managers:
        # Fallback: use dep.name directly (same as type: system)
        pkg_name = dep.name
    else:
        pkg_name = dep.package_managers.get(manager.name)
        if pkg_name is None:
            print_warning(f"  [skip] {dep.name}: not available for {manager.name}")
            return

    if shutil.which(dep.name):
        print_info(f"  [skip] {dep.name} already installed")
        return

    cmd = manager.install_command([pkg_name])
    if manager.needs_sudo:
        cmd = ["sudo"] + cmd

    print_info(f"  [pkg] Installing {dep.name} as '{pkg_name}' via {manager.name}...")

    if dry_run:
        print_info(f"  [DRY] Would run: {' '.join(cmd)}")
        return

    try:
        subprocess.run(cmd, check=True)
        print_success(f"  Installed {dep.name}")
    except subprocess.CalledProcessError:
        print_error(f"  Failed to install {dep.name}")

def install_binary_dep(dep: Dependency, dry_run: bool):
    """Handle binary download and extraction."""
    if not dep.source or not dep.target:
        print_warning(f"Skipping binary dependency '{dep.name}': missing source or target.")
        return
    
    dest = expand_path(dep.target)
    if dest.exists():
        print_info(f"  [skip] {dep.name} already exists at {dest}")
        return

    # Handle templating
    arch = get_system_arch()
    url = dep.source.replace("{{arch}}", arch)
    if dep.arch_map:
        # If arch_map is provided, use it to resolve the arch string
        mapped_arch = dep.arch_map.get(arch, arch)
        url = dep.source.replace("{{arch}}", mapped_arch)

    if dep.version:
        url = url.replace("{{version}}", dep.version)

    print_info(f"  [bin] Downloading {dep.name} from {url}...")
    
    if dry_run:
        return

    try:
        # Download to temp file
        import tempfile
        with tempfile.NamedTemporaryFile(delete=False) as tmp:
            response = requests.get(url, stream=True)
            response.raise_for_status()
            for chunk in response.iter_content(chunk_size=8192):
                tmp.write(chunk)
            tmp_path = Path(tmp.name)

        # Create parent dir
        dest.parent.mkdir(parents=True, exist_ok=True)

        # Extract or move
        if url.endswith(".tar.gz") or url.endswith(".tgz"):
             with tarfile.open(tmp_path, "r:gz") as tar:
                # Security: simplistic extraction, filtering members usually recommended
                tar.extractall(path=dest.parent)
                # Assume binary name matches dep.name or we might need a specific 'extract_file' field
                # For now, just extracting to parent dir. 
                # Improvement: handle 'extract_path' logic.
        else:
             # Assume raw binary
             shutil.move(tmp_path, dest)
             dest.chmod(0o755)

        print_success(f"  Installed {dep.name}")
        
    except Exception as e:
        print_error(f"  Failed to install {dep.name}: {e}")

def run_post_install(dep: Dependency, dry_run: bool):
    """Run post-install command if exists."""
    if dep.post_install:
        print_info(f"  [exec] Running post-install for {dep.name}...")
        if not dry_run:
            subprocess.run(dep.post_install, shell=True)

def install_cmd(
    dry_run: bool = typer.Option(False, "--dry-run", help="Show commands without executing"),
    modules: List[str] = typer.Argument(None, help="Specific modules to install (default: all)")
):
    """
    Install dependencies declared in path.yaml files across all modules.
    """
    print_header("Installing Dependencies")

    manager = get_package_manager()
    if not manager:
        print_error("No supported package manager found (pacman, apt, brew).")
        raise typer.Exit(1)

    print_info(f"Detected Package Manager: [bold]{manager.name}[/bold]")
    print_info(f"Architecture: [bold]{get_system_arch()}[/bold]")

    config = DotsConfig.load()
    all_dependencies: List[Dependency] = []

    # Load dependencies from modules
    module_dirs = config.get_module_dirs()
    if modules:
        module_dirs = [d for d in module_dirs if d.name in modules]

    for module_dir in module_dirs:
        yaml_path = module_dir / "path.yaml"
        if yaml_path.exists():
            deps = parse_dependencies(yaml_path)
            existing_names = {d.name for d in all_dependencies}
            for d in deps:
                if d.name not in existing_names:
                    all_dependencies.append(d)
                    existing_names.add(d.name)

    if not all_dependencies:
        print_info("No dependencies found.")
        raise typer.Exit(0)

    # Install each dependency
    for dep in all_dependencies:
        if dep.type == "git":
            install_git_dep(dep, dry_run)
        elif dep.type == "binary":
            install_binary_dep(dep, dry_run)
        elif dep.type in ("package", "system"):  # "system" como alias legacy
            install_package_dep(dep, manager, dry_run)
        else:
            print_warning(f"  [skip] {dep.name}: unknown type '{dep.type}'")

        run_post_install(dep, dry_run)

    print_success("\nDependency installation process finished.")


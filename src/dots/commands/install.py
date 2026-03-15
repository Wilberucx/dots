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

# Map: generic_name -> {"pacman": "pkg", "apt": "pkg", "brew": "pkg"}
import yaml


def _load_package_map() -> dict:
    """Load package mappings from packages.yaml."""
    data_dir = Path(__file__).parent.parent / "data"
    yaml_path = data_dir / "packages.yaml"
    if not yaml_path.exists():
        return {}
    with open(yaml_path, 'r') as f:
        try:
            data = yaml.safe_load(f)
        except yaml.YAMLError:
            return {}
    return data if data else {}

PACKAGE_MAP = _load_package_map()


def get_mapped_package(tool: str, manager_name: str) -> Optional[str]:
    """
    Get the package name for a tool in the given package manager.
    
    Returns:
        Package name, or None if not available for this manager
    """
    if tool not in PACKAGE_MAP:
        return tool
    
    mapping = PACKAGE_MAP[tool]
    return mapping.get(manager_name)

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
            subprocess.run(["git", "clone", dep.source, str(dest)], check=True, stdout=subprocess.DEVNULL)
            print_success(f"  Installed {dep.name}")
        except subprocess.CalledProcessError:
            print_error(f"  Failed to clone {dep.name}")

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
    tools: List[str] = typer.Argument(None, help="Specific tools to install (default: all core tools)")
):
    """
    Install system dependencies and custom tools.
    """
    print_header("Installing Dependencies")
    
    manager = get_package_manager()
    if not manager:
        print_error("No supported package manager found (pacman, apt, brew).")
        raise typer.Exit(1)

    print_info(f"Detected Package Manager: [bold]{manager.name}[/bold]")
    print_info(f"Architecture: [bold]{get_system_arch()}[/bold]")

    all_dependencies: List[Dependency] = []

    # 1. Load default packages (legacy list support)
    # If user provided args, we skip loading default yaml unless they want it mixed? 
    # Usually args override defaults.
    
    if not tools:
        # Load from default_packages.yaml
        data_dir = Path(__file__).parent.parent / "data"
        default_yaml = data_dir / "default_packages.yaml"
        if default_yaml.exists():
            with open(default_yaml, 'r') as f:
                try:
                    data = yaml.safe_load(f)
                    if data and isinstance(data, dict):
                        # Parse defaults (assume strings)
                        pkg_list = data.get("packages", [])
                        for p in pkg_list:
                            all_dependencies.append(Dependency(name=p, type="system"))
                except yaml.YAMLError:
                    print_error("Failed to parse default_packages.yaml")
        
        # Fallback
        if not all_dependencies:
             defaults = ["git", "curl", "wget", "unzip", "gcc", "zsh", "starship", "tmux", "fzf", "rg", "bat", "eza"]
             for d in defaults:
                 all_dependencies.append(Dependency(name=d, type="system"))
    else:
        # User provided tools via CLI args (assume system packages)
        for t in tools:
            all_dependencies.append(Dependency(name=t, type="system"))

    # 2. Load module dependencies (path.yaml)
    config = DotsConfig.load()
    for module_dir in config.get_module_dirs():
        yaml_path = module_dir / "path.yaml"
        if yaml_path.exists():
            deps = parse_dependencies(yaml_path) # Now returns List[Dependency]
            # Avoid duplicates by name
            existing_names = {d.name for d in all_dependencies}
            for d in deps:
                if d.name not in existing_names:
                    all_dependencies.append(d)
                    existing_names.add(d.name)

    # 3. Sort into categories
    system_packages_to_install = []
    custom_deps = []
    
    already_installed_system = []
    unavailable_system = []

    for dep in all_dependencies:
        if dep.type == "system":
            # Check if installed
            if shutil.which(dep.name):
                already_installed_system.append(dep.name)
            else:
                mapped = get_mapped_package(dep.name, manager.name)
                if mapped is None:
                    unavailable_system.append(dep.name)
                else:
                    system_packages_to_install.append(mapped)
        else:
            custom_deps.append(dep)

    # 4. Report System Packages status
    if already_installed_system:
        print_success(f"Already installed (system): {', '.join(already_installed_system)}")
    
    if unavailable_system:
        print_warning(f"Not available via {manager.name}: {', '.join(unavailable_system)}")
    
    # 5. Install System Packages
    if system_packages_to_install:
        cmd = manager.install_command(system_packages_to_install)
        if manager.needs_sudo:
            full_cmd = ["sudo"] + cmd
        else:
            full_cmd = cmd

        console.print(f"\n[bold yellow]System Install Command:[/bold yellow] [cyan]{' '.join(full_cmd)}[/cyan]")
        
        if dry_run:
            print_info("[DRY] Would install system packages.")
        else:
            if Confirm.ask("Install system packages?"):
                try:
                    subprocess.run(full_cmd, check=True)
                    print_success("System packages installed.")
                except subprocess.CalledProcessError:
                    print_error("Failed to install system packages.")

    # 6. Install Custom Dependencies
    if custom_deps:
        print_header("Installing Custom Dependencies")
        for dep in custom_deps:
            if dep.type == "git":
                install_git_dep(dep, dry_run)
            elif dep.type == "binary":
                install_binary_dep(dep, dry_run)
            
            run_post_install(dep, dry_run)

    print_success("\nDependency installation process finished.")


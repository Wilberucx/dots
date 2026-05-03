"""
Shared configuration for Dots CLI.

This module provides a single source of truth for all runtime context,
eliminating fragile path resolution via __file__ parent chains.

Repo detection uses a marker directory strategy: .dots/config.yaml must exist at the
root of the dotfiles repository. This is portable across machines and
operating systems, with no hardcoded assumptions about installed tools.

Backward compatibility: legacy dots.toml marker is also supported.
"""
from dataclasses import dataclass
from pathlib import Path
from dots.core.system import detect_os

MARKER_DIR = ".dots"
MARKER_CONFIG = "config.yaml"
LEGACY_MARKER = "dots.toml"


def is_dotfiles_repo(path: Path) -> bool:
    """Check if path is a dotfiles repo (new .dots/config.yaml or legacy dots.toml)."""
    # New format: .dots/config.yaml
    if (path / MARKER_DIR / MARKER_CONFIG).exists():
        return True
    # Legacy format: dots.toml
    if (path / LEGACY_MARKER).exists():
        return True
    return False


@dataclass(frozen=True)
class DotsConfig:
    """Immutable runtime configuration for the Dots CLI."""
    
    repo_root: Path      # Root of Dot.files repository
    current_os: str      # "linux" | "mac" | "windows" | "unknown"
    home_dir: Path       # User's home directory
    cli_dir: Path        # cli/ subdirectory
    
    @classmethod
    def _create(cls, repo_root: Path) -> "DotsConfig":
        return cls(
            repo_root=repo_root.resolve(),
            current_os=detect_os(),
            home_dir=Path.home(),
            cli_dir=(repo_root / "cli").resolve()
        )

    @classmethod
    def load(cls) -> "DotsConfig":
        """
        Load configuration by finding the Dotfiles repository.

        Search order:
        1. DOTS_REPO environment variable (override)
        2. Walk up from current working directory
        3. Common locations in user's home (~/Dot.files, ~/.dotfiles, ~/dotfiles)
        """
        import os

        home = Path.home()

        # 1. Environment variable override
        if "DOTS_REPO" in os.environ:
            potential_root = Path(os.environ["DOTS_REPO"]).resolve()
            if is_dotfiles_repo(potential_root):
                return cls._create(potential_root)

        # 2. Walk up from CWD
        search_from = Path.cwd()
        for parent in [search_from] + list(search_from.parents):
            if is_dotfiles_repo(parent):
                return cls._create(parent)

        # 3. Common fallback locations
        for common_dir in ["Dot.files", ".dotfiles", "dotfiles"]:
            potential_root = home / common_dir
            if is_dotfiles_repo(potential_root):
                return cls._create(potential_root)

        raise RuntimeError(
            f"Could not find a dotfiles repository.\n"
            f"No '{MARKER_DIR}/{MARKER_CONFIG}' or '{LEGACY_MARKER}' found in current directory tree or common locations.\n"
            f"\nTo fix this, you have 3 options:\n"
            f"  1. [green]dots --path ~/your-dotfiles <command>[/green] — specify path directly\n"
            f"  2. [green]export DOTS_REPO=~/your-dotfiles[/green] — set environment variable\n"
            f"  3. [green]cd ~/your-dotfiles && dots init[/green] — initialize if not done yet"
        )
    
    def get_module_dirs(
        self,
        modules: list[str] | None = None,
        types: list[str] | None = None,
    ) -> list[Path]:
        """
        Return sorted list of all valid module directories.
        If modules is provided, return only those that match by name.
        If types is provided, return only those whose path.yaml has a matching type.
        Unknown module names are warned but do not raise.
        """
        all_dirs = sorted([
            d for d in self.repo_root.iterdir()
            if d.is_dir() and not d.name.startswith(".") and (d / "path.yaml").exists()
        ])

        result = all_dirs
        if modules:
            all_names = {d.name: d for d in all_dirs}
            result = []
            for name in modules:
                if name in all_names:
                    result.append(all_names[name])
                else:
                    # Import aquí para evitar circular
                    from dots.ui.output import print_warning
                    print_warning(f"Module '{name}' not found in repo — skipping.")
        
        if types:
            from dots.core.yaml_parser import parse_module_meta
            filtered = []
            for d in result:
                meta = parse_module_meta(d / "path.yaml")
                module_type = meta.get('type')
                if module_type in types:
                    filtered.append(d)
            result = filtered

        return result

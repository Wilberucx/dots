"""
Shared configuration for Dots CLI.

This module provides a single source of truth for all runtime context,
eliminating fragile path resolution via __file__ parent chains.

Repo detection uses a marker file strategy: dots.toml must exist at the
root of the dotfiles repository. This is portable across machines and
operating systems, with no hardcoded assumptions about installed tools.
"""
from dataclasses import dataclass
from pathlib import Path
from dots.core.system import detect_os

MARKER_FILE = "dots.toml"


@dataclass(frozen=True)
class DotsConfig:
    """Immutable runtime configuration for the Dots CLI."""
    
    repo_root: Path      # Root of Dot.files repository
    current_os: str      # "linux" | "mac" | "windows" | "unknown"
    home_dir: Path       # User's home directory
    cli_dir: Path        # cli/ subdirectory
    
    @classmethod
    def load(cls) -> "DotsConfig":
        """
        Load configuration by finding the Dotfiles repository.

        Searches from the current working directory upward for a directory
        containing a 'dots.toml' marker file. This approach is portable
        across machines and OS — no hardcoded tool names required.

        To set up a dotfiles repo for use with dots, create a dots.toml
        at the root of that repository:

            echo '[dots]\nversion = "1"' > ~/your-dotfiles/dots.toml
        """
        search_from = Path.cwd()

        for parent in [search_from] + list(search_from.parents):
            if (parent / MARKER_FILE).exists():
                repo_root = parent
                cli_dir = repo_root / "cli"
                return cls(
                    repo_root=repo_root.resolve(),
                    current_os=detect_os(),
                    home_dir=Path.home(),
                    cli_dir=cli_dir.resolve()
                )

        raise RuntimeError(
            f"Could not find a dotfiles repository. "
            f"No '{MARKER_FILE}' marker file found in '{search_from}' or any parent directory.\n"
            f"Create one at the root of your dotfiles repo:\n"
            f"  echo '[dots]\\nversion = \"1\"' > <your-dotfiles-root>/{MARKER_FILE}"
        )
    
    def get_module_dirs(self) -> list[Path]:
        """Return sorted list of all valid module directories (directories containing path.yaml)."""
        return sorted([
            d for d in self.repo_root.iterdir()
            if d.is_dir() and not d.name.startswith(".") and (d / "path.yaml").exists()
        ])

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
        1. Custom path in DOTS_REPO environment variable
        2. Global configuration file (~/.dotsrc) created by `dots init`
        3. Walk up from current working directory
        4. Common locations in user's home (~/Dot.files, ~/.dotfiles, ~/dotfiles)
        """
        import os
        
        # 1. Environment variable override
        if "DOTS_REPO" in os.environ:
            potential_root = Path(os.environ["DOTS_REPO"]).resolve()
            if (potential_root / MARKER_FILE).exists():
                return cls._create(potential_root)

        # 2. Global configuration file (~/.dotsrc)
        home = Path.home()
        dotsrc = home / ".dotsrc"
        if dotsrc.exists():
            try:
                for line in dotsrc.read_text().splitlines():
                    if line.startswith("DOTS_REPO="):
                        path_str = line.split("=", 1)[1].strip('"\'')
                        potential_root = Path(path_str).resolve()
                        if (potential_root / MARKER_FILE).exists():
                            return cls._create(potential_root)
            except Exception:
                pass  # Ignore malformed file

        # 3. Walk up from CWD
        search_from = Path.cwd()
        for parent in [search_from] + list(search_from.parents):
            if (parent / MARKER_FILE).exists():
                return cls._create(parent)
                
        # 4. Common fallback locations
        for common_dir in ["Dot.files", ".dotfiles", "dotfiles"]:
            potential_root = home / common_dir
            if (potential_root / MARKER_FILE).exists():
                return cls._create(potential_root)

        raise RuntimeError(
            f"Could not find a dotfiles repository.\n"
            f"No '{MARKER_FILE}' marker file found in current directory tree or common locations.\n"
            f"To fix this, create the marker file at the root of your dotfiles repo:\n"
            f"  cd ~/your-dotfiles && dots init\n"
            f"Or specify the location explicitly: export DOTS_REPO=~/your-dotfiles"
        )
    
    def get_module_dirs(self) -> list[Path]:
        """Return sorted list of all valid module directories (directories containing path.yaml)."""
        return sorted([
            d for d in self.repo_root.iterdir()
            if d.is_dir() and not d.name.startswith(".") and (d / "path.yaml").exists()
        ])

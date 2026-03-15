"""
Shared configuration for Dots CLI.

This module provides a single source of truth for all runtime context,
eliminating fragile path resolution via __file__ parent chains.
"""
from dataclasses import dataclass
from pathlib import Path
from dots.core.system import detect_os


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
        
        Searches from current working directory upward for a directory containing
        a .git folder and known dotfile module directories.
        """
        search_from = Path.cwd()
        
        for parent in [search_from] + list(search_from.parents):
            if (parent / ".git").exists():
                potential_root = parent
                module_indicators = ["Alacritty", "Fish", "Git", "Hypr", "Kitty", "Nvim", "Zsh"]
                has_modules = sum(1 for m in module_indicators if (potential_root / m).exists())
                
                if has_modules >= 3:
                    repo_root = potential_root
                    cli_dir = repo_root / "cli"
                    return cls(
                        repo_root=repo_root.resolve(),
                        current_os=detect_os(),
                        home_dir=Path.home(),
                        cli_dir=cli_dir.resolve()
                    )
        
        raise RuntimeError(
            "Could not find Dotfiles repository. "
            "Please run from within your Dot.files directory."
        )
    
    def get_module_dirs(self) -> list[Path]:
        """Return sorted list of all valid module directories (directories containing path.yaml)."""
        return sorted([
            d for d in self.repo_root.iterdir()
            if d.is_dir() and not d.name.startswith(".") and (d / "path.yaml").exists()
        ])

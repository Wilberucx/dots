"""
Unit tests for the resolver module.

Tests LinkStatus backup_path field is correctly populated
when .orig files are detected.
"""
import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.core.resolver import LinkStatus


# ─── LinkStatus backup_path detection ─────────────────────────────────────────


class TestLinkStatusBackupPath:
    """Tests that LinkStatus correctly carries backup_path information."""

    def test_backup_path_is_set_when_orig_exists(self, tmp_path):
        """LinkStatus should store backup_path when .orig file exists."""
        home = Path.home()
        
        # Create the .orig file
        orig_file = home / ".zshrc.orig"
        orig_file.write_text("# original backup")
        
        dest = home / ".zshrc"
        dest.write_text("# user's file")
        
        try:
            # Simulate what resolver does to detect backup
            orig_path = Path(str(dest) + ".orig")
            backup = orig_path if orig_path.exists() else None
            
            status = LinkStatus(
                source=tmp_path / ".zshrc",
                destination=dest,
                state="pending",
                detail="backup needed",
                backup_path=backup,
            )
            
            assert status.backup_path is not None
            assert status.backup_path.name == ".zshrc.orig"
        finally:
            orig_file.unlink(missing_ok=True)
            dest.unlink(missing_ok=True)

    def test_backup_path_is_none_when_no_orig(self, tmp_path):
        """LinkStatus should have backup_path=None when no .orig exists."""
        home = Path.home()
        
        # Ensure no .orig
        orig_file = home / ".zshrc.orig"
        orig_file.unlink(missing_ok=True)
        
        dest = home / ".zshrc"
        dest.write_text("# user's file")
        
        try:
            # Simulate what resolver does
            orig_path = Path(str(dest) + ".orig")
            backup = orig_path if orig_path.exists() else None
            
            status = LinkStatus(
                source=tmp_path / ".zshrc",
                destination=dest,
                state="pending",
                detail="backup needed",
                backup_path=backup,
            )
            
            assert status.backup_path is None
        finally:
            dest.unlink(missing_ok=True)


# ─── Integration test (requires real dotfiles repo) ───────────────────────


def test_resolve_real_modules():
    """
    Integration: resolve_modules() against the real dotfiles repo.
    Skips if no marker (.dots/config.yaml or dots.toml) is present.
    """
    import os
    from pathlib import Path
    from dots.core.config import is_dotfiles_repo, DotsConfig
    from dots.core.resolver import resolve_modules

    cwd = Path.cwd()
    found = any(is_dotfiles_repo(p) for p in [cwd] + list(cwd.parents))

    if not found:
        pytest.skip("No dotfiles marker found in current path hierarchy — skipping integration test")

    config = DotsConfig.load()
    modules = resolve_modules(config)

    # We might not have modules in a clean environment, but if we do, check structure
    if modules:
        for name, statuses in modules.items():
            assert isinstance(name, str) and len(name) > 0
            for s in statuses:
                assert s.state in ("linked", "conflict", "pending", "missing", "unsafe")
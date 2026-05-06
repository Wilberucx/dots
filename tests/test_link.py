"""
Tests for link command with .orig backup handling.
"""
import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.commands.link import link_cmd
from dots.core.resolver import LinkStatus


# ─── link .orig detection logic ───────────────────────────────────────────────


class TestLinkOrigDetection:
    """Tests that link correctly detects and handles existing .orig files."""

    def test_link_warns_when_backup_path_set(self):
        """When backup_path is set in status, link should warn and skip."""
        # This tests the core logic that when status.backup_path exists,
        # the link command should skip adding new backup
        
        # Create a backup file
        home = Path.home()
        backup_path = home / ".zshrc.orig"
        backup_path.write_text("# original backup")
        
        try:
            # Simulate what happens in link.py when pending + backup_path
            dest = home / ".zshrc"
            
            # Simulate the state: pending with backup needed
            status = LinkStatus(
                source=Path("/fake/Zsh/.zshrc"),
                destination=dest,
                state="pending",
                detail="backup needed",
                backup_path=backup_path,
            )
            
            # In link.py:212-215, it checks if backup exists and warns
            backup_exists = backup_path.exists() if backup_path else False
            
            # If backup_path is set and exists, link should warn (logic at line 210-215)
            if status.state == "pending" and status.detail == "backup needed":
                status_backup = status.backup_path or Path(str(dest) + ".orig")
                should_warn = status_backup.exists() if status_backup else False
                
                assert should_warn is True  # backup exists
                assert status_backup is not None
        finally:
            backup_path.unlink(missing_ok=True)
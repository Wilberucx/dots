"""
Transactional file operations with rollback support.
"""
from dataclasses import dataclass
from pathlib import Path
from typing import Literal, Optional
import shutil


ActionType = Literal["symlink", "backup", "mkdir", "unlink"]


@dataclass
class LinkAction:
    """Represents a single file system operation."""
    type: ActionType
    path: Path
    target: Optional[Path] = None
    backup_path: Optional[Path] = None


class TransactionLog:
    """
    Records file system operations for potential rollback.
    
    Usage:
        log = TransactionLog()
        try:
            log.symlink(dest, src)
            log.backup(dest)
            log.commit()
        except Exception:
            log.rollback()
            raise
    """
    
    def __init__(self):
        self.actions: list[LinkAction] = []
        self.committed = False
    
    def symlink(self, path: Path, target: Path):
        """Create a symlink and record it."""
        path.symlink_to(target)
        self.actions.append(LinkAction(
            type="symlink",
            path=path,
            target=target
        ))
    
    def backup(self, path: Path, backup_path: Path):
        """Move a file to backup and record it."""
        shutil.move(str(path), str(backup_path))
        self.actions.append(LinkAction(
            type="backup",
            path=path,
            backup_path=backup_path
        ))
    
    def mkdir(self, path: Path):
        """Create a directory and record it."""
        path.mkdir(parents=True, exist_ok=True)
        self.actions.append(LinkAction(
            type="mkdir",
            path=path
        ))
    
    def unlink(self, path: Path):
        """Remove a symlink and record it."""
        target = path.readlink() if path.is_symlink() else None
        path.unlink()
        self.actions.append(LinkAction(
            type="unlink",
            path=path,
            target=target
        ))
    
    def commit(self):
        """Mark transaction as successful (no rollback needed)."""
        self.committed = True
    
    def rollback(self):
        """Undo all recorded operations in reverse order."""
        if self.committed:
            return
        
        for action in reversed(self.actions):
            try:
                if action.type == "symlink":
                    # Remove the symlink we created
                    if action.path.is_symlink():
                        action.path.unlink()
                
                elif action.type == "backup":
                    # Restore the backup
                    if action.backup_path and action.backup_path.exists():
                        shutil.move(str(action.backup_path), str(action.path))
                
                elif action.type == "mkdir":
                    # Remove directory if empty
                    if action.path.exists() and action.path.is_dir():
                        try:
                            action.path.rmdir()
                        except OSError:
                            # Directory not empty, leave it
                            pass
                
                elif action.type == "unlink":
                    # Restore the symlink we removed
                    if action.target and not action.path.exists():
                        action.path.symlink_to(action.target)
            
            except Exception:
                # Best-effort rollback, continue even if one step fails
                pass

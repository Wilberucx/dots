import tempfile
from pathlib import Path
from dots.core.transaction import TransactionLog

def test_symlink_rollback():
    with tempfile.TemporaryDirectory() as tmp:
        src = Path(tmp) / "source.txt"
        src.write_text("hello")
        dest = Path(tmp) / "link.txt"
        
        log = TransactionLog()
        log.symlink(dest, src)
        assert dest.is_symlink()
        
        log.rollback()
        assert not dest.exists()

def test_unlink_rollback():
    with tempfile.TemporaryDirectory() as tmp:
        src = Path(tmp) / "source.txt"
        src.write_text("hello")
        dest = Path(tmp) / "link.txt"
        dest.symlink_to(src)
        
        log = TransactionLog()
        log.unlink(dest)
        assert not dest.exists()
        
        log.rollback()
        assert dest.is_symlink()
        assert dest.resolve() == src.resolve()

def test_commit_prevents_rollback():
    with tempfile.TemporaryDirectory() as tmp:
        src = Path(tmp) / "source.txt"
        src.write_text("hello")
        dest = Path(tmp) / "link.txt"
        
        log = TransactionLog()
        log.symlink(dest, src)
        log.commit()
        log.rollback()  # Should be a no-op
        
        assert dest.is_symlink()  # Still exists

def test_backup_rollback(tmp_path):
    original = Path(tmp_path) / "config.txt"
    original.write_text("original content")
    backup = Path(tmp_path) / "config.txt-backup"
    
    log = TransactionLog()
    log.backup(original, backup)
    assert not original.exists()
    assert backup.exists()
    
    log.rollback()
    assert original.exists()
    assert original.read_text() == "original content"
    assert not backup.exists()


def test_backup_already_deleted(tmp_path):
    """TOCTOU: file deleted manually before calling backup()."""
    source = tmp_path / "config"
    source.write_text("original content")
    backup = tmp_path / "config.bak"
    
    # Simulate manual deletion - file no longer exists
    source.unlink()
    assert not source.exists()
    
    log = TransactionLog()
    # Should not raise FileNotFoundError
    log.backup(source, backup)


def test_unlink_already_deleted(tmp_path):
    """TOCTOU: symlink deleted manually before calling unlink()."""
    target = tmp_path / "target"
    target.write_text("hello")
    symlink = tmp_path / "ghost_link"
    symlink.symlink_to(target)
    
    # Simulate manual deletion - symlink no longer exists
    symlink.unlink()
    assert not symlink.exists()
    
    log = TransactionLog()
    # Should not raise FileNotFoundError
    log.unlink(symlink)


def test_unlink_broken_symlink(tmp_path):
    """Broken symlink: points to a target that doesn't exist."""
    target = tmp_path / "nonexistent_target"
    symlink = tmp_path / "broken_link"
    symlink.symlink_to(target)  # target doesn't exist - broken symlink
    
    # is_symlink() returns True for broken symlinks
    assert symlink.is_symlink()
    
    log = TransactionLog()
    log.unlink(symlink)
    assert not symlink.exists()

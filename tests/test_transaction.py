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

def test_backup_rollback():
    with tempfile.TemporaryDirectory() as tmp:
        original = Path(tmp) / "config.txt"
        original.write_text("original content")
        backup = Path(tmp) / "config.txt-backup"
        
        log = TransactionLog()
        log.backup(original, backup)
        assert not original.exists()
        assert backup.exists()
        
        log.rollback()
        assert original.exists()
        assert original.read_text() == "original content"
        assert not backup.exists()

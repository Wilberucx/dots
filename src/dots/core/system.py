import platform
import sys
from pathlib import Path
from typing import Optional

def detect_os() -> str:
    """
    Detect the current operating system.
    Returns: 'linux', 'mac', 'windows', or 'unknown'
    """
    system = platform.system().lower()
    if system == "linux":
        return "linux"
    elif system == "darwin":
        return "mac"
    elif system == "windows" or "mingw" in system or "cygwin" in system:
        return "windows"
    return "unknown"

def get_home_dir() -> Path:
    """
    Get the user's home directory.
    """
    return Path.home()

def is_safe_path(path: Path) -> bool:
    """
    Check if a path is safe to modify (inside HOME).
    """
    try:
        home = get_home_dir().resolve()
        target = path.resolve()
        
        # Must be inside home
        if not str(target).startswith(str(home)):
            return False
            
        # Must not be home itself
        if target == home:
            return False
            
        return True
    except Exception:
        return False

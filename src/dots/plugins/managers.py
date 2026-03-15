"""
Package manager adapters for cross-platform dependency installation.
"""
import shutil
import subprocess
from abc import ABC, abstractmethod
from typing import List, Optional
from rich.console import Console


class PackageManager(ABC):
    """Abstract base class for package managers."""
    
    @property
    @abstractmethod
    def name(self) -> str:
        """Human-readable name of the package manager."""
        pass
    
    @property
    @abstractmethod
    def needs_sudo(self) -> bool:
        """Whether this package manager requires sudo for installation."""
        pass
    
    @abstractmethod
    def is_available(self) -> bool:
        """Check if this package manager is available on the system."""
        pass

    @abstractmethod
    def install_command(self, packages: List[str]) -> List[str]:
        """
        Return the command to install packages as a list of arguments.
        
        Returns:
            List of command arguments (e.g., ["pacman", "-S", "--noconfirm", "pkg1", "pkg2"])
        """
        pass

    @abstractmethod
    def update_command(self) -> List[str]:
        """Return the command to update package lists."""
        pass
    
    def is_package_available(self, package: str) -> bool:
        """
        Check if a package is available in this manager's repositories.
        
        Default implementation returns True. Subclasses can override for validation.
        """
        return True


class Pacman(PackageManager):
    @property
    def name(self) -> str:
        return "pacman"
    
    @property
    def needs_sudo(self) -> bool:
        return True
    
    def is_available(self) -> bool:
        return shutil.which("pacman") is not None

    def install_command(self, packages: List[str]) -> List[str]:
        return ["pacman", "-S", "--noconfirm"] + packages
        
    def update_command(self) -> List[str]:
        return ["pacman", "-Sy"]


class Apt(PackageManager):
    @property
    def name(self) -> str:
        return "apt"
    
    @property
    def needs_sudo(self) -> bool:
        return True
    
    def is_available(self) -> bool:
        return shutil.which("apt-get") is not None

    def install_command(self, packages: List[str]) -> List[str]:
        return ["apt-get", "install", "-y"] + packages
        
    def update_command(self) -> List[str]:
        return ["apt-get", "update"]


class Brew(PackageManager):
    @property
    def name(self) -> str:
        return "brew"
    
    @property
    def needs_sudo(self) -> bool:
        return False
    
    def is_available(self) -> bool:
        return shutil.which("brew") is not None

    def install_command(self, packages: List[str]) -> List[str]:
        return ["brew", "install"] + packages
        
    def update_command(self) -> List[str]:
        return ["brew", "update"]


def get_package_manager() -> Optional[PackageManager]:
    """
    Detect and return the available package manager.
    
    Priority: Pacman > Apt > Brew
    """
    if shutil.which("pacman"):
        return Pacman()
    if shutil.which("apt-get"):
        return Apt()
    if shutil.which("brew"):
        return Brew()
    return None

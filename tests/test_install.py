"""
Tests for install command logic.

Strategy: mock subprocess, shutil.which, get_package_manager and DotsConfig
to test behavior without touching the real system.
"""
import pytest
from unittest.mock import patch, MagicMock, call
from pathlib import Path
from dots.commands.install import (
    install_git_dep,
    install_binary_dep,
    install_package_dep,
)
from dots.core.yaml_parser import Dependency


# ─── Fixtures ────────────────────────────────────────────────────────────────

@pytest.fixture
def mock_manager():
    """Fake package manager: pacman-like, needs sudo."""
    manager = MagicMock()
    manager.name = "pacman"
    manager.needs_sudo = True
    manager.install_command.side_effect = lambda pkgs: ["pacman", "-S", "--noconfirm"] + pkgs
    return manager


@pytest.fixture
def git_dep():
    return Dependency(
        name="powerlevel10k",
        type="git",
        source="https://github.com/romkatv/powerlevel10k.git",
        target="~/.local/share/zsh/plugins/powerlevel10k",
    )


@pytest.fixture
def git_dep_with_ref():
    return Dependency(
        name="powerlevel10k",
        type="git",
        source="https://github.com/romkatv/powerlevel10k.git",
        target="~/.local/share/zsh/plugins/powerlevel10k",
        ref="v1.19.0",
    )


# ─── install_git_dep ─────────────────────────────────────────────────────────

class TestInstallGitDep:

    def test_clones_when_dest_does_not_exist(self, git_dep, tmp_path):
        """Clona el repo si el destino no existe."""
        target = tmp_path / "powerlevel10k"
        dep = Dependency(
            name=git_dep.name,
            type=git_dep.type,
            source=git_dep.source,
            target=str(target),
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=False)
            mock_run.assert_called_once()
            args = mock_run.call_args[0][0]
            assert args[0] == "git"
            assert args[1] == "clone"
            assert args[-1] == str(target)

    def test_skips_when_dest_exists(self, tmp_path):
        """Skipea silenciosamente si el destino ya existe."""
        target = tmp_path / "powerlevel10k"
        target.mkdir()
        dep = Dependency(
            name="powerlevel10k",
            type="git",
            source="https://github.com/romkatv/powerlevel10k.git",
            target=str(target),
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=False)
            mock_run.assert_not_called()

    def test_checkout_ref_after_clone(self, tmp_path):
        """Si ref está definido, hace checkout después del clone."""
        target = tmp_path / "powerlevel10k"
        dep = Dependency(
            name="powerlevel10k",
            type="git",
            source="https://github.com/romkatv/powerlevel10k.git",
            target=str(target),
            ref="v1.19.0",
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=False)
            assert mock_run.call_count == 2
            checkout_call = mock_run.call_args_list[1][0][0]
            assert "checkout" in checkout_call
            assert "v1.19.0" in checkout_call

    def test_no_checkout_when_ref_is_none(self, tmp_path):
        """Sin ref, solo hace clone — sin checkout."""
        target = tmp_path / "powerlevel10k"
        dep = Dependency(
            name="powerlevel10k",
            type="git",
            source="https://github.com/romkatv/powerlevel10k.git",
            target=str(target),
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=False)
            assert mock_run.call_count == 1
            assert "checkout" not in mock_run.call_args[0][0]

    def test_dry_run_does_not_call_subprocess(self, tmp_path):
        """dry_run=True no ejecuta ningún subprocess."""
        target = tmp_path / "powerlevel10k"
        dep = Dependency(
            name="powerlevel10k",
            type="git",
            source="https://github.com/romkatv/powerlevel10k.git",
            target=str(target),
            ref="v1.19.0",
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=True)
            mock_run.assert_not_called()

    def test_warns_when_source_or_target_missing(self):
        """Skipea con warning si falta source o target."""
        dep = Dependency(name="something", type="git")
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_git_dep(dep, dry_run=False)
            mock_run.assert_not_called()


# ─── install_package_dep ─────────────────────────────────────────────────────

class TestInstallPackageDep:

    def test_uses_dep_name_when_no_package_managers(self, mock_manager):
        """Sin package-managers, usa dep.name directamente."""
        dep = Dependency(name="zsh", type="package")
        with patch("dots.commands.install.shutil.which", return_value=None), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            mock_manager.install_command.assert_called_once_with(["zsh"])
            mock_run.assert_called_once()

    def test_uses_mapped_name_when_package_managers_present(self, mock_manager):
        """Con package-managers, usa el nombre mapeado para el manager actual."""
        dep = Dependency(
            name="rg",
            type="package",
            package_managers={"pacman": "ripgrep", "apt": "ripgrep"},
        )
        with patch("dots.commands.install.shutil.which", return_value=None), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            mock_manager.install_command.assert_called_once_with(["ripgrep"])

    def test_skips_when_manager_not_in_package_managers(self, mock_manager):
        """Skip con warning si el manager actual no está en package-managers."""
        dep = Dependency(
            name="eza",
            type="package",
            package_managers={"brew": "eza"},  # pacman ausente
        )
        with patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            mock_run.assert_not_called()

    def test_skips_when_already_installed(self, mock_manager):
        """Skip silencioso si shutil.which encuentra el binario."""
        dep = Dependency(name="zsh", type="package")
        with patch("dots.commands.install.shutil.which", return_value="/usr/bin/zsh"), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            mock_run.assert_not_called()

    def test_dry_run_does_not_call_subprocess(self, mock_manager):
        """dry_run=True no ejecuta ningún subprocess."""
        dep = Dependency(name="zsh", type="package")
        with patch("dots.commands.install.shutil.which", return_value=None), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=True)
            mock_run.assert_not_called()

    def test_prepends_sudo_when_manager_needs_it(self, mock_manager):
        """Si el manager necesita sudo, el comando lo incluye."""
        dep = Dependency(name="zsh", type="package")
        mock_manager.needs_sudo = True
        with patch("dots.commands.install.shutil.which", return_value=None), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            actual_cmd = mock_run.call_args[0][0]
            assert actual_cmd[0] == "sudo"

    def test_no_sudo_when_manager_does_not_need_it(self, mock_manager):
        """Si el manager no necesita sudo, no lo incluye."""
        dep = Dependency(name="fzf", type="package")
        mock_manager.needs_sudo = False
        mock_manager.install_command.side_effect = lambda pkgs: ["brew", "install"] + pkgs
        with patch("dots.commands.install.shutil.which", return_value=None), \
             patch("dots.commands.install.subprocess.run") as mock_run:
            install_package_dep(dep, mock_manager, dry_run=False)
            actual_cmd = mock_run.call_args[0][0]
            assert actual_cmd[0] != "sudo"

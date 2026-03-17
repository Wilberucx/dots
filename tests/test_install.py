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

# ─── install_binary_dep ──────────────────────────────────────────────────────

class TestInstallBinaryDep:

    def test_skips_when_dest_exists(self, tmp_path):
        """Caso 1: Skip silencioso si el destino ya existe."""
        target = tmp_path / "mybin"
        target.touch()
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin",
            target=str(target),
        )
        with patch("dots.commands.install.requests.get") as mock_get:
            install_binary_dep(dep, dry_run=False)
            mock_get.assert_not_called()

    def test_dry_run_no_side_effects(self, tmp_path):
        """Caso 2: dry_run=True no ejecuta requests ni toca filesystem."""
        target = tmp_path / "mybin"
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin",
            target=str(target),
        )
        with patch("dots.commands.install.requests.get") as mock_get:
            install_binary_dep(dep, dry_run=True)
            mock_get.assert_not_called()
            assert not target.exists()

    def test_template_arch_resolution(self, tmp_path):
        """Caso 3: {{arch}} se reemplaza por el arch detectado."""
        target = tmp_path / "mybin"
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin-{{arch}}",
            target=str(target),
        )
        mock_response = MagicMock()
        mock_response.iter_content.return_value = [b"data"]
        mock_response.raise_for_status = MagicMock()
        with patch("dots.commands.install.get_system_arch", return_value="x86_64"), \
             patch("dots.commands.install.requests.get", return_value=mock_response) as mock_get, \
             patch("dots.commands.install.shutil.move"):
            install_binary_dep(dep, dry_run=False)
            mock_get.assert_called_once()
            assert "x86_64" in mock_get.call_args[0][0]

    def test_arch_map_overrides_template(self, tmp_path):
        """Caso 4: arch_map sobreescribe {{arch}}."""
        target = tmp_path / "mybin"
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin-{{arch}}",
            target=str(target),
            arch_map={"x86_64": "amd64"},
        )
        mock_response = MagicMock()
        mock_response.iter_content.return_value = [b"data"]
        mock_response.raise_for_status = MagicMock()
        with patch("dots.commands.install.get_system_arch", return_value="x86_64"), \
             patch("dots.commands.install.requests.get", return_value=mock_response) as mock_get, \
             patch("dots.commands.install.shutil.move"):
            install_binary_dep(dep, dry_run=False)
            mock_get.assert_called_once()
            url_called = mock_get.call_args[0][0]
            assert "amd64" in url_called
            assert "x86_64" not in url_called

    def test_template_version_resolution(self, tmp_path):
        """Caso 5: {{version}} se reemplaza cuando dep.version está presente."""
        target = tmp_path / "mybin"
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin-{{version}}",
            target=str(target),
            version="1.2.3",
        )
        mock_response = MagicMock()
        mock_response.iter_content.return_value = [b"data"]
        mock_response.raise_for_status = MagicMock()
        with patch("dots.commands.install.requests.get", return_value=mock_response) as mock_get, \
             patch("dots.commands.install.shutil.move"):
            install_binary_dep(dep, dry_run=False)
            mock_get.assert_called_once()
            assert "1.2.3" in mock_get.call_args[0][0]

    def test_raw_binary_moved_and_chmod(self, tmp_path):
        """Caso 6: Binario raw se mueve al destino y se le da chmod 755."""
        target = tmp_path / "mybin"
        dep = Dependency(
            name="mybin",
            type="binary",
            source="https://example.com/mybin",
            target=str(target),
        )
        mock_response = MagicMock()
        mock_response.iter_content.return_value = [b"fake_binary_data"]
        mock_response.raise_for_status = MagicMock()

        with patch("dots.commands.install.requests.get", return_value=mock_response), \
             patch("dots.commands.install.shutil.move") as mock_move, \
             patch("dots.commands.install.Path.chmod") as mock_chmod:
            install_binary_dep(dep, dry_run=False)
            mock_move.assert_called_once()
            # arg 0 es el src temp_path, arg 1 es el destino
            assert mock_move.call_args[0][1] == target
            mock_chmod.assert_called_once_with(0o755)

    def test_warns_when_source_or_target_missing(self):
        """Caso 7: Falla (warns) si source o target están ausentes."""
        dep = Dependency(name="mybin", type="binary")
        with patch("dots.commands.install.requests.get") as mock_get:
            install_binary_dep(dep, dry_run=False)
            mock_get.assert_not_called()

    def test_extract_path_extracts_specific_member(self, tmp_path):
        """Con extract_path definido, extrae solo ese miembro del tarball."""
        import tarfile
        import io

        # Crear un tarball sintético en memoria con estructura típica de release
        tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=tar_buffer, mode="w:gz") as tar:
            # Simular: eza-linux-x86_64/eza
            binary_data = b"#!/bin/sh\necho eza"
            info = tarfile.TarInfo(name="eza-linux-x86_64/eza")
            info.size = len(binary_data)
            info.mode = 0o755
            tar.addfile(info, io.BytesIO(binary_data))
            # Archivo extra que NO debe extraerse
            readme_data = b"readme content"
            info2 = tarfile.TarInfo(name="eza-linux-x86_64/README.md")
            info2.size = len(readme_data)
            tar.addfile(info2, io.BytesIO(readme_data))
        tar_buffer.seek(0)

        dest = tmp_path / "eza"
        dep = Dependency(
            name="eza",
            type="binary",
            source="https://example.com/eza-{{arch}}.tar.gz",
            target=str(dest),
            extract_path="eza-linux-x86_64/eza",
        )

        mock_response = MagicMock()
        mock_response.iter_content.return_value = [tar_buffer.read()]
        mock_response.raise_for_status = MagicMock()

        with patch("dots.commands.install.requests.get", return_value=mock_response), \
             patch("dots.commands.install.get_system_arch", return_value="x86_64"), \
             patch("dots.commands.install.tempfile.NamedTemporaryFile") as mock_tmp:
            # Escribir el tarball real en un archivo temporal real para que tarfile lo abra
            real_tmp = tmp_path / "download.tar.gz"
            tar_buffer.seek(0)
            real_tmp.write_bytes(tar_buffer.read())
            mock_tmp.return_value.__enter__.return_value.name = str(real_tmp)

            install_binary_dep(dep, dry_run=False)

        assert dest.exists()
        assert dest.read_bytes() == b"#!/bin/sh\necho eza"


    def test_extract_path_errors_when_member_not_found(self, tmp_path):
        """Si extract_path no existe en el tarball, reporta error sin explotar."""
        import tarfile
        import io

        tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=tar_buffer, mode="w:gz") as tar:
            data = b"binary"
            info = tarfile.TarInfo(name="other-dir/other-binary")
            info.size = len(data)
            tar.addfile(info, io.BytesIO(data))
        tar_buffer.seek(0)

        dest = tmp_path / "mytool"
        dep = Dependency(
            name="mytool",
            type="binary",
            source="https://example.com/mytool.tar.gz",
            target=str(dest),
            extract_path="wrong-dir/mytool",  # no existe en el tarball
        )

        mock_response = MagicMock()
        mock_response.iter_content.return_value = [tar_buffer.read()]
        mock_response.raise_for_status = MagicMock()

        with patch("dots.commands.install.requests.get", return_value=mock_response), \
             patch("dots.commands.install.get_system_arch", return_value="x86_64"), \
             patch("dots.commands.install.tempfile.NamedTemporaryFile") as mock_tmp:
            real_tmp = tmp_path / "download.tar.gz"
            tar_buffer.seek(0)
            real_tmp.write_bytes(tar_buffer.read())
            mock_tmp.return_value.__enter__.return_value.name = str(real_tmp)

            install_binary_dep(dep, dry_run=False)

        # El destino no debe haberse creado
        assert not dest.exists()

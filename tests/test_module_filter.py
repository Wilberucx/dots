"""
Tests for --module filter behavior across commands.
Focuses on get_module_dirs filtering logic in DotsConfig.
"""
import pytest
from pathlib import Path
from unittest.mock import patch
from dots.core.config import DotsConfig


@pytest.fixture
def mock_config(tmp_path):
    """Config apuntando a un repo falso con módulos de prueba."""
    # Crear estructura mínima
    (tmp_path / "dots.toml").touch()
    for mod in ["Zsh", "Nvim", "Git", "Fish"]:
        mod_dir = tmp_path / mod
        mod_dir.mkdir()
        (mod_dir / "path.yaml").touch()

    return DotsConfig(
        repo_root=tmp_path,
        current_os="linux",
        home_dir=Path.home(),
        cli_dir=tmp_path / "cli",
    )


def test_get_module_dirs_returns_all_when_no_filter(mock_config):
    """Sin filtro, retorna todos los módulos."""
    dirs = mock_config.get_module_dirs()
    names = [d.name for d in dirs]
    assert set(names) == {"Zsh", "Nvim", "Git", "Fish"}


def test_get_module_dirs_filters_by_name(mock_config):
    """Con filtro, retorna solo los módulos solicitados."""
    dirs = mock_config.get_module_dirs(modules=["Zsh", "Nvim"])
    names = [d.name for d in dirs]
    assert set(names) == {"Zsh", "Nvim"}
    assert len(names) == 2


def test_get_module_dirs_warns_unknown_module(mock_config):
    """Módulo inexistente genera warning pero no falla."""
    with patch("dots.ui.output.print_warning") as mock_warn:
        dirs = mock_config.get_module_dirs(modules=["Zsh", "NonExistent"])
        names = [d.name for d in dirs]
        assert names == ["Zsh"]
        mock_warn.assert_called_once()
        assert "NonExistent" in mock_warn.call_args[0][0]


def test_get_module_dirs_empty_filter_returns_all(mock_config):
    """Lista vacía equivale a sin filtro."""
    dirs_no_filter = mock_config.get_module_dirs()
    dirs_empty = mock_config.get_module_dirs(modules=[])
    assert [d.name for d in dirs_no_filter] == [d.name for d in dirs_empty]


def test_get_module_dirs_single_module(mock_config):
    """Un solo módulo retorna lista de uno."""
    dirs = mock_config.get_module_dirs(modules=["Git"])
    assert len(dirs) == 1
    assert dirs[0].name == "Git"

class TestTypeFilter:

    def test_type_filter_returns_matching_modules(self, tmp_path):
        """Solo retorna módulos cuyo path.yaml tiene el type solicitado."""
        (tmp_path / "dots.toml").touch()

        # Módulo con type: minimal
        zsh = tmp_path / "Zsh"
        zsh.mkdir()
        (zsh / "path.yaml").write_text("type: minimal\nfiles: []\n")

        # Módulo con type: work
        nvim = tmp_path / "Nvim"
        nvim.mkdir()
        (nvim / "path.yaml").write_text("type: work\nfiles: []\n")

        # Módulo sin type
        git = tmp_path / "Git"
        git.mkdir()
        (git / "path.yaml").write_text("files: []\n")

        config = DotsConfig(
            repo_root=tmp_path,
            current_os="linux",
            home_dir=Path.home(),
            cli_dir=tmp_path / "cli",
        )

        result = config.get_module_dirs(types=["minimal"])
        names = [d.name for d in result]
        assert names == ["Zsh"]

    def test_type_filter_excludes_modules_without_type(self, tmp_path):
        """Módulo sin campo type queda excluido cuando se filtra por tipo."""
        (tmp_path / "dots.toml").touch()

        mod = tmp_path / "Git"
        mod.mkdir()
        (mod / "path.yaml").write_text("files: []\n")

        config = DotsConfig(
            repo_root=tmp_path,
            current_os="linux",
            home_dir=Path.home(),
            cli_dir=tmp_path / "cli",
        )

        result = config.get_module_dirs(types=["minimal"])
        assert result == []

    def test_type_filter_multiple_types(self, tmp_path):
        """Múltiples tipos actúan como OR — retorna módulos de cualquiera."""
        (tmp_path / "dots.toml").touch()

        for name, t in [("Zsh", "minimal"), ("Nvim", "work"), ("Git", "gaming")]:
            d = tmp_path / name
            d.mkdir()
            (d / "path.yaml").write_text(f"type: {t}\nfiles: []\n")

        config = DotsConfig(
            repo_root=tmp_path,
            current_os="linux",
            home_dir=Path.home(),
            cli_dir=tmp_path / "cli",
        )

        result = config.get_module_dirs(types=["minimal", "work"])
        names = {d.name for d in result}
        assert names == {"Zsh", "Nvim"}

    def test_module_and_type_filters_combine_as_and(self, tmp_path):
        """--module y --type combinados: AND — solo módulos que cumplen ambos."""
        (tmp_path / "dots.toml").touch()

        for name, t in [("Zsh", "minimal"), ("Nvim", "minimal"), ("Git", "work")]:
            d = tmp_path / name
            d.mkdir()
            (d / "path.yaml").write_text(f"type: {t}\nfiles: []\n")

        config = DotsConfig(
            repo_root=tmp_path,
            current_os="linux",
            home_dir=Path.home(),
            cli_dir=tmp_path / "cli",
        )

        # Pide Zsh y Nvim por nombre, pero solo type:work → ninguno pasa
        result = config.get_module_dirs(modules=["Zsh", "Nvim"], types=["work"])
        assert result == []

    def test_no_type_filter_returns_all(self, tmp_path):
        """Sin filtro de tipo, retorna todos independiente de si tienen type."""
        (tmp_path / "dots.toml").touch()

        (tmp_path / "Zsh").mkdir()
        (tmp_path / "Zsh" / "path.yaml").write_text("type: minimal\nfiles: []\n")
        (tmp_path / "Git").mkdir()
        (tmp_path / "Git" / "path.yaml").write_text("files: []\n")

        config = DotsConfig(
            repo_root=tmp_path,
            current_os="linux",
            home_dir=Path.home(),
            cli_dir=tmp_path / "cli",
        )

        result = config.get_module_dirs()
        names = {d.name for d in result}
        assert names == {"Zsh", "Git"}

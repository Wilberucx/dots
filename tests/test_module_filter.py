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

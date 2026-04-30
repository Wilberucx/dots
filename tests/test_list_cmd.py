import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.commands.list import list_cmd
from dots.core.resolver import LinkStatus

@pytest.fixture
def mock_config(tmp_path):
    (tmp_path / "dots.toml").touch()
    config = MagicMock()
    config.repo_root = tmp_path
    config.home_dir = Path.home()
    config.current_os = "linux"
    return config

@pytest.fixture
def sample_modules(tmp_path):
    zsh_src = tmp_path / "Zsh" / ".zshrc"
    zsh_src.parent.mkdir(parents=True, exist_ok=True)
    zsh_src.touch()

    nvim_src = tmp_path / "Nvim" / "init.lua"
    nvim_src.parent.mkdir(parents=True, exist_ok=True)
    nvim_src.touch()

    broken_src = tmp_path / "Broken" / "config"
    broken_src.parent.mkdir(parents=True, exist_ok=True)
    broken_src.touch()

    return {
        "Zsh": [
            LinkStatus(
                source=zsh_src,
                destination=Path.home() / ".zshrc",
                state="linked",
            ),
        ],
        "Nvim": [
            LinkStatus(
                source=nvim_src,
                destination=Path.home() / ".config" / "nvim" / "init.lua",
                state="pending",
            ),
        ],
        "Broken": [
            LinkStatus(
                source=broken_src,
                destination=Path.home() / ".broken",
                state="conflict",
                detail="points to somewhere else"
            )
        ]
    }

def test_list_all_modules(mock_config, sample_modules):
    with patch("dots.commands.list.DotsConfig.load", return_value=mock_config), \
         patch("dots.commands.list.resolve_modules", return_value=sample_modules), \
         patch("dots.commands.list.console.print") as mock_print:

        list_cmd(linked=False, unlinked=False, broken=False, variant=False, bak=False)

        printed = [call.args[0] for call in mock_print.call_args_list]
        assert "Zsh" in printed
        assert "Nvim" in printed
        assert "Broken" in printed

def test_list_linked_only(mock_config, sample_modules):
    with patch("dots.commands.list.DotsConfig.load", return_value=mock_config), \
         patch("dots.commands.list.resolve_modules", return_value=sample_modules), \
         patch("dots.commands.list.console.print") as mock_print:

        list_cmd(linked=True, unlinked=False, broken=False, variant=False, bak=False)

        printed = [call.args[0] for call in mock_print.call_args_list]
        assert "Zsh" in printed
        assert "Nvim" not in printed
        assert "Broken" not in printed

def test_list_unlinked_only(mock_config, sample_modules):
    with patch("dots.commands.list.DotsConfig.load", return_value=mock_config), \
         patch("dots.commands.list.resolve_modules", return_value=sample_modules), \
         patch("dots.commands.list.console.print") as mock_print:

        list_cmd(linked=False, unlinked=True, broken=False, variant=False, bak=False)

        printed = [call.args[0] for call in mock_print.call_args_list]
        assert "Zsh" not in printed
        assert "Nvim" in printed
        assert "Broken" not in printed

def test_list_broken_only(mock_config, sample_modules):
    with patch("dots.commands.list.DotsConfig.load", return_value=mock_config), \
         patch("dots.commands.list.resolve_modules", return_value=sample_modules), \
         patch("dots.commands.list.console.print") as mock_print:

        list_cmd(linked=False, unlinked=False, broken=True, variant=False, bak=False)

        printed = [call.args[0] for call in mock_print.call_args_list]
        assert "Zsh" not in printed
        assert "Nvim" not in printed
        assert "Broken" in printed

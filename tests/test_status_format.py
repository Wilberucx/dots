"""
Tests for --format flag in status command.
Covers _render_json and _render_table output correctness.
Tests use mocked resolve_modules and DotsConfig to avoid
touching the real filesystem.
"""
import json
import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.commands.status import _render_json, _render_table, OutputFormat
from dots.core.resolver import LinkStatus


# ─── Fixtures ────────────────────────────────────────────────────────────────


@pytest.fixture
def mock_config(tmp_path):
    """Minimal DotsConfig pointing to a fake repo."""
    (tmp_path / "dots.toml").touch()

    # Create a minimal module with path.yaml that has type
    zsh = tmp_path / "Zsh"
    zsh.mkdir()
    (zsh / "path.yaml").write_text("type: minimal\nfiles: []\n")

    nvim = tmp_path / "Nvim"
    nvim.mkdir()
    (nvim / "path.yaml").write_text("files: []\n")

    config = MagicMock()
    config.repo_root = tmp_path
    config.home_dir = Path.home()
    return config


@pytest.fixture
def sample_modules(tmp_path):
    """Sample resolve_modules output with mixed states."""
    zsh_src = tmp_path / "Zsh" / ".zshrc"
    zsh_src.parent.mkdir(parents=True, exist_ok=True)
    zsh_src.touch()

    nvim_src = tmp_path / "Nvim" / "init.lua"
    nvim_src.parent.mkdir(parents=True, exist_ok=True)
    nvim_src.touch()

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
    }


# ─── _render_json ─────────────────────────────────────────────────────────────


class TestRenderJson:

    def test_json_output_is_valid(self, mock_config, sample_modules, capsys):
        """El output de _render_json es JSON válido."""
        output_lines = []

        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        raw = "\n".join(output_lines)
        parsed = json.loads(raw)
        assert isinstance(parsed, dict)

    def test_json_contains_modules_key(self, mock_config, sample_modules):
        """El JSON tiene la clave 'modules'."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        assert "modules" in parsed

    def test_json_contains_summary_key(self, mock_config, sample_modules):
        """El JSON tiene la clave 'summary'."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        assert "summary" in parsed
        assert "linked" in parsed["summary"]
        assert "unlinked" in parsed["summary"]

    def test_json_summary_counts_correctly(self, mock_config, sample_modules):
        """Summary cuenta correctamente linked y unlinked."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        assert parsed["summary"]["linked"] == 1
        assert parsed["summary"]["unlinked"] == 1

    def test_json_state_filter_excludes_states(self, mock_config, sample_modules):
        """state_filter excluye estados no solicitados del JSON."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            # Solo linked — pending queda fuera
            _render_json(sample_modules, {"linked"}, mock_config)

        parsed = json.loads("\n".join(output_lines))
        # Nvim tiene state pending — no debe aparecer
        assert "Nvim" not in parsed["modules"]
        assert "Zsh" in parsed["modules"]

    def test_json_module_has_type_when_declared(self, mock_config, sample_modules):
        """Módulo con type declarado en path.yaml lo incluye en el JSON."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        assert parsed["modules"]["Zsh"]["type"] == "minimal"

    def test_json_module_type_is_none_when_not_declared(self, mock_config, sample_modules):
        """Módulo sin type en path.yaml tiene type: null en el JSON."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        assert parsed["modules"]["Nvim"]["type"] is None

    def test_json_destination_uses_tilde(self, mock_config, sample_modules):
        """El destination en JSON usa ~ en lugar del home path absoluto."""
        output_lines = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda s: output_lines.append(str(s))
            _render_json(sample_modules, None, mock_config)

        parsed = json.loads("\n".join(output_lines))
        zsh_files = parsed["modules"]["Zsh"]["files"]
        assert all(f["destination"].startswith("~") for f in zsh_files)


# ─── _render_table ────────────────────────────────────────────────────────────


class TestRenderTable:

    def test_table_renders_without_error(self, mock_config, sample_modules):
        """_render_table no lanza excepciones con datos válidos."""
        with patch("dots.commands.status.console"):
            try:
                _render_table(sample_modules, None, mock_config)
            except Exception as e:
                pytest.fail(f"_render_table raised: {e}")

    def test_table_state_filter_applied(self, mock_config, sample_modules):
        """Con state_filter, solo procesa los estados incluidos."""
        calls = []
        with patch("dots.commands.status.console") as mock_console:
            mock_console.print.side_effect = lambda *a, **kw: calls.append(a)
            _render_table(sample_modules, {"linked"}, mock_config)

        # Debe haber llamado print al menos una vez (la tabla)
        assert len(calls) > 0

    def test_table_empty_after_filter_shows_warning(self, mock_config, sample_modules):
        """Si el filtro excluye todo, muestra warning."""
        with patch("dots.commands.status.console"), \
             patch("dots.commands.status.print_warning") as mock_warn:
            # "unsafe" no existe en sample_modules → total = 0
            _render_table(sample_modules, {"unsafe"}, mock_config)
            mock_warn.assert_called_once()

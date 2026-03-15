import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.core.config import DotsConfig, MARKER_FILE


# ── Unit tests (no filesystem dependency) ────────────────────────────────────

def test_load_finds_marker_file(tmp_path):
    """DotsConfig.load() must find dots.toml walking up from cwd."""
    marker = tmp_path / MARKER_FILE
    marker.write_text("[dots]\nversion = \"1\"\n")

    with patch("dots.core.config.Path.cwd", return_value=tmp_path):
        config = DotsConfig.load()

    assert config.repo_root == tmp_path.resolve()
    assert config.home_dir.exists()
    assert config.current_os in ("linux", "mac", "windows", "unknown")


def test_load_finds_marker_in_parent(tmp_path):
    """DotsConfig.load() must walk up and find dots.toml in a parent dir."""
    marker = tmp_path / MARKER_FILE
    marker.write_text("[dots]\nversion = \"1\"\n")
    nested = tmp_path / "subdir" / "deeper"
    nested.mkdir(parents=True)

    with patch("dots.core.config.Path.cwd", return_value=nested):
        config = DotsConfig.load()

    assert config.repo_root == tmp_path.resolve()


def test_load_raises_when_no_marker(tmp_path):
    """DotsConfig.load() must raise RuntimeError when no dots.toml is found."""
    # tmp_path has no dots.toml and no parents that would have one
    # We root the search at a leaf path with no marker
    isolated = tmp_path / "no_marker_here"
    isolated.mkdir()

    with patch("dots.core.config.Path.cwd", return_value=isolated):
        with pytest.raises(RuntimeError, match=MARKER_FILE):
            DotsConfig.load()


def test_get_module_dirs_returns_dirs_with_path_yaml(tmp_path):
    """get_module_dirs() must only return dirs containing path.yaml."""
    (tmp_path / MARKER_FILE).write_text("[dots]\nversion = \"1\"\n")

    # Create two valid modules
    for name in ("Nvim", "Zsh"):
        d = tmp_path / name
        d.mkdir()
        (d / "path.yaml").write_text("[]")

    # Create a dir without path.yaml (should be excluded)
    (tmp_path / "random_dir").mkdir()

    with patch("dots.core.config.Path.cwd", return_value=tmp_path):
        config = DotsConfig.load()

    dirs = config.get_module_dirs()
    names = [d.name for d in dirs]
    assert "Nvim" in names
    assert "Zsh" in names
    assert "random_dir" not in names
    for d in dirs:
        assert (d / "path.yaml").exists()


# ── Integration test (requires real dotfiles repo) ───────────────────────────

def test_integration_config_loads_from_real_repo():
    """
    Integration: DotsConfig.load() from within the real dotfiles repo.
    Skips if no dots.toml is present in the environment.
    """
    import os
    from pathlib import Path

    # Walk up from cwd to find the marker
    cwd = Path.cwd()
    found = any((p / MARKER_FILE).exists() for p in [cwd] + list(cwd.parents))

    if not found:
        pytest.skip("No dots.toml found in current path hierarchy — skipping integration test")

    config = DotsConfig.load()
    assert config.repo_root.exists()
    assert config.current_os in ("linux", "mac", "windows", "unknown")
    assert config.home_dir.exists()
    for d in config.get_module_dirs():
        assert d.is_dir()
        assert (d / "path.yaml").exists()

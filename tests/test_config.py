import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock
from dots.core.config import DotsConfig, MARKER_DIR, MARKER_CONFIG, LEGACY_MARKER, is_dotfiles_repo


# ── Unit tests (no filesystem dependency) ────────────────────────────────────

def test_load_finds_new_marker(tmp_path):
    """DotsConfig.load() must find .dots/config.yaml walking up from cwd."""
    marker_dir = tmp_path / MARKER_DIR
    marker_dir.mkdir()
    marker = marker_dir / MARKER_CONFIG
    marker.write_text("[dots]\nversion = \"1\"\n")

    with patch.dict("os.environ", {}, clear=True), \
         patch("dots.core.config.Path.cwd", return_value=tmp_path), \
         patch("dots.core.config.Path.home", return_value=tmp_path / "home"):
        config = DotsConfig.load()

    assert config.repo_root == tmp_path.resolve()
    assert config.current_os in ("linux", "mac", "windows", "unknown")


def test_load_finds_legacy_marker(tmp_path):
    """DotsConfig.load() must find legacy dots.toml (backward compatibility)."""
    marker = tmp_path / LEGACY_MARKER
    marker.write_text("[dots]\nversion = \"1\"\n")

    with patch.dict("os.environ", {}, clear=True), \
         patch("dots.core.config.Path.cwd", return_value=tmp_path), \
         patch("dots.core.config.Path.home", return_value=tmp_path / "home"):
        config = DotsConfig.load()

    assert config.repo_root == tmp_path.resolve()


def test_load_finds_marker_in_parent(tmp_path):
    """DotsConfig.load() must walk up and find marker in a parent dir."""
    marker_dir = tmp_path / MARKER_DIR
    marker_dir.mkdir()
    (marker_dir / MARKER_CONFIG).write_text("[dots]\nversion = \"1\"\n")
    nested = tmp_path / "subdir" / "deeper"
    nested.mkdir(parents=True)

    with patch.dict("os.environ", {}, clear=True), \
         patch("dots.core.config.Path.cwd", return_value=nested), \
         patch("dots.core.config.Path.home", return_value=tmp_path / "home"):
        config = DotsConfig.load()

    assert config.repo_root == tmp_path.resolve()


def test_load_raises_when_no_marker(tmp_path):
    """DotsConfig.load() must raise RuntimeError when no marker is found."""
    # tmp_path has no marker and no parents that would have one
    # We root the search at a leaf path with no marker
    isolated = tmp_path / "no_marker_here"
    isolated.mkdir()

    with patch.dict("os.environ", {}, clear=True), \
         patch("dots.core.config.Path.cwd", return_value=isolated), \
         patch("dots.core.config.Path.home", return_value=isolated / "home"):
        with pytest.raises(RuntimeError, match=MARKER_DIR):
            DotsConfig.load()


def test_get_module_dirs_returns_dirs_with_path_yaml(tmp_path):
    """get_module_dirs() must only return dirs containing path.yaml."""
    marker_dir = tmp_path / MARKER_DIR
    marker_dir.mkdir()
    (marker_dir / MARKER_CONFIG).write_text("[dots]\nversion = \"1\"\n")

    # Create two valid modules
    for name in ("Nvim", "Zsh"):
        d = tmp_path / name
        d.mkdir()
        (d / "path.yaml").write_text("[]")

    # Create a dir without path.yaml (should be excluded)
    (tmp_path / "random_dir").mkdir()

    with patch.dict("os.environ", {}, clear=True), \
         patch("dots.core.config.Path.cwd", return_value=tmp_path), \
         patch("dots.core.config.Path.home", return_value=tmp_path / "home"):
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
    Skips if no marker (.dots/config.yaml or dots.toml) is present.
    """
    import os
    from pathlib import Path

    # Walk up from cwd to find the marker
    cwd = Path.cwd()
    found = any(is_dotfiles_repo(p) for p in [cwd] + list(cwd.parents))

    if not found:
        pytest.skip("No dotfiles marker found in current path hierarchy — skipping integration test")

    config = DotsConfig.load()
    assert config.repo_root.exists()
    assert config.current_os in ("linux", "mac", "windows", "unknown")
    assert config.home_dir.exists()
    for d in config.get_module_dirs():
        assert d.is_dir()
        assert (d / "path.yaml").exists()

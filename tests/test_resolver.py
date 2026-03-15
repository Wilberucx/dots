import pytest
from pathlib import Path
from dots.core.config import DotsConfig, MARKER_FILE
from dots.core.resolver import resolve_modules


def test_resolve_real_modules():
    """
    Integration: resolve_modules() against the real dotfiles repo.
    Skips if no dots.toml is present in the environment.
    """
    cwd = Path.cwd()
    found = any((p / MARKER_FILE).exists() for p in [cwd] + list(cwd.parents))

    if not found:
        pytest.skip("No dots.toml found in current path hierarchy — skipping integration test")

    config = DotsConfig.load()
    modules = resolve_modules(config)

    # We might not have modules in a clean environment, but if we do, check structure
    if modules:
        for name, statuses in modules.items():
            assert isinstance(name, str) and len(name) > 0
            for s in statuses:
                assert s.state in ("linked", "conflict", "pending", "missing", "unsafe")

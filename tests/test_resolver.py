import pytest
from pathlib import Path
from dots.core.config import DotsConfig, is_dotfiles_repo
from dots.core.resolver import resolve_modules


def test_resolve_real_modules():
    """
    Integration: resolve_modules() against the real dotfiles repo.
    Skips if no marker (.dots/config.yaml or dots.toml) is present.
    """
    cwd = Path.cwd()
    found = any(is_dotfiles_repo(p) for p in [cwd] + list(cwd.parents))

    if not found:
        pytest.skip("No dotfiles marker found in current path hierarchy — skipping integration test")

    config = DotsConfig.load()
    modules = resolve_modules(config)

    # We might not have modules in a clean environment, but if we do, check structure
    if modules:
        for name, statuses in modules.items():
            assert isinstance(name, str) and len(name) > 0
            for s in statuses:
                assert s.state in ("linked", "conflict", "pending", "missing", "unsafe")

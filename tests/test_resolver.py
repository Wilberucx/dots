import tempfile
from pathlib import Path
import yaml
from dots.core.config import DotsConfig
from dots.core.resolver import resolve_modules

def test_resolve_real_modules():
    config = DotsConfig.load()
    modules = resolve_modules(config)

    # We might not have modules in a clean environment, but if we do, check structure
    if modules:
        for name, statuses in modules.items():
            assert isinstance(name, str) and len(name) > 0
            for s in statuses:
                assert s.state in ("linked", "conflict", "pending", "missing", "unsafe")

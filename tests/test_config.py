from dots.core.config import DotsConfig

def test_config_loads():
    config = DotsConfig.load()
    assert config.repo_root.exists()
    assert config.cli_dir.exists()
    assert config.current_os in ("linux", "mac", "windows", "unknown")
    assert config.home_dir.exists()

def test_module_dirs():
    config = DotsConfig.load()
    dirs = config.get_module_dirs()
    assert isinstance(dirs, list)
    # Every returned directory must contain a path.yaml (that's the contract)
    for d in dirs:
        assert d.is_dir(), f"{d} should be a directory"
        assert (d / "path.yaml").exists(), f"{d} should contain path.yaml"

import tempfile
from pathlib import Path
import yaml
from dots.core.yaml_parser import parse_path_yaml

def test_basic_parsing():
    with tempfile.TemporaryDirectory() as tmp:
        yaml_path = Path(tmp) / "path.yaml"
        yaml_path.write_text(yaml.dump({
            "files": [
                {"source": "nvim", "destination": "~/.config/nvim"}
            ]
        }))
        
        mappings = parse_path_yaml(yaml_path, "linux")
        assert len(mappings) == 1
        assert mappings[0].source == "nvim"
        assert mappings[0].destination == "~/.config/nvim"

def test_os_filtering():
    with tempfile.TemporaryDirectory() as tmp:
        yaml_path = Path(tmp) / "path.yaml"
        yaml_path.write_text(yaml.dump({
            "files": [
                {"source": "config", "destination": "~/.config/app", "os": ["mac"]},
                {"source": "other", "destination": "~/.other"}
            ]
        }))
        
        mappings = parse_path_yaml(yaml_path, "linux")
        assert len(mappings) == 1
        assert mappings[0].source == "other"

def test_os_specific_destination():
    with tempfile.TemporaryDirectory() as tmp:
        yaml_path = Path(tmp) / "path.yaml"
        yaml_path.write_text(yaml.dump({
            "files": [
                {
                    "source": ".gitconfig",
                    "destination": "~/.gitconfig-default",
                    "destination-linux": "~/.gitconfig"
                }
            ]
        }))
        
        mappings = parse_path_yaml(yaml_path, "linux")
        assert mappings[0].destination == "~/.gitconfig"

def test_empty_yaml():
    with tempfile.TemporaryDirectory() as tmp:
        yaml_path = Path(tmp) / "path.yaml"
        yaml_path.write_text("")
        
        mappings = parse_path_yaml(yaml_path, "linux")
        assert mappings == []

def test_frozen_dataclass():
    with tempfile.TemporaryDirectory() as tmp:
        yaml_path = Path(tmp) / "path.yaml"
        yaml_path.write_text(yaml.dump({
            "files": [{"source": "x", "destination": "~/.x"}]
        }))
        
        mappings = parse_path_yaml(yaml_path, "linux")
        try:
            mappings[0].source = "modified"
            assert False, "Should raise FrozenInstanceError"
        except AttributeError:
            pass  # Expected — frozen dataclass

def test_destination_override_by_os(tmp_path):
    """destination-override.linux toma prioridad sobre destination."""
    yaml_content = """
files:
  - source: config.toml
    os: [linux, mac]
    destination: ~/.config/tool/config.toml
    destination-override:
      mac: ~/Library/Preferences/tool/config.toml
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    mappings_linux = parse_path_yaml(yaml_path, "linux")
    assert len(mappings_linux) == 1
    assert mappings_linux[0].destination == "~/.config/tool/config.toml"

    mappings_mac = parse_path_yaml(yaml_path, "mac")
    assert len(mappings_mac) == 1
    assert mappings_mac[0].destination == "~/Library/Preferences/tool/config.toml"


def test_destination_generic_fallback(tmp_path):
    """destination genérico se usa cuando no hay override ni destination-OS."""
    yaml_content = """
files:
  - source: .zshrc
    os: [linux, mac]
    destination: ~/.zshrc
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    mappings = parse_path_yaml(yaml_path, "linux")
    assert len(mappings) == 1
    assert mappings[0].destination == "~/.zshrc"


def test_dependency_ref_field(tmp_path):
    """Campo ref se parsea correctamente en dependencies."""
    from dots.core.yaml_parser import parse_dependencies
    yaml_content = """
dependencies:
  - name: powerlevel10k
    type: git
    source: https://github.com/romkatv/powerlevel10k.git
    target: ~/.local/share/zsh/plugins/powerlevel10k
    ref: v1.19.0
  - name: zsh-autosuggestions
    type: git
    source: https://github.com/zsh-users/zsh-autosuggestions.git
    target: ~/.local/share/zsh/plugins/zsh-autosuggestions
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    deps = parse_dependencies(yaml_path)
    assert len(deps) == 2

    p10k = deps[0]
    assert p10k.ref == "v1.19.0"

    autosugg = deps[1]
    assert autosugg.ref is None

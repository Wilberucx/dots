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

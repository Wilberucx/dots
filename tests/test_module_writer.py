import pytest
from pathlib import Path
from dots.core.module_writer import is_destination_declared, append_file_entry

def test_is_destination_declared_v3_generic():
    data = {"files": [{"destination": "~/test"}]}
    assert is_destination_declared(data, "~/test") is True
    assert is_destination_declared(data, "~/other") is False

def test_is_destination_declared_v3_per_os():
    data = {"files": [{"per-os": {"linux": "~/test"}}]}
    assert is_destination_declared(data, "~/test") is True
    assert is_destination_declared(data, "~/other") is False

def test_is_destination_declared_ignores_v2_fields():
    """
    Confirma que v2 fields (destination-linux, destination-mac)
    no generan falsos positivos en schema v3.
    """
    data = {"files": [
        {"source": "gitconfig", "destination-linux": "~/.gitconfig"}  # v2
    ]}
    assert is_destination_declared(data, "~/.gitconfig") is False

def test_append_file_entry(tmp_path):
    yaml_path = tmp_path / "path.yaml"
    entry = {"source": "file", "destination": "~/test"}
    
    append_file_entry(yaml_path, entry)
    
    with open(yaml_path, "r") as f:
        import yaml
        data = yaml.safe_load(f)
        assert data["files"] == [entry]

def test_append_file_entry_preserves_existing(tmp_path):
    """Append no sobreescribe entradas previas."""
    yaml_path = tmp_path / "path.yaml"
    first = {"source": "first", "destination": "~/first"}
    second = {"source": "second", "destination": "~/second"}

    append_file_entry(yaml_path, first)
    append_file_entry(yaml_path, second)

    with open(yaml_path) as f:
        import yaml
        data = yaml.safe_load(f)
    
    assert len(data["files"]) == 2
    assert data["files"][0] == first
    assert data["files"][1] == second

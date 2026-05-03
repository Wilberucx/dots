"""Tests for migrate command."""

import pytest
import yaml
from pathlib import Path
from dots.commands.migrate import (
    migrate_file,
    _migrate_dependency,
    _migrate_file,
    find_all_path_yaml,
)


class TestMigrateDependency:
    """Tests for dependency migration from v2 to v3."""

    def test_migrate_source_to_url(self):
        """Map 'source' field to 'url'."""
        dep = {"name": "bat", "source": "https://github.com/sharkdp/bat/releases/download/v0.24.0/bat-v0.24.0-x86_64-unknown-linux-gnu.tar.gz"}
        result = _migrate_dependency(dep)

        assert "url" in result
        assert result["url"] == dep["source"]
        assert "source" not in result

    def test_migrate_target_to_dest(self):
        """Map 'target' field to 'dest'."""
        dep = {"name": "bat", "target": "/usr/local/bin/bat"}
        result = _migrate_dependency(dep)

        assert "dest" in result
        assert result["dest"] == "/usr/local/bin/bat"
        assert "target" not in result

    def test_migrate_extract_path_to_extract(self):
        """Map 'extract-path' field to 'extract'."""
        dep = {"name": "bat", "extract-path": "bat-v0.24.0-x86_64-unknown-linux-gnu/bat"}
        result = _migrate_dependency(dep)

        assert "extract" in result
        assert result["extract"] == "bat-v0.24.0-x86_64-unknown-linux-gnu/bat"
        assert "extract-path" not in result

    def test_migrate_arch_map_to_arch(self):
        """Map 'arch_map' field to 'arch'."""
        dep = {
            "name": "fd",
            "arch_map": {
                "x86_64": "https://github.com/sharkdp/fd/releases/download/v8.7.0/fd-v8.7.0-x86_64-unknown-linux-gnu.tar.gz",
                "aarch64": "https://github.com/sharkdp/fd/releases/download/v8.7.0/fd-v8.7.0-aarch64-unknown-linux-gnu.tar.gz",
            }
        }
        result = _migrate_dependency(dep)

        assert "arch" in result
        assert "aarch64" in result["arch"]
        assert "arch_map" not in result

    def test_migrate_package_managers_to_managers(self):
        """Map 'package-managers' field to 'managers'."""
        dep = {
            "name": "bat",
            "package-managers": {
                "pacman": "bat",
                "apt": "bat",
            }
        }
        result = _migrate_dependency(dep)

        assert "managers" in result
        assert "pacman" in result["managers"]
        assert "package-managers" not in result

    def test_migrate_type_system_to_package(self):
        """Map 'type: system' to 'type: package'."""
        dep = {"name": "git", "type": "system"}
        result = _migrate_dependency(dep)

        assert result["type"] == "package"

    def test_preserves_existing_v3_fields(self):
        """Don't overwrite fields that are already v3."""
        dep = {
            "name": "bat",
            "url": "https://example.com/bat.tar.gz",
            "dest": "/usr/bin/bat",
        }
        result = _migrate_dependency(dep)

        assert result["url"] == "https://example.com/bat.tar.gz"
        assert result["dest"] == "/usr/bin/bat"

    def test_migrate_full_dependency_v2_to_v3(self):
        """Full migration of a v2 dependency to v3."""
        dep = {
            "name": "bat",
            "type": "system",
            "source": "https://example.com/bat.tar.gz",
            "target": "/usr/bin/bat",
            "extract-path": "bat/bat",
            "arch_map": {"x86_64": "https://x64.com", "aarch64": "https://arm64.com"},
            "package-managers": {"pacman": "bat"},
        }
        result = _migrate_dependency(dep)

        assert result["type"] == "package"
        assert result["url"] == "https://example.com/bat.tar.gz"
        assert result["dest"] == "/usr/bin/bat"
        assert result["extract"] == "bat/bat"
        assert result["arch"] == {"x86_64": "https://x64.com", "aarch64": "https://arm64.com"}
        assert result["managers"] == {"pacman": "bat"}


class TestMigrateFile:
    """Tests for file mapping migration from v2 to v3."""

    def test_migrate_destination_linux_to_per_os(self):
        """Map 'destination-linux' to 'per-os'."""
        file_entry = {"source": "config/linux.conf", "destination-linux": "/home/user/.config/app.conf"}
        result = _migrate_file(file_entry)

        assert "per-os" in result
        assert result["per-os"]["linux"] == "/home/user/.config/app.conf"
        assert "destination-linux" not in result

    def test_migrate_destination_mac_to_per_os(self):
        """Map 'destination-mac' to 'per-os'."""
        file_entry = {"source": "config/mac.conf", "destination-mac": "/Users/user/.config/app.conf"}
        result = _migrate_file(file_entry)

        assert "per-os" in result
        assert result["per-os"]["mac"] == "/Users/user/.config/app.conf"
        assert "destination-mac" not in result

    def test_migrate_destination_override_to_per_os(self):
        """Map 'destination-override' to 'per-os'."""
        file_entry = {"source": "config/app.conf", "destination-override": "/custom/path/app.conf"}
        result = _migrate_file(file_entry)

        assert "per-os" in result
        # destination-override as string applies to linux and mac
        assert result["per-os"]["linux"] == "/custom/path/app.conf"
        assert result["per-os"]["mac"] == "/custom/path/app.conf"

    def test_migrate_destination_override_dict(self):
        """Map 'destination-override' dict to 'per-os'."""
        file_entry = {
            "source": "config/app.conf",
            "destination-override": {
                "linux": "/linux/path/app.conf",
                "mac": "/mac/path/app.conf",
            }
        }
        result = _migrate_file(file_entry)

        assert "per-os" in result
        assert result["per-os"]["linux"] == "/linux/path/app.conf"
        assert result["per-os"]["mac"] == "/mac/path/app.conf"

    def test_merge_multiple_destinations_to_per_os(self):
        """Combine destination-linux, destination-mac into per-os."""
        file_entry = {
            "source": "config/app.conf",
            "destination-linux": "/linux/path/app.conf",
            "destination-mac": "/mac/path/app.conf",
        }
        result = _migrate_file(file_entry)

        assert "per-os" in result
        assert result["per-os"]["linux"] == "/linux/path/app.conf"
        assert result["per-os"]["mac"] == "/mac/path/app.conf"
        assert "destination-linux" not in result
        assert "destination-mac" not in result


class TestMigrateFileIntegration:
    """Integration tests for migrate_file function."""

    def test_migrate_full_path_yaml_v2(self, tmp_path):
        """Migrate a complete v2 path.yaml file."""
        path_yaml = tmp_path / "path.yaml"
        v2_data = {
            "dependencies": [
                {
                    "name": "bat",
                    "type": "system",
                    "source": "https://example.com/bat.tar.gz",
                    "target": "/usr/bin/bat",
                },
                {
                    "name": "git",
                    "package-managers": {"pacman": "git", "apt": "git"},
                },
            ],
            "files": [
                {
                    "source": "linux/config.conf",
                    "destination-linux": "/home/user/.config/app.conf",
                },
                {
                    "source": "mac/config.conf",
                    "destination-mac": "/Users/user/.config/app.conf",
                },
            ],
        }

        with open(path_yaml, "w") as f:
            yaml.safe_dump(v2_data, f)

        modified = migrate_file(path_yaml, dry_run=False)

        assert modified is True

        with open(path_yaml, "r") as f:
            result = yaml.safe_load(f)

        # Check dependencies
        assert result["dependencies"][0]["type"] == "package"
        assert result["dependencies"][0]["url"] == "https://example.com/bat.tar.gz"
        assert result["dependencies"][0]["dest"] == "/usr/bin/bat"
        assert result["dependencies"][1]["managers"] == {"pacman": "git", "apt": "git"}

        # Check files
        assert result["files"][0]["per-os"]["linux"] == "/home/user/.config/app.conf"
        assert result["files"][1]["per-os"]["mac"] == "/Users/user/.config/app.conf"

    def test_migrate_already_v3_no_change(self, tmp_path):
        """Files already in v3 format should not be modified."""
        path_yaml = tmp_path / "path.yaml"
        v3_data = {
            "dependencies": [
                {"name": "bat", "type": "package", "url": "https://example.com/bat.tar.gz"},
            ],
            "files": [
                {"source": "config.conf", "destination": "/home/user/.config/app.conf"},
            ],
        }

        with open(path_yaml, "w") as f:
            yaml.safe_dump(v3_data, f)

        modified = migrate_file(path_yaml, dry_run=False)

        assert modified is False

        with open(path_yaml, "r") as f:
            result = yaml.safe_load(f)

        assert result == v3_data


class TestDryRun:
    """Tests for dry-run behavior."""

    def test_dry_run_does_not_modify_file(self, tmp_path):
        """Dry-run should not modify any files."""
        path_yaml = tmp_path / "path.yaml"
        original_content = """dependencies:
  - name: bat
    type: system
    source: https://example.com/bat.tar.gz
    target: /usr/bin/bat
files:
  - source: config.conf
    destination-linux: /home/user/.config/app.conf
"""

        with open(path_yaml, "w") as f:
            f.write(original_content)

        modified = migrate_file(path_yaml, dry_run=True)

        # dry-run says it needs migration
        assert modified is True

        # But file should be unchanged
        with open(path_yaml, "r") as f:
            current_content = f.read()

        assert current_content == original_content


class TestIdempotency:
    """Tests for idempotency - running multiple times should not break."""

    def test_migrate_twice_no_change_second_time(self, tmp_path):
        """Second migration should not change anything."""
        path_yaml = tmp_path / "path.yaml"
        v2_data = {
            "dependencies": [
                {"name": "bat", "type": "system", "source": "https://example.com/bat.tar.gz"},
            ],
            "files": [
                {"source": "config.conf", "destination-linux": "/home/user/.config/app.conf"},
            ],
        }

        with open(path_yaml, "w") as f:
            yaml.safe_dump(v2_data, f)

        # First migration
        modified1 = migrate_file(path_yaml, dry_run=False)
        assert modified1 is True

        # Second migration
        modified2 = migrate_file(path_yaml, dry_run=False)
        assert modified2 is False

        # Content should still be v3
        with open(path_yaml, "r") as f:
            result = yaml.safe_load(f)

        assert result["dependencies"][0]["type"] == "package"
        assert result["dependencies"][0]["url"] == "https://example.com/bat.tar.gz"
        assert result["files"][0]["per-os"]["linux"] == "/home/user/.config/app.conf"

    def test_three_migrations_idempotent(self, tmp_path):
        """Running migration 3 times should be stable."""
        path_yaml = tmp_path / "path.yaml"
        v2_data = {
            "dependencies": [{"name": "bat", "source": "https://example.com/bat.tar.gz"}],
            "files": [{"source": "x", "destination-linux": "/linux/path"}],
        }

        with open(path_yaml, "w") as f:
            yaml.safe_dump(v2_data, f)

        for _ in range(3):
            migrate_file(path_yaml, dry_run=False)

        with open(path_yaml, "r") as f:
            result = yaml.safe_load(f)

        # Should be valid v3
        assert result["dependencies"][0]["url"] == "https://example.com/bat.tar.gz"
        assert result["files"][0]["per-os"]["linux"] == "/linux/path"


class TestFindPathYaml:
    """Tests for finding path.yaml files."""

    def test_find_path_yaml_in_subdirectories(self, tmp_path):
        """Find all path.yaml in nested directories."""
        (tmp_path / "module1").mkdir()
        (tmp_path / "module1" / "path.yaml").touch()
        (tmp_path / "module2" / "nested").mkdir(parents=True)
        (tmp_path / "module2" / "nested" / "path.yaml").touch()
        (tmp_path / "module3").mkdir()
        (tmp_path / "module3" / "path.yaml").touch()

        results = find_all_path_yaml(tmp_path)

        assert len(results) == 3

    def test_find_path_yaml_none_found(self, tmp_path):
        """Return empty list when no path.yaml files exist."""
        results = find_all_path_yaml(tmp_path)
        assert results == []
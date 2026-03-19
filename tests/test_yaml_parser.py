import tempfile
from pathlib import Path
import yaml
from dots.core.yaml_parser import (
    parse_path_yaml,
    parse_dependencies,
    detect_variants,
    filter_by_variant,
    DotFileMapping,
    VariantInfo,
    Dependency,
)

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

def test_dependency_package_managers_field(tmp_path):
    """Campo package-managers se parsea correctamente en type: package."""
    from dots.core.yaml_parser import parse_dependencies
    yaml_content = """
dependencies:
  - name: ripgrep
    type: package
    package-managers:
      pacman: ripgrep
      apt: ripgrep
      brew: ripgrep
  - name: eza
    type: package
    package-managers:
      pacman: eza
      brew: eza
  - name: git
    type: system
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    deps = parse_dependencies(yaml_path)
    assert len(deps) == 3

    rg = deps[0]
    assert rg.type == "package"
    assert rg.package_managers == {"pacman": "ripgrep", "apt": "ripgrep", "brew": "ripgrep"}

    eza = deps[1]
    assert eza.type == "package"
    assert eza.package_managers == {"pacman": "eza", "brew": "eza"}
    assert eza.package_managers.get("apt") is None

    git_dep = deps[2]
    assert git_dep.type == "system"
    assert git_dep.package_managers is None

def test_string_shorthand_is_package_type(tmp_path):
    """String shorthand usa type: package por default (no system)."""
    from dots.core.yaml_parser import parse_dependencies
    yaml_content = """
dependencies:
  - git
  - curl
  - name: zsh
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    deps = parse_dependencies(yaml_path)
    assert len(deps) == 3
    for dep in deps:
        assert dep.type == "package"
    assert deps[0].name == "git"
    assert deps[1].name == "curl"
    assert deps[2].name == "zsh"

def test_type_system_still_parsed(tmp_path):
    """type: system legacy se parsea sin error (alias tolerado)."""
    from dots.core.yaml_parser import parse_dependencies
    yaml_content = """
dependencies:
  - name: git
    type: system
"""
    yaml_path = tmp_path / "path.yaml"
    yaml_path.write_text(yaml_content)

    deps = parse_dependencies(yaml_path)
    assert len(deps) == 1
    assert deps[0].type == "system"
    assert deps[0].name == "git"


class TestVariants:

    def test_detect_variants_when_two_sources_share_destination(self, tmp_path):
        """Dos sources con mismo destination son detectados como variants."""
        mappings = [
            DotFileMapping("nvim-stdlib/init.vim", "~/.config/nvim/init.vim"),
            DotFileMapping("nvim-lazy/init.vim",   "~/.config/nvim/init.vim"),
        ]
        info = detect_variants(mappings)

        assert info.has_variants is True
        assert set(info.variants) == {"nvim-stdlib/init.vim", "nvim-lazy/init.vim"}

    def test_detect_variants_cascade_default_is_last(self, tmp_path):
        """El default variant es el último declarado en el YAML (cascade)."""
        mappings = [
            DotFileMapping("nvim-stdlib/init.vim", "~/.config/nvim/init.vim"),
            DotFileMapping("nvim-lazy/init.vim",   "~/.config/nvim/init.vim"),
        ]
        info = detect_variants(mappings)

        assert info.default_variant == "nvim-lazy/init.vim"

    def test_detect_variants_no_variants_when_destinations_differ(self):
        """Sources con destinations distintos NO son variants."""
        mappings = [
            DotFileMapping(".zshrc",  "~/.zshrc"),
            DotFileMapping(".zshenv", "~/.zshenv"),
        ]
        info = detect_variants(mappings)

        assert info.has_variants is False
        assert info.variants == []
        assert info.default_variant == ""

    def test_detect_variants_empty_mappings(self):
        """Lista vacía devuelve VariantInfo vacío."""
        info = detect_variants([])

        assert info.has_variants is False
        assert info.variants == []

    def test_filter_by_variant_returns_only_matching_source(self):
        """filter_by_variant retorna solo el mapping con el source indicado."""
        mappings = [
            DotFileMapping("nvim-stdlib/init.vim", "~/.config/nvim/init.vim"),
            DotFileMapping("nvim-lazy/init.vim",   "~/.config/nvim/init.vim"),
        ]
        result = filter_by_variant(mappings, "nvim-stdlib/init.vim")

        assert len(result) == 1
        assert result[0].source == "nvim-stdlib/init.vim"

    def test_filter_by_variant_empty_returns_all(self):
        """Sin variant (None o empty string), devuelve todos los mappings."""
        mappings = [
            DotFileMapping("nvim-stdlib/init.vim", "~/.config/nvim/init.vim"),
            DotFileMapping("nvim-lazy/init.vim",   "~/.config/nvim/init.vim"),
        ]
        assert filter_by_variant(mappings, "") == mappings
        assert filter_by_variant(mappings, None) == mappings

    def test_variant_destinations_maps_source_to_destination(self):
        """variant_destinations mapea cada source a su destination."""
        mappings = [
            DotFileMapping("nvim-stdlib/init.vim", "~/.config/nvim/init.vim"),
            DotFileMapping("nvim-lazy/init.vim",   "~/.config/nvim/init.vim"),
        ]
        info = detect_variants(mappings)

        assert info.variant_destinations["nvim-stdlib/init.vim"] == "~/.config/nvim/init.vim"
        assert info.variant_destinations["nvim-lazy/init.vim"]   == "~/.config/nvim/init.vim"

    def test_detect_variants_from_path_yaml(self, tmp_path):
        """detect_variants integrado con parse_path_yaml detecta variants reales."""
        yaml_content = """
files:
  - source: nvim-stdlib/init.vim
    destination: ~/.config/nvim/init.vim
  - source: nvim-lazy/init.vim
    destination: ~/.config/nvim/init.vim
"""
        yaml_path = tmp_path / "path.yaml"
        yaml_path.write_text(yaml_content)

        mappings = parse_path_yaml(yaml_path, "linux")
        info = detect_variants(mappings)

        assert info.has_variants is True
        assert info.default_variant == "nvim-lazy/init.vim"
        assert len(info.variants) == 2

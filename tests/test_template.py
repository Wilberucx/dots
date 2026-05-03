"""Tests for template module."""

from dots.core.template import (
    get_system_arch,
    resolve_arch,
    render,
    build_context,
)


class TestGetSystemArch:
    def test_returns_known_architectures(self):
        arch = get_system_arch()
        assert arch in ["x86_64", "aarch64"]


class TestResolveArch:
    def test_without_arch_map_returns_raw(self):
        """Sin arch_map, devuelve el valor bruto del sistema."""
        result = resolve_arch(None)
        assert result == get_system_arch()

    def test_with_arch_map_covers_current(self):
        """Con arch_map que cubre la arch actual, usa el mapeo."""
        arch = get_system_arch()
        arch_map = {"x86_64": "amd64", "aarch64": "arm64"}
        result = resolve_arch(arch_map)
        assert result == arch_map.get(arch, arch)

    def test_with_arch_map_does_not_cover_current(self):
        """Con arch_map que NO cubre la arch, usa fallback (raw)."""
        # Usamos un map que no tiene la arch actual
        arch_map = {"unknown": "mapped"}
        result = resolve_arch(arch_map)
        # Debe fallback al valor raw porque unknown no está en el map
        assert result == get_system_arch()


class TestRender:
    def test_single_placeholder(self):
        """Reemplaza un solo placeholder."""
        result = render("{{version}}", {"version": "v1.0.0"})
        assert result == "v1.0.0"

    def test_multiple_placeholders(self):
        """Reemplaza múltiples placeholders."""
        result = render("fd-{{version}}-{{arch}}.tar.gz", {"version": "v8.7.0", "arch": "x86_64"})
        assert result == "fd-v8.7.0-x86_64.tar.gz"

    def test_missing_key_removes_placeholder(self):
        """Clave ausente → leave placeholder."""
        result = render("{{version}}-{{arch}}", {"version": "v1.0"})
        assert result == "v1.0-{{arch}}"

    def test_empty_value_replaces_with_empty_string(self):
        """Valor vacío → reemplaza con string vacío."""
        result = render("{{version}}file", {"version": ""})
        assert result == "file"


class TestBuildContext:
    def test_with_version_and_arch_map(self):
        """Build context con version y arch_map."""
        arch_map = {"x86_64": "amd64"}
        ctx = build_context("v2.50.0", arch_map)
        assert ctx["version"] == "v2.50.0"
        assert ctx["arch"] == "amd64"  # mapeado por arch_map

    def test_with_version_no_arch_map(self):
        """Build context con version, sin arch_map."""
        ctx = build_context("v1.0.0", None)
        assert ctx["version"] == "v1.0.0"
        assert ctx["arch"] == get_system_arch()

    def test_without_version(self):
        """Build context sin version."""
        ctx = build_context(None, None)
        assert ctx["version"] == ""
        assert ctx["arch"] == get_system_arch()
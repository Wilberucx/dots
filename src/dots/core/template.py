"""Template engine for dependency URL resolution."""

import platform
from typing import Optional


def get_system_arch() -> str:
    """Detect system architecture (x86_64, aarch64, etc)."""
    machine = platform.machine().lower()
    if machine in ["x86_64", "amd64"]:
        return "x86_64"
    elif machine in ["aarch64", "arm64"]:
        return "aarch64"
    return machine


def resolve_arch(arch_map: Optional[dict[str, str]]) -> str:
    """
    Resuelve la arquitectura final.

    Si hay arch_map, traduce. Si no, devuelve el valor bruto del sistema.
    """
    raw = get_system_arch()
    if arch_map:
        return arch_map.get(raw, raw)
    return raw


def render(template: str, context: dict[str, str]) -> str:
    """
    Reemplaza {{key}} con context[key] en el template.

    >>> render("fd-{{version}}-{{arch}}.tar.gz", {"version": "v8.7.0", "arch": "x86_64"})
    'fd-v8.7.0-x86_64.tar.gz'
    """
    result = template
    for key, value in context.items():
        placeholder = f"{{{{{key}}}}}"
        result = result.replace(placeholder, value)
    return result


def build_context(version: Optional[str], arch_map: Optional[dict]) -> dict[str, str]:
    """
    Construye el contexto de templating para una dependency.
    """
    return {
        "arch": resolve_arch(arch_map),
        "version": version or "",
    }
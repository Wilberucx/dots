from pathlib import Path
from typing import List, Optional, Dict, Any, Union
from dataclasses import dataclass
import yaml
from dots.core.system import detect_os


@dataclass(frozen=True)
class Dependency:
    """Represents a dependency to be installed."""

    name: str
    type: str = "package"  # system, git, binary, script, curl, package
    source: Optional[str] = None  # URL, package name, or script content
    target: Optional[str] = None  # Destination path (for git/binary)
    version: Optional[str] = None
    ref: Optional[str] = None  # git tag/branch/commit hash
    arch_map: Optional[Dict[str, str]] = None  # Mapping for architecture-specific URLs
    post_install: Optional[str] = None  # Command to run after installation
    package_managers: Optional[Dict[str, str]] = None  # {"pacman": "pkg", "apt": "pkg"}
    extract_path: Optional[str] = None  # ruta relativa del binario dentro del tarball
    fallback: Optional[Dict[str, Any]] = (
        None  # inline dep para cuando PM no tiene el paquete
    )


@dataclass(frozen=True)
class DotFileMapping:
    """Immutable mapping from source file to destination."""

    source: str
    destination: str


@dataclass
class VariantInfo:
    """Information about variant configurations in a module."""

    has_variants: bool
    variants: List[str]  # sources that share destinations
    default_variant: str  # last variant (cascade)
    variant_destinations: Dict[str, str]  # source -> destination mapping


def parse_path_yaml(yaml_path: Path, current_os: str = None) -> List[DotFileMapping]:
    """
    Parse a path.yaml file and return a list of file mappings for the current OS.
    """
    if not yaml_path.exists():
        return []

    if current_os is None:
        current_os = detect_os()

    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError:
        return []

    if not data or "files" not in data:
        return []

    mappings: List[DotFileMapping] = []

    for item in data.get("files", []):
        source = item.get("source")
        if not source:
            continue

        # OS Filtering at item level
        allowed_os = item.get("os")
        if allowed_os and current_os not in allowed_os:
            continue

        # Determine destination
        # 1. explicit 'destination-OS'
        # 2. default 'destination'

        dest = None

        # 1. Nuevo override explícito por OS
        override = item.get("destination-override", {})
        if isinstance(override, dict):
            dest = override.get(current_os)

        # 2. Retrocompatibilidad con destination-linux / destination-mac
        if not dest:
            dest = item.get(f"destination-{current_os}")

        # 3. Destino genérico
        if not dest:
            dest = item.get("destination")

        if not dest:
            continue

        mappings.append(DotFileMapping(source.rstrip("/"), dest))

    return mappings


def parse_dependencies(yaml_path: Path) -> List[Dependency]:
    """
    Parse a path.yaml file and return a list of dependencies.
    Supports both simple strings (legacy) and complex objects.
    """
    if not yaml_path.exists():
        return []

    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError:
        return []

    if not data or "dependencies" not in data:
        return []

    raw_deps = data.get("dependencies", [])
    dependencies: List[Dependency] = []

    for d in raw_deps:
        if isinstance(d, str):
            # Legacy string format -> Package
            dependencies.append(Dependency(name=d))
        elif isinstance(d, dict):
            # Complex object format
            name = d.get("name")
            if not name:
                continue  # Skip unnamed dependencies

            dependencies.append(
                Dependency(
                    name=name,
                    type=d.get("type", "package"),
                    source=d.get("source"),
                    target=d.get("target"),
                    version=d.get("version"),
                    ref=d.get("ref"),
                    arch_map=d.get("arch_map"),
                    post_install=d.get("post_install"),
                    package_managers=d.get("package-managers"),
                    extract_path=d.get("extract-path"),
                    fallback=d.get("fallback"),
                )
            )

    return dependencies


def detect_variants(mappings: List[DotFileMapping]) -> VariantInfo:
    """
    Detect variants in mappings: same destination + multiple sources.

    Returns VariantInfo with:
    - has_variants: True if any destination has multiple sources
    - variants: list of all source names that are variants
    - default_variant: the LAST variant (cascade behavior)
    - variant_destinations: source -> destination mapping
    """
    if not mappings:
        return VariantInfo(
            has_variants=False, variants=[], default_variant="", variant_destinations={}
        )

    # Group by destination
    dest_to_sources: Dict[str, List[str]] = {}
    for m in mappings:
        dest_to_sources.setdefault(m.destination, []).append(m.source)

    # Find destinations with multiple sources (variants)
    all_variants: List[str] = []
    variant_destinations: Dict[str, str] = {}

    for destination, sources in dest_to_sources.items():
        if len(sources) > 1:
            all_variants.extend(sources)
            for source in sources:
                variant_destinations[source] = destination

    # Maintain original order
    all_variants_ordered = []
    seen = set()
    for m in mappings:
        if m.source not in seen and m.source in all_variants:
            all_variants_ordered.append(m.source)
            seen.add(m.source)

    return VariantInfo(
        has_variants=len(all_variants_ordered) > 0,
        variants=all_variants_ordered,
        default_variant=all_variants_ordered[-1] if all_variants_ordered else "",
        variant_destinations=variant_destinations,
    )


def filter_by_variant(
    mappings: List[DotFileMapping], variant: str
) -> List[DotFileMapping]:
    """
    Filter mappings to only include a specific variant source.

    If variant is empty/None, returns all mappings.
    """
    if not variant:
        return mappings

    return [m for m in mappings if m.source.rstrip("/") == variant.rstrip("/")]


def parse_module_meta(yaml_path: Path) -> dict:
    """
    Parse top-level metadata fields from a path.yaml.
    Returns a dict with optional keys: 'type'.
    Returns empty dict if file doesn't exist or has no metadata.
    """
    if not yaml_path.exists():
        return {}

    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError:
        return {}

    if not data or not isinstance(data, dict):
        return {}

    meta = {}
    if "type" in data:
        meta["type"] = str(data["type"])

    return meta

from pathlib import Path
from typing import List, Optional, Dict, Any, Union
from dataclasses import dataclass
import yaml
from dots.core.system import detect_os
from dots.core.schema import validate_dependency, validate_file_mapping, detect_v2_schema
from dots.ui.output import print_error


@dataclass(frozen=True)
class Dependency:
    """Represents a dependency to be installed."""

    name: str
    type: str = "package"  # package, git, binary, script, curl
    url: Optional[str] = None          # URL, package name, or script content (era: source)
    dest: Optional[str] = None         # Destination path (for git/binary) (era: target)
    version: Optional[str] = None
    ref: Optional[str] = None  # git tag/branch/commit hash
    arch: Optional[Dict[str, str]] = None  # Mapping for architecture-specific URLs (era: arch_map)
    managers: Optional[Dict[str, str]] = None  # {"pacman": "pkg", "apt": "pkg"} (era: package_managers)
    extract: Optional[str] = None  # ruta relativa del binario dentro del tarball (era: extract_path)
    fallback: Optional[Dict[str, Any]] = None  # inline dep para cuando PM no tiene el paquete
    post_install: Optional[str] = None  # Command to run after installation
    bin: Optional[str] = None  # nombre del ejecutable cuando difiere de name


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

    if not data:
        return []

    # Early fail on v2 schema
    v2_errors = detect_v2_schema(data, str(yaml_path))
    if v2_errors:
        for err in v2_errors:
            print_error(err)
        return []

    if "files" not in data:
        return []

    mappings: List[DotFileMapping] = []

    for item in data.get("files", []):
        # Validate file mapping
        errors = validate_file_mapping(item, str(yaml_path))
        # Errors logged by caller

        source = item.get("source")
        if not source:
            continue

        # OS Filtering at item level
        allowed_os = item.get("os")
        if allowed_os and current_os not in allowed_os:
            continue

        # Determine destination
        # 1. per-os[current_os]
        # 2. default 'destination'
        # 3. Skip

        dest = None

        # 1. Override explícito por OS
        per_os = item.get("per-os", {})
        if isinstance(per_os, dict):
            dest = per_os.get(current_os)

        # 2. Destino genérico
        if not dest:
            dest = item.get("destination")

        if not dest:
            continue

        mappings.append(DotFileMapping(source.rstrip("/"), dest))

    return mappings


def parse_single_dependency(raw_dict: Dict[str, Any]) -> Dependency:
    """
    Parses a single dependency dictionary into a Dependency dataclass.
    Used by both parse_dependencies and fallback logic.
    """
    name = raw_dict.get("name")
    
    return Dependency(
        name=name,
        type=raw_dict.get("type", "package"),
        url=raw_dict.get("url"),
        dest=raw_dict.get("dest"),
        version=raw_dict.get("version"),
        ref=raw_dict.get("ref"),
        arch=raw_dict.get("arch"),
        managers=raw_dict.get("managers"),
        extract=raw_dict.get("extract"),
        post_install=raw_dict.get("post-install") or raw_dict.get("post_install"),
        fallback=raw_dict.get("fallback"),
        bin=raw_dict.get("bin"),
    )


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

    if not data:
        return []

    # Early fail on v2 schema
    v2_errors = detect_v2_schema(data, str(yaml_path))
    if v2_errors:
        for err in v2_errors:
            print_error(err)
        return []

    if "dependencies" not in data:
        return []

    raw_deps = data.get("dependencies", [])
    dependencies: List[Dependency] = []

    # Validate dependencies
    for d in raw_deps:
        if isinstance(d, str):
            # Legacy string format -> Package
            dependencies.append(Dependency(name=d))
        elif isinstance(d, dict):
            # Validate before parsing
            errors = validate_dependency(d, str(yaml_path))
            if errors:
                # Errors are logged by caller or via schema validation
                pass

            name = d.get("name")
            if not name:
                continue  # Skip unnamed dependencies

            dependencies.append(parse_single_dependency(d))

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

    def _variant_key(source: str) -> str:
        """Normalize source name for variant grouping.

        Glob sources like 'wilber/*' are keyed as 'wilber' so the user can
        use ``--variant wilber`` instead of the unwieldy ``--variant 'wilber/*'``.
        """
        return source.rstrip("/*") if "*" in source else source

    # Group by destination — use the normalized key so glob sources participate
    # in variant detection together with non-glob sources.
    dest_to_sources: Dict[str, List[str]] = {}
    for m in mappings:
        key = _variant_key(m.source)
        dest_to_sources.setdefault(m.destination, []).append(key)

    # Find destinations with multiple sources (variants)
    all_variants: List[str] = []
    variant_destinations: Dict[str, str] = {}

    for destination, sources in dest_to_sources.items():
        if len(sources) > 1:
            all_variants.extend(sources)
            for source in sources:
                variant_destinations[source] = destination

    # Maintain original order using normalized keys
    all_variants_ordered = []
    seen = set()
    for m in mappings:
        key = _variant_key(m.source)
        if key not in seen and key in all_variants:
            all_variants_ordered.append(key)
            seen.add(key)

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
    Supports both exact matches and normalized glob matches:
    ``--variant wilber`` matches a source of ``wilber/*``.
    """
    if not variant:
        return mappings

    def _normalized(source: str) -> str:
        return source.rstrip("/*") if "*" in source else source

    variant_norm = variant.rstrip("/")
    return [m for m in mappings if _normalized(m.source) == variant_norm]


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


def validate_path_yaml(yaml_path: Path) -> list[str]:
    """
    Valida un path.yaml y retorna lista de errores.
    Retorna lista vacía si todo OK.
    """
    from dots.core.schema import validate_path_yaml as schema_validate

    if not yaml_path.exists():
        return []

    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
    except yaml.YAMLError:
        return [f"[{yaml_path}]: YAML inválido"]

    if not data:
        return []

    return schema_validate(data, str(yaml_path))

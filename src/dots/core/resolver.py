"""
Shared module resolver for Dots CLI.

This module eliminates code duplication between link.py and status.py
by providing a single function to scan modules and resolve symlink states.
"""

from dataclasses import dataclass
from pathlib import Path
from typing import Literal
import os

from dots.core.config import DotsConfig
from dots.core.system import is_safe_path
from dots.core.yaml_parser import (
    parse_path_yaml,
    detect_variants,
    filter_by_variant,
    VariantInfo,
)


LinkState = Literal["linked", "conflict", "pending", "missing", "unsafe"]


@dataclass
class LinkStatus:
    """Status of a single source -> destination mapping."""

    source: Path
    destination: Path
    state: LinkState
    detail: str = ""


def expand_path(path_str: str) -> Path:
    """Expand ~ and convert to absolute path."""
    return Path(os.path.expanduser(path_str)).absolute()


def get_module_variant_info(config: DotsConfig, module_name: str) -> VariantInfo | None:
    """
    Get variant information for a specific module.

    Args:
        config: DotsConfig with module directories and OS info
        module_name: Name of the module to check

    Returns:
        VariantInfo if module has path.yaml, None otherwise
    """
    module_dir = config.repo_root / module_name
    yaml_path = module_dir / "path.yaml"

    if not yaml_path.exists():
        return None

    mappings = parse_path_yaml(yaml_path, config.current_os)
    if not mappings:
        return None

    return detect_variants(mappings)


def get_active_variant(config: DotsConfig, module_name: str) -> str | None:
    """
    Determine which variant is currently active for a module by
    inspecting existing symlinks.

    Returns the source name of the active variant, or None if
    the module has no variants or none is linked.
    """
    module_dir = config.repo_root / module_name
    yaml_path = module_dir / "path.yaml"

    if not yaml_path.exists():
        return None

    mappings = parse_path_yaml(yaml_path, config.current_os)
    variant_info = detect_variants(mappings)

    if not variant_info.has_variants:
        return None

    for variant_source, destination in variant_info.variant_destinations.items():
        dest_path = expand_path(destination)
        src_path = (module_dir / variant_source.lstrip("/")).resolve()

        if dest_path.is_symlink():
            try:
                link_target = dest_path.readlink()
                if not link_target.is_absolute():
                    link_target = (dest_path.parent / link_target).resolve()
                else:
                    link_target = link_target.resolve()
                if link_target == src_path:
                    return variant_source
            except (OSError, ValueError):
                continue

    return None


def get_module_available_sources(config: DotsConfig, module_name: str) -> list[str]:
    """
    Get all available source names for a specific module.

    Args:
        config: DotsConfig with module directories and OS info
        module_name: Name of the module to check

    Returns:
        List of source names defined in path.yaml
    """
    variant_info = get_module_variant_info(config, module_name)
    if not variant_info:
        return []

    # Return all sources (not just variants)
    module_dir = config.repo_root / module_name
    yaml_path = module_dir / "path.yaml"
    mappings = parse_path_yaml(yaml_path, config.current_os)
    return [m.source for m in mappings]


def resolve_modules(
    config: DotsConfig,
    modules: list[str] | None = None,
    types: list[str] | None = None,
    variant: str | None = None,
) -> dict[str, list[LinkStatus]]:
    """
    Scan all configuration modules and return link status for each.

    Args:
        config: DotsConfig with module directories and OS info
        modules: Optional list of module names to filter down to
        types: Optional list of module types to filter by
        variant: Optional variant name to filter by. If None and module has variants,
                 uses the default variant (last one - cascade behavior).

    Returns:
        Dict mapping module name -> list of LinkStatus objects
    """
    results = {}

    for module_dir in config.get_module_dirs(modules=modules, types=types):
        yaml_path = module_dir / "path.yaml"
        if not yaml_path.exists():
            continue

        mappings = parse_path_yaml(yaml_path, config.current_os)
        if not mappings:
            continue

        # Detect variants in this module
        variant_info = detect_variants(mappings)

        # Apply variant filtering
        if variant:
            # User specified a variant - validate it exists
            if not variant_info.has_variants:
                # No variants in this module, but user asked for one
                # This will be handled at CLI level with proper error
                pass
            mappings = filter_by_variant(mappings, variant)
        elif variant_info.has_variants:
            # Use active variant if one is linked, else cascade to default
            active = get_active_variant(config, module_dir.name)
            effective = active if active else variant_info.default_variant
            mappings = filter_by_variant(mappings, effective)

        statuses = []

        for m in mappings:
            # Clean source path
            clean_source = m.source.lstrip("/")
            if not clean_source:
                clean_source = "."

            source_path = module_dir / clean_source

            # Handle globs
            if "*" in clean_source:
                sources = list(module_dir.glob(clean_source))
            else:
                sources = [source_path] if source_path.exists() else []

            # Does the YAML destination express an "expand into" intent?
            # Convention: trailing slash on the destination (e.g. ~/.gemini/)
            # means "link the *contents* of the source dir into dest", not the
            # directory itself.
            dest_is_container = m.destination.endswith("/")
            is_glob_source = "*" in clean_source

            for src in sources:
                # OS suffix check (file-linux, file-mac, file-windows)
                if "-" in src.name:
                    suffix = src.name.rsplit("-", 1)[-1]
                    if (
                        suffix in ["linux", "mac", "windows"]
                        and suffix != config.current_os
                    ):
                        continue

                dest = expand_path(m.destination)

                # Determine final destination.
                #
                # Priority:
                # 1. Glob source (wilber/*) → each expanded file goes inside dest
                # 2. Dir source + trailing slash dest → expand contents of the dir
                # 3. Dir source (no trailing slash) → symlink the dir itself
                # 4. File + existing dest dir → place file inside it
                # 5. File + non-existing dest → dest IS the symlink target name
                if is_glob_source:
                    final_dest = dest / src.name
                elif src.is_dir() and dest_is_container:
                    # Expand contents: link each child file into dest individually
                    for child in sorted(src.iterdir()):
                        child_dest = dest / child.name
                        if not is_safe_path(child_dest):
                            statuses.append(
                                LinkStatus(
                                    source=child,
                                    destination=child_dest,
                                    state="unsafe",
                                    detail="path outside home directory",
                                )
                            )
                            continue
                        # Inline state resolution for the child
                        if child_dest.is_symlink():
                            t = child_dest.readlink()
                            if not t.is_absolute():
                                t = (child_dest.parent / t).resolve()
                            else:
                                t = t.resolve()
                            if t == child.resolve():
                                statuses.append(LinkStatus(source=child, destination=child_dest, state="linked"))
                            else:
                                statuses.append(LinkStatus(source=child, destination=child_dest, state="conflict", detail=f"points to {t}"))
                        elif child_dest.exists():
                            statuses.append(LinkStatus(source=child, destination=child_dest, state="pending", detail="backup needed"))
                        else:
                            statuses.append(LinkStatus(source=child, destination=child_dest, state="pending", detail="will create"))
                    continue  # skip the shared state-check below
                elif src.is_dir():
                    final_dest = dest
                else:
                    final_dest = dest / src.name if dest.is_dir() else dest

                # Safety check
                if not is_safe_path(final_dest):
                    statuses.append(
                        LinkStatus(
                            source=src,
                            destination=final_dest,
                            state="unsafe",
                            detail="path outside home directory",
                        )
                    )
                    continue

                # Check current state
                if final_dest.is_symlink():
                    target = final_dest.readlink()
                    if not target.is_absolute():
                        target = (final_dest.parent / target).resolve()
                    else:
                        target = target.resolve()

                    if target == src.resolve():
                        statuses.append(
                            LinkStatus(
                                source=src,
                                destination=final_dest,
                                state="linked",
                                detail="",
                            )
                        )
                    else:
                        statuses.append(
                            LinkStatus(
                                source=src,
                                destination=final_dest,
                                state="conflict",
                                detail=f"points to {target}",
                            )
                        )
                elif final_dest.exists():
                    statuses.append(
                        LinkStatus(
                            source=src,
                            destination=final_dest,
                            state="pending",
                            detail="backup needed",
                        )
                    )
                else:
                    statuses.append(
                        LinkStatus(
                            source=src,
                            destination=final_dest,
                            state="pending",
                            detail="will create",
                        )
                    )

        if statuses:
            results[module_dir.name] = statuses

    return results

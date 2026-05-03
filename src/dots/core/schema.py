"""Schema validation for path.yaml dependencies and files."""

from typing import Optional


# Campos requeridos por tipo de dependency
REQUIRED_FIELDS: dict[str, list[str]] = {
    "binary": ["url", "dest"],
    "git": ["url", "dest"],
    "package": ["name"],
}


def validate_dependency(raw: dict, yaml_path: str) -> list[str]:
    """
    Valida un dict crudo de dependency.
    Retorna lista de errores (vacía si todo ok).
    """
    errors = []
    dep_type = raw.get("type", "package")
    dep_name = raw.get("name", "<unnamed>")
    prefix = f"[{yaml_path}] dependency '{dep_name}'"

    # Tipo válido
    if dep_type not in REQUIRED_FIELDS:
        errors.append(f"{prefix}: type '{dep_type}' desconocido (conocidos: {', '.join(REQUIRED_FIELDS.keys())})")

    # Campos requeridos por tipo
    if dep_type in REQUIRED_FIELDS:
        for field in REQUIRED_FIELDS[dep_type]:
            if not raw.get(field):
                errors.append(f"{prefix}: campo requerido '{field}' faltante para type '{dep_type}'")

    return errors


def validate_file_mapping(raw: dict, yaml_path: str) -> list[str]:
    """
    Valida un dict crudo de file mapping.
    Retorna lista de errores (vacía si todo ok).
    """
    errors = []
    source = raw.get("source", "<unnamed>")
    prefix = f"[{yaml_path}] file mapping '{source}'"

    if not raw.get("source"):
        errors.append(f"{prefix}: sin 'source'")

    has_destination = raw.get("destination") or raw.get("per-os")
    if not has_destination:
        errors.append(f"{prefix}: sin 'destination' ni 'per-os'")

    # Validar per-os si existe
    per_os = raw.get("per-os")
    if per_os and not isinstance(per_os, dict):
        errors.append(f"{prefix}: 'per-os' debe ser un dict")

    # Validar os si existe
    os_filter = raw.get("os")
    if os_filter and not isinstance(os_filter, list):
        errors.append(f"{prefix}: 'os' debe ser una lista")

    return errors


def validate_path_yaml(data: dict, yaml_path: str) -> list[str]:
    """
    Valida un path.yaml completo.
    Retorna lista de errores (vacía si todo ok).
    """
    errors = []

    if not isinstance(data, dict):
        return [f"[{yaml_path}]: debe ser un dict"]

    # Validar dependencies
    dependencies = data.get("dependencies", [])
    if dependencies and not isinstance(dependencies, list):
        errors.append(f"[{yaml_path}]: 'dependencies' debe ser una lista")
    elif isinstance(dependencies, list):
        for i, dep in enumerate(dependencies):
            if isinstance(dep, dict):
                errors.extend(validate_dependency(dep, yaml_path))
            elif isinstance(dep, str):
                # Strings son válidas (legacy shorthand)
                pass
            else:
                errors.append(f"[{yaml_path}]: dependency #{i} debe ser dict o string")

    # Validar files
    files = data.get("files", [])
    if files and not isinstance(files, list):
        errors.append(f"[{yaml_path}]: 'files' debe ser una lista")
    elif isinstance(files, list):
        for i, file_map in enumerate(files):
            if isinstance(file_map, dict):
                errors.extend(validate_file_mapping(file_map, yaml_path))
            else:
                errors.append(f"[{yaml_path}]: file #{i} debe ser un dict")

    return errors
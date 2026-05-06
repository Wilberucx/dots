# src/dots/core/module_writer.py

import yaml
from pathlib import Path
from typing import Optional


def destination_str(abs_path: Path, home_dir: Path) -> str:
    """Retorna el destination en formato ~/... para escribir en path.yaml."""
    try:
        return f"~/{abs_path.relative_to(home_dir)}"
    except ValueError:
        return str(abs_path)


def load_module_data(yaml_path: Path) -> dict:
    """
    Lee un path.yaml y retorna el dict crudo.
    Retorna estructura vacía si no existe o está corrupto.
    """
    if not yaml_path.exists():
        return {"files": []}
    try:
        with open(yaml_path, "r") as f:
            data = yaml.safe_load(f)
            return data if data and isinstance(data, dict) else {"files": []}
    except yaml.YAMLError:
        return {"files": []}


def is_destination_declared(data: dict, destination: str) -> bool:
    """
    Retorna True si destination ya está declarado en files.
    Compatible con schema v3 (per-os) y fallback a destination genérico.
    """
    for entry in data.get("files", []):
        if not isinstance(entry, dict):
            continue

        # Schema v3: destination genérico
        if entry.get("destination") == destination:
            return True

        # Schema v3: per-os
        per_os = entry.get("per-os", {})
        if isinstance(per_os, dict) and destination in per_os.values():
            return True

    return False


def append_file_entry(yaml_path: Path, entry: dict) -> None:
    """
    Agrega una entrada a la lista files de un path.yaml.
    Crea el archivo si no existe.
    """
    data = load_module_data(yaml_path)
    data["files"].append(entry)
    with open(yaml_path, "w") as f:
        yaml.dump(data, f, sort_keys=False, allow_unicode=True)

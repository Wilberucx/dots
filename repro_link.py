import sys
import os
from pathlib import Path

# Agregar src al path
sys.path.insert(0, str(Path("src").resolve()))

from dots.ui.dashboard import _capture_cmd
from dots.commands.link import link_cmd
from dots.core.config import DotsConfig

# Asegurar que estamos en el root del repo (donde está dots.toml)
# Si no existe dots.toml, crearlo dummy para que DotsConfig no falle
if not Path("dots.toml").exists():
    with open("dots.toml", "w") as f:
        f.write("")

print("Intentando linkear 'Alacritty' vía _capture_cmd...")

# Mockear argumentos como lo hace la TUI
names = ["Alacritty"]

# La firma de link_cmd en dashboard.py:1559 es:
# link_cmd(module=names, dry_run=False, force=False, interactive=False, variant=None)
# Aquí usamos los argumentos del usuario
success, lines = _capture_cmd(
    link_cmd,
    module=names,
    dry_run=True,  # Usar dry_run para no romper nada real
    force=False,
    interactive=False,
    variant=None,
)

print(f"Success: {success}")
print("Captured lines:")
for line in lines:
    print(f"  {line}")

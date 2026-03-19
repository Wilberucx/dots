# dots — Pending Implementations

Especificaciones de features e impovements pendientes de implementación.
Cada ítem tiene su contexto y criterios de aceptación definidos.
Actualizar este archivo al cerrar cada sesión de trabajo.

---

## UI / Output

### [x] Status command — labels y colores semánticos
**Contexto:** El output actual de `dots status` muestra "Needs Review" para módulos
no linkeados, lo que implica que algo salió mal cuando en realidad es un estado neutral.
Los colores deben comunicar estado al instante, sin necesidad de leer el texto.

**Cambios:**
- "Needs Review" → "Unlinked" con color neutro (blanco o gris)
- Symlinks rotos → label "Broken" con color rojo explícito
- Linked → verde (sin cambios)

**Archivos involucrados:** `src/dots/ui/output.py`, `src/dots/commands/status.py`

---

## CLI / Comandos

### [x] Flag `--module` / `-m` en link, unlink, status, install
**Contexto:** Hoy los comandos operan sobre todos los módulos o ninguno.
Con 31+ módulos en el repo real, la ausencia de filtro por módulo genera fricción.

**Comportamiento esperado:**
```bash
dots link --module Zsh
dots link -m Zsh -m Nvim
dots status --module Fish
dots install --module Packages
```

**Implementación:** El filtro se agrega en `config.get_module_dirs()` o en el punto
de iteración de cada comando. Los cuatro comandos comparten la misma lógica — 
el cambio se hace una vez y se propaga.

**Archivos involucrados:** `src/dots/commands/link.py`, `src/dots/commands/unlink.py`,
`src/dots/commands/status.py`, `src/dots/commands/install.py`

### [x] Flag `--type` / `-t` en link, status, install
**Contexto:** Permite agrupar módulos por tipo declarado en path.yaml y
operar solo sobre ese grupo. Más flexible que un flag `--default` hardcodeado
— el usuario define sus propios nombres de grupo.

**Schema — campo nuevo top-level en path.yaml:**
```yaml
type: minimal   # libre, opcional, sin valores predefinidos
files:
  - source: .zshrc
    destination: ~/.zshrc
```

**Comportamiento:**
```bash
dots link --type minimal        # linkea solo módulos con type: minimal
dots link -t minimal -t work    # múltiples tipos (OR)
dots link                       # sin flag = todos los módulos (sin cambios)
dots status --type minimal      # filtrar status por tipo
dots install --type minimal     # instalar deps solo de ese grupo
```

**Implementación:**
- Agregar lectura del campo `type` top-level en `yaml_parser.py`
  (función `parse_module_meta(yaml_path) -> dict` que retorna metadata
  del módulo: type, default, etc.)
- Agregar flag `--type` / `-t` con `multiple=True` en link, status, install
- El filtro se aplica en el punto de iteración de módulos
- Sin conflicto con `--module` — pueden combinarse

**Archivos involucrados:** `src/dots/core/yaml_parser.py`,
`src/dots/commands/link.py`, `src/dots/commands/status.py`,
`src/dots/commands/install.py`, `src/dots/core/resolver.py`

### [x] Variants / Flavors — implementado, testeado
**Contexto:** Múltiples sources apuntando al mismo destination en path.yaml.
Cascade: último en YAML es el default. `--variant` para selección explícita.
UI: tab "flavors" en TUI. Core: `detect_variants`, `filter_by_variant` en yaml_parser.
Visual: `status` muestra variants expandidos con ● activo y ○ inactivos.
Link output muestra `[variant]` tag cuando el módulo tiene variants.

**Tests:** `tests/test_yaml_parser.py` — clase `TestVariants` (8 tests).

### [x] `adopt` inteligente — detecta módulo existente
**Contexto:** Al adoptar un archivo cuyo módulo ya existe y el destino ya está
declarado en path.yaml, ofrece crear un variant en lugar de sobreescribir.
**Comportamiento:**
- `_destination_already_declared()` detecta conflictos en path.yaml existente
- Si módulo existe + destino duplicado → pregunta si crear variant
- Variant flow: pide nombre de subcarpeta, crea dir, mueve archivo, agrega entry
- Caso normal: comportamiento anterior intacto
**Archivos involucrados:** `src/dots/commands/adopt.py`

### [x] `select_modules` — helper reutilizable de selección interactiva
**Contexto:** La lógica de `--interactive` en varios comandos estaba
duplicada. Extraída a helper compartido.
**Implementado:** `_checkbox` base privada, `select_modules` con filtros
`modules`/`types`, `select_variant` (single-choice), `confirm` (yes/no).
**Archivos involucrados:** `src/dots/ui/selector.py`

### [x] Flag `--format` en status
**Contexto:** El output actual de `dots status` está pensado para lectura humana.
`--format` permite consumir el mismo dato en otros formatos sin cambiar el comando.

**Valores:**
- Sin flag (default): output actual agrupado por estado — sin cambios
- `--format table`: tabla Rich con columnas Module / Source / Destination / State / Type
- `--format json`: JSON estructurado para scripting e integración con otros tools

**Estructura JSON esperada:**
```json
{
  "modules": {
    "Zsh": {
      "type": "minimal",
      "files": [
        {
          "source": ".zshrc",
          "destination": "~/.zshrc",
          "state": "linked"
        }
      ]
    }
  },
  "summary": {
    "linked": 23,
    "unlinked": 4,
    "broken": 0,
    "missing": 0,
    "unsafe": 0
  }
}
```

**Implementación:**
- Agregar `--format` / `-f` con enum `OutputFormat` (default, table, json)
  en `status_cmd`
- Extraer lógica de renderizado en funciones separadas:
  `_render_default`, `_render_table`, `_render_json`
- `_render_table` reutiliza la lógica visual que tenía `show.py`
- `_render_json` usa `json.dumps` con `indent=2`, respeta filtros activos

**Archivos involucrados:** `src/dots/commands/status.py`

---

## Core / install

### [x] Extracción de tar.gz en install_binary_dep
**Contexto:** La rama `.tar.gz` / `.tgz` de `install_binary_dep` usa `tar.extractall`
sin lógica de subrutas. Si el binario está dentro de un subdirectorio del tarball
(caso común en releases de GitHub), la extracción deposita todo en `dest.parent`
sin ubicar el binario correcto.

**Cambios necesarios:**
- Agregar campo opcional `extract_path` en `Dependency` — ruta relativa dentro
  del tarball donde vive el binario (ej: `"eza-linux-x86_64/eza"`)
- Si `extract_path` está presente, extraer solo ese miembro y moverlo a `dest`
- Si está ausente, comportamiento actual como fallback
- Agregar tests con tarball sintético usando `tarfile` de stdlib en memoria

**Archivos involucrados:** `src/dots/core/yaml_parser.py` (campo nuevo en Dependency),
`src/dots/commands/install.py` (lógica de extracción)

---

### [x] Campo `fallback` en dependencies
**Contexto:** Paquetes no disponibles en ciertos PM (ej: starship, eza en apt)
pueden declarar un fallback binary o git como alternativa automática.
Sin fallback el comportamiento es skip con warning (sin cambios).

---

## TUI

### [ ] Rewrite de dashboard.py con Textual
**Contexto:** `dashboard.py` implementa una máquina de estados monolítica con
manejo crudo de keyboard events y scroll manual. Es frágil y viola SRP.
Reemplazar con Textual cuando se inicie la fase de UI polish.

**Scope:** Rewrite completo, no refactor incremental.
**Prerequisito:** Cerrar todas las features de core primero.

### [ ] TUI panel 1 — columna ícono .bak
**Contexto:** En el panel principal de la TUI mostrar visualmente si un módulo
tiene un archivo `.bak` activo usando un ícono Nerd Font.
Comunica al instante que hay un backup pendiente de revisión sin necesidad
de leer texto.

**Comportamiento:**
- Si existe `<destination>.bak` para algún archivo del módulo → mostrar ícono (ej: )
- Si no hay .bak → sin ícono o ícono neutro
- El ícono debe usar color amarillo/warning — es información, no error

**Prerequisito:** Rewrite de dashboard.py con Textual completado primero.
**Archivos involucrados:** TBD cuando se inicie el rewrite con Textual.


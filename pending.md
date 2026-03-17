# dots — Pending Implementations

Especificaciones de features e impovements pendientes de implementación.
Cada ítem tiene su contexto y criterios de aceptación definidos.
Actualizar este archivo al cerrar cada sesión de trabajo.

---

## UI / Output

### [ ] Status command — labels y colores semánticos
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

### [ ] Flag `--module` / `-m` en link, unlink, status, install
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

### [ ] Flag `--default` en link
**Contexto:** Al bootstrapear una máquina nueva, el usuario necesita linkear
un subconjunto de módulos esenciales sin tener que especificarlos uno a uno.
`dots link --default` resuelve esto de forma declarativa, sin configuración
separada — consistente con el principio de que `path.yaml` es la única
fuente de verdad por módulo.

**Schema — campo nuevo en path.yaml:**
```yaml
default: true   # top-level, opcional, false por omisión
files:
  - source: .zshrc
    destination: ~/.zshrc
```

**Comportamiento:**
- `dots link` → linkea todos los módulos (comportamiento actual sin cambios)
- `dots link --default` → linkea solo módulos con `default: true` en su path.yaml

**Implementación:**
- Agregar lectura del campo `default` en `yaml_parser.py` (función nueva
  `parse_module_meta` o campo en el return de `parse_path_yaml`)
- Agregar flag `--default` en `link.py`
- El filtro se aplica antes de iterar módulos en `resolve_modules`

**Archivos involucrados:** `src/dots/core/yaml_parser.py`,
`src/dots/commands/link.py`, `src/dots/core/resolver.py`

---

## Core / install

### [ ] Extracción de tar.gz en install_binary_dep
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

## TUI

### [ ] Rewrite de dashboard.py con Textual
**Contexto:** `dashboard.py` implementa una máquina de estados monolítica con
manejo crudo de keyboard events y scroll manual. Es frágil y viola SRP.
Reemplazar con Textual cuando se inicie la fase de UI polish.

**Scope:** Rewrite completo, no refactor incremental.
**Prerequisito:** Cerrar todas las features de core primero.

---

## Backlog sin especificar aún

### [ ] Flag adicional (a definir con el orquestador)
**Contexto:** El usuario tiene una idea de flag pendiente de detallar.
Especificar cuando se retome el trabajo de CLI.

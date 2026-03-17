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

### [ ] Subcomando `show`
**Contexto:** Vista inline de todos los módulos y sus archivos en formato tabla.
Complementa `status` (que muestra estado de symlinks) con una vista
de qué archivos gestiona cada módulo y sus metadatos.

**Comportamiento:**
```bash
dots show                       # tabla completa de todos los módulos
dots show --type minimal        # filtrar por tipo declarado en path.yaml
dots show --state unlinked      # filtrar por estado de symlink
dots show --type minimal --state linked  # combinable
```

**Columnas de la tabla:**
- Módulo
- Archivo fuente
- Destino
- Estado (linked / unlinked / broken)
- Tipo del módulo (si está declarado)

**Archivos involucrados:** `src/dots/commands/` (archivo nuevo `show.py`),
`src/dots/core/resolver.py`, `__main__.py` (registro del subcomando)

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

---

## Backlog sin especificar aún

### [ ] Flag adicional (a definir con el orquestador)
**Contexto:** El usuario tiene una idea de flag pendiente de detallar.
Especificar cuando se retome el trabajo de CLI.

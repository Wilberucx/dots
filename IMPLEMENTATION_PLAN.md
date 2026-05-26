# 📋 Plan de Implementación: Migración de YAML a Lua (`dots.lua`)

_Versión: 2 — Actualizado: 2026-05-25_
_Estado: PENDIENTE DE APROBACIÓN_

---

## 1. Resumen Ejecutivo

Reemplazar el sistema de configuración basado en YAML (`path.yaml`, `.dots/config.yaml`) por archivos Lua embebidos (`dots.lua`, `init.lua`) usando `gopher-lua`. Esto elimina 3 DSLs inventados (files, dependencies, templates) en favor de una API Lua explícita. Se añade un sistema de plugins vía `require()` para las operaciones de dependencias (curl, git, archive), reemplazando el parsing manual de comandos. Se añaden operaciones de directorio (`dir():to()`, `dir():into()`) para manejar carpetas enteras sin declarar archivo por archivo.

**Principios de diseño:**

- **`module_paths` apunta DÓNDE buscar, no QUÉ cargar**: Es un field de `init.lua` que redirige la búsqueda de módulos a una ruta específica. Por defecto se busca en la raíz del repo.
- **Reemplaza, no suma**: `module_paths = "custom/"` significa "mis módulos están en `custom/`, no busques en otro lado". Si no está ahí, no es un módulo.
- **Flexibilidad de estructura**: No se impone una carpeta `modules/`. Los módulos se detectan escaneando directorios con `dots.lua` dentro de la ruta configurada.
- **Prioridad de dots.lua**: Si un directorio tiene ambos `dots.lua` y `path.yaml`, `dots.lua` tiene prioridad con un warning.
- **init.lua auto-generado**: `dots init` crea `init.lua` con configuración general.

**Tiempo estimado total:** 3-4 sprints (~3-4 semanas).

---

## 2. Contexto y Estado Actual

### Situación actual

- `dots` es un CLI de gestión de dotfiles escrito en Go (v0.11.0)
- Configuración por módulo: `path.yaml` (YAML con 3 DSLs propietarios)
- Configuración raíz: `.dots/config.yaml` (marker) + `dots.toml` (legacy)
- Dependencias: sistema de 3 tipos (package, git, binary) con template engine `{{version}}`/`{{arch}}`
- Package managers: pacman, apt, brew con detección automática
- Symlinks: archivo por archivo (con expansión limitada vía `/*` en destino, no documentada)
- Syntax checker para path.yaml
- Transaction log para rollback
- Bubbletea TUI para selección interactiva

### Cómo se descubre un módulo hoy

```go
// config.go — GetModuleDirs()
// 1. Lee todas las entradas del repo root
// 2. Filtra: solo directorios, sin prefijo "."
// 3. Busca path.yaml dentro de cada directorio
// 4. Si existe → es un módulo
```

Esto significa: los módulos son **siempre directorios planos en la raíz del repo**. No hay soporte para anidamiento ni `modules/` subdirectorios.

### Problema o gap

1. **3 DSLs inventados en path.yaml** — files, dependencies, y templates tienen sintaxis propia
2. **Sin operaciones de directorio nativas** — no se puede decir "toma esta carpeta y reemplaza esta otra". Hoy se simula con `/*` en destino, pero no está documentado ni es explícito.
3. **Comandos parseados dentro de dots** — curl, wget, unzip, tar se manejan con lógica ad-hoc
4. **Sin extensibilidad** — no hay forma de agregar nuevos tipos de dependencias sin modificar el core
5. **Dos archivos de configuración raíz** — `.dots/config.yaml` + `dots.toml` legacy

### Suposiciones validadas del diseño actual

- Cada módulo = un directorio con un archivo de configuración (hoy `path.yaml`, mañana `dots.lua`)
- Los módulos se descubren por presencia del archivo de configuración, no por listing explícito
- El OS se detecta en runtime (`runtime.GOOS` → `linux`/`mac`/`windows`)
- Los destinos pueden contener `~` que se expande al home del usuario
- Paths inseguros (fuera de `$HOME`) se bloquean excepto confirmación explícita
- Las dependencias se deduplican por `name` (primera definición gana)
- Los módulos no tienen estado interno — todo se resuelve desde los archivos de configuración

### Dependencias externas

- `github.com/yuin/gopher-lua v1.x` — runtime Lua 5.4 embebido
- `github.com/yuin/gluamapper` — mapeo de tablas Lua a Go structs (opcional)
- Las dependencias existentes se mantienen: cobra, bubbletea, lipgloss, yaml.v3 (temporal)

---

## 3. Arquitectura / Diseño de la Solución

### Estructura de repositorio (flexible)

```
dotfiles/                          ← raíz del repo
├── init.lua                       ← configuración raíz (reemplaza .dots/config.yaml + dots.toml)
├── Zsh/
│   ├── dots.lua                   ← configuración del módulo (reemplaza path.yaml)
│   └── .zshrc
├── Kitty/
│   ├── dots.lua
│   └── kitty.conf
├── Nvim/
│   ├── dots.lua
│   └── config/
├── misc/                          ← se puede agrupar
│   ├── Scripts/
│   │   ├── dots.lua
│   │   └── ...
│   └── Themes/
│       ├── dots.lua
│       └── ...
├── dots/                          ← plugins compartidos (opcional)
│   └── helpers.lua
└── .gitignore
```

**Reglas de descubrimiento de módulos:**

```lua
-- Regla ÚNICA:
-- Si init.lua tiene module_paths → buscar SOLO en esa(s) ruta(s)
-- Si NO tiene module_paths → buscar en la raíz del repo (default)
-- Si no está en la ruta de búsqueda → no es un módulo
-- De forma implícita, un módulo sin dots.lua es ignorado y se informa con advertencia ya que está en el Area de búsqueda y el usuario pudo olvidar colocar el dots.lua
```

`module_paths` acepta:

- Un string: `module_paths = "modules/"` → busca solo en `modules/`
- Una tabla: `module_paths = {"packages/", "custom/"}` → busca en ambas rutas

Ejemplos:

```lua
-- Default: busca en la raíz del repo
return { name = "dotfiles" }

-- Redirige: busca SOLO en "modules/"
return {
  name = "dotfiles",
  module_paths = "modules/",
}

-- Múltiples rutas: busca en "packages/" y "custom/"
return {
  name = "dotfiles",
  module_paths = {"packages/", "custom/"},
}
```

**¿Qué pasa con módulos fuera de la ruta?** No se cargan. Si el usuario quiere excluir módulos, los mueve fuera de la ruta de búsqueda o cambia `module_paths`.

### Diagrama de componentes

```
┌─────────────────────────────────────────────────────────────┐
│                        dots CLI                             │
├─────────────────────────────────────────────────────────────┤
│  internal/config/          internal/lua/                    │
│  ├── config.go (adaptado)  ├── vm.go          ← gopher-lua │
│  │                         ├── types.go       ← structs    │
│  │                         ├── api_files.go   ← file/dir   │
│  │                         ├── api_deps.go    ← pkg/git    │
│  │                         ├── loader.go      ← require()  │
│  │                         └── loader_modules.go ← scan    │
├─────────────────────────────────────────────────────────────┤
│  internal/yaml/ (FASE 0-4)  →  ELIMINAR (FASE 5)           │
│  internal/template/ (FASE 0-3)  →  ELIMINAR (FASE 5)       │
│  internal/plugins/  →  REFACTOR como plugin Lua             │
└─────────────────────────────────────────────────────────────┘
```

### Decisiones de Arquitectura

```
DECISIÓN: Descubrimiento de módulos
OPCIONES: Lista explícita / Auto-detección por dots.lua / `module_paths` que redirige búsqueda
ELECCIÓN: `module_paths` como field de `init.lua` que reemplaza la raíz de búsqueda
RAZÓN: Es simple, es Lua 100% válido (dentro de `return { ... }`), y no requiere funciones globales.
        Una sola regla: "mis módulos están aquí" (module_paths). Si no se declara, default = raíz.
        El usuario que no necesita personalización no escribe nada.
        El usuario que organiza en subcarpetas solo añade una línea.

DECISIÓN: Estructura de directorios
OPCIONES: Forzar modules/ / Flexible (cualquier profundidad) / Plana como hoy
ELECCIÓN: Flexible con module_paths para redirigir búsqueda
RAZÓN: El usuario actual tiene módulos planos en la raíz. Forzar modules/
        rompería su estructura. Pero no debemos prohibir anidamiento.

DECISIÓN: Lenguaje de scripting
OPCIONES: Lua (gopher-lua) / Starlark (starlark-go) / YAML v4 / TOML
ELECCIÓN: Lua (gopher-lua)
RAZÓN: gopher-lua es la librería más madura para embeker Lua 5.4 en Go.
        Starlark tiene menos ecosistema y su sintaxis es menos familiar.
        YAML/TOML no permiten el nivel de expresividad que necesitamos
        (funciones, condicionales, plugins vía require).

DECISIÓN: init.lua como marker
OPCIONES: init.lua / config.lua / dots.lua raíz
ELECCIÓN: init.lua (con config.lua como fallback)
RAZÓN: Es el naming estándar de Lua. Familiar para cualquiera que haya usado Lua.

DECISIÓN: Coexistencia YAML + Lua
ELECCIÓN: Coexistencia durante 3 fases, deprecación warning en fase 4, remoción en fase 5
RAZÓN: El usuario tiene dotfiles reales con path.yaml. No podemos romperle el setup sin aviso.
        Si ambos existen en el mismo módulo → dots.lua tiene prioridad con warning.

DECISIÓN: Operaciones de directorio
OPCIONES: dir():to() y dir():into() / flags en dir() / patrones de glob
ELECCIÓN: Métodos encadenables to() e into()
RAZÓN: to() es "reemplaza el destino con esta carpeta" (symlink de directorio).
       into() es "pon los contenidos dentro del destino" (symlinks individuales).
       Ambos son explícitos y no requieren aprender sintaxis nueva.
```

### API Lua Concreta

```lua
-- init.lua (raíz del repositorio) — GENERADO por `dots init`

-- Opción A: Default — busca módulos en la RAÍZ del repo
return {
  name = "cantoarch/dotfiles",
  plugins = { "dots.http", "dots.archive", "dots.git" },
}

-- Opción B: Redirigir búsqueda — busca SOLO en "modules/"
-- return {
--   name = "cantoarch/dotfiles",
--   module_paths = "modules/",
--   plugins = { "dots.http" },
-- }

-- Opción C: Múltiples rutas de búsqueda
-- return {
--   name = "cantoarch/dotfiles",
--   module_paths = {"packages/", "custom/"},
--   plugins = { "dots.http" },
-- }
```

```lua
-- Zsh/dots.lua
return {
  type = "minimal",

  files = {
    -- Archivo individual
    file(".zshrc", "~/.zshrc"),

    -- Archivo con filtro OS
    file(".zshenv", "~/.zshenv"):when("linux"),

    -- Destino por OS
    file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
      mac  = "~/Library/Application Support/alacritty.toml",
      linux = "~/.config/alacritty.toml",
    }),

    -- Directorio completo: reemplaza el destino con esta carpeta
    dir("config"):to("~/.config/tool"),

    -- Contenido del directorio: los archivos DENTRO van al destino
    dir("scripts"):into("~/.local/bin"),

    -- Glob: archivos que matchean el patrón
    glob("*.toml"):into("~/.config/"),
  },

  dependencies = {
    -- Sistema (detección automática de PM)
    pkg "ripgrep",

    -- Sistema con nombre distinto por PM
    pkg("fd"):on({
      pacman = "fd",
      apt    = "fd-find",
      brew   = "fd",
    }),

    -- Con fallback a binary
    pkg("starship"):on({ pacman = "starship", brew = "starship" })
      :fallback(curl("https://github.com/..."):extract("starship")),

    -- Binary: require del plugin http + archive
    curl("https://github.com/eza-community/eza/releases/...")
      :extract("eza")
      :to("~/.local/bin/eza"),

    -- Git clone
    git("https://github.com/romkatv/powerlevel10k.git")
      :to("~/.local/share/zsh/plugins/powerlevel10k")
      :at("v1.19.0"),
  },
}
```

### Modelo de datos interno

```go
// FileOp representa una operación de symlink
type FileOp struct {
    Type        FileOpType            // FileOpFile, FileOpDirTo, FileOpDirInto, FileOpGlob
    Source      string                // path relativo al módulo
    Destination string                // path de destino (con ~)
    Pattern     string                // para glob
    OSFilter    string                // "linux", "mac", "windows", ""
    PerOS       map[string]string     // destino por OS
}

type FileOpType int
const (
    FileOpFile    FileOpType = iota // file(): symlink simple
    FileOpDirTo                     // dir():to() — symlink de directorio
    FileOpDirInto                   // dir():into() — expandir contenidos
    FileOpGlob                      // glob():into() — pattern matching
)

// DepOp representa una dependencia
type DepOp struct {
    Name        string
    Type        string            // "package", "binary", "git"
    URL         string
    Destination string
    Version     string
    Ref         string
    Extract     string
    Arch        map[string]string
    Managers    map[string]string
    Bin         string
    PostInstall string
    Fallback    *DepOp
}
```

---

## 4. Assumptions y Edge-cases (Revisión exhaustiva)

### Assumptions del sistema actual que se mantienen

| #   | Assumption                                  | Impacto en diseño                                                                                                         | Validación                   |
| --- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| A1  | Los módulos son directorios en el repo      | `GetModuleDirs()` escanea directorios planos. El nuevo sistema debe mantener esta detección pero permitir `module_paths`. | Código actual en `config.go` |
| A2  | El marker del repo es un archivo específico | Hoy es `.dots/config.yaml` o `dots.toml`. El nuevo sistema añade `init.lua`. `IsDotfilesRepo()` debe detectar los 3.      | `config.go:IsDotfilesRepo()` |
| A3  | OS se detecta en runtime                    | `runtime.GOOS` → `linux`/`mac`/`windows`. Se mantiene igual.                                                              | `system.go:DetectOS()`       |
| A4  | `~` se expande a `$HOME`                    | Se mantiene igual vía `system.ExpandPath()`.                                                                              | `system.go:ExpandPath()`     |
| A5  | Paths fuera de `$HOME` se bloquean          | Se mantiene igual vía `system.IsSafePath()`.                                                                              | `system.go:IsSafePath()`     |
| A6  | Dependencias se deduplican por `name`       | Se mantiene igual. Primera definición gana.                                                                               | `install.go`                 |
| A7  | Los módulos no tienen estado interno        | Toda la configuración está en archivos (hoy path.yaml, mañana dots.lua).                                                  | Arquitectura actual          |

### Assumptions NUEVAS del diseño Lua

| #   | Assumption                                                                          | Riesgo                                                  | Mitigación                                     |
| --- | ----------------------------------------------------------------------------------- | ------------------------------------------------------- | ---------------------------------------------- |
| B1  | `gopher-lua` puede cargar dots.lua y devolver una tabla                             | Bajo — API probada                                      | Prototipo en Fase 0                            |
| B2  | Los métodos encadenables (`:when()`, `:to()`) funcionan con metatables Lua desde Go | Medio — metatables en gopher-lua son manuales           | Test unitario en Fase 0                        |
| B3  | La migración path.yaml → dots.lua es determinística                                 | Medio — hay casos ambiguos (ej: directorio como source) | Migrador con `--dry-run` y preview             |
| B4  | El usuario puede escribir Lua sin aprender gopher-lua internals                     | Bajo — la API es pequeña                                | Documentación clara y templates en `dots init` |
| B5  | `module_paths` como field de `RootConfig` que reemplaza la raíz de búsqueda         | Bajo — es solo un string/tabla opcional en el struct    | Test de integración en Fase 2                  |

### Edge-cases identificados

#### E1: Directorio de módulo vacío

- **Qué pasa hoy**: Se ignora (no tiene path.yaml → no es módulo)
- **Qué debe pasar**: Se ignora (no tiene dots.lua → no es módulo)
- **Implementación**: `loader_modules.go` filtra directorios sin dots.lua

#### E2: Directorio con ambos dots.lua y path.yaml

- **Qué pasa hoy**: Solo existe path.yaml
- **Qué debe pasar**: dots.lua tiene prioridad, path.yaml se ignora con warning
- **Implementación**: En loader_modules.go, if both exist → usar Lua, print "[WARN] Module X has both dots.lua and path.yaml. Using dots.lua."

#### E3: Módulos anidados (subdirectorios)

- **Qué pasa hoy**: No existen. GetModuleDirs() solo mira el primer nivel.
- **Qué debe pasar**: Por defecto se escanean recursivamente (find -name dots.lua). init.lua puede limitar con `module_paths`.
- **Implementación**: `loader_modules.go` usa `filepath.Walk` con exclude de `.git`, `.dots`, node_modules.

#### E4: init.lua malformed (syntax error en Lua)

- **Qué debe pasar**: Error claro como el checker de YAML actual — "Syntax error in init.lua: line 3: unexpected symbol near '}'"
- **Implementación**: El VM captura errores de `L.DoFile()` y los reporta con formato estructurado.

#### E5: dots.lua no retorna una tabla

- **Qué debe pasar**: Error: "Module X/dots.lua must return a table, got string"
- **Implementación**: `LoadModuleConfig()` verifica `L.Get(-1)` con type check.

#### E6: dir():to() con source inexistente

- **Qué debe pasar**: Error en link time: "Module X: source directory 'foo' not found"
- **Implementación**: Validación semántica en checker + en link time.

#### E7: dir():into() con directorio vacío

- **Qué debe pasar**: No hace nada, éxito silencioso
- **Implementación**: Simplemente no hay archivos que iterar.

#### E8: glob() sin matches

- **Qué debe pasar**: No hace nada, éxito silencioso
- **Implementación**: `filepath.Glob()` devuelve slice vacío → no hay symlinks que crear.

#### E9: Conflictos de destino entre módulos (misma destination en dos módulos distintos)

- **Qué pasa hoy**: El segundo symlink sobrescribe al primero
- **Qué debe pasar**: Igual que hoy — el último módulo linkeado gana. No hay protección cross-module.
- **Nota**: Esto es intencional. La responsabilidad es del usuario.

#### E10: `require("dots.http")` circular

- **Qué debe pasar**: gopher-lua detecta require circular y da error. No tenemos que implementar nada especial.

#### E11: Plugin no encontrado (`require("nonexistent")`)

- **Qué debe pasar**: Error claro: "Module X: plugin 'nonexistent' not found. Searched: built-in, dots/ in repo"
- **Implementación**: Loader guarda paths buscados y los incluye en el error.

#### E12: `dots adopt` con módulo Lua

- **Qué pasa hoy**: Escribe en path.yaml
- **Qué debe pasar**: Escribe en dots.lua (añade `file()` al array `files`)
- **Implementación**: `module_writer.go` adaptado para detectar formato del módulo.

#### E13: Variants en Lua

- **Qué pasa hoy**: Variants se detectan por múltiples sources → mismo destination en path.yaml
- **Qué debe pasar**: En Lua, variants se declaran igual: múltiples `file()` apuntando al mismo destino. `detect_variants()` funciona sobre `FileOp[]`.
- **Implementación**: La misma lógica de detección pero operando sobre `[]FileOp` en vez de `[]DotFileMapping`.

#### E14: Migración de path.yaml con destino `/*` (expansión implícita de directorio)

- **Qué pasa hoy**: En el resolver, `/*` al final del destino indica "expandir contenidos" (equivalente a `dir():into()`)
- **Qué debe pasar**: `dots migrate` detecta `/*` y genera `dir("source"):into("dest")` en vez de `file()`
- **Implementación**: En `migrate.go`, al parsear entries, si destination termina en `/*` → generar `dir():into()`

#### E15: `dots status` con módulos mixtos (Lua + YAML)

- **Qué debe pasar**: Status muestra ambos tipos. Los módulos Lua se resuelven con su VM, los YAML con su parser.
- **Implementación**: `ResolveModules()` bifurca según tipo de módulo.

#### E16: Checker para módulos Lua

- **Qué debe pasar**: El checker carga el dots.lua en el VM y captura errores de sintaxis/runtime. Además valida semánticamente (source existe? paths seguros?).
- **Implementación**: `checker.go` adaptado con `checkLuaModule()`.

#### E17: `dots init` cuando ya existe `.dots/config.yaml`

- **Qué debe pasar**: Detecta el marker existente y pregunta si migrar a init.lua.
- **Implementación**: `Init()` detecta repo type y ofrece migración.

#### E18: `dots init` en repo vacío

- **Qué debe pasar**: Crea `init.lua` con template general y detecta módulos existentes (con dots.lua o path.yaml).
- **Implementación**: Template de init.lua con comentarios.

#### E19: Transacciones con operaciones de directorio

- **Qué pasa hoy**: `transaction.go` maneja symlink/unlink/mkdir/backup individuales
- **Qué debe pasar**: `dir():to()` (symlink de directorio entero) se registra como un solo symlink. `dir():into()` registra N symlinks individuales.
- **Implementación**: Cada `FileOp` genera su propia secuencia de operaciones atómicas.

#### E20: idempotencia del migrador

- **Qué debe pasar**: Si se migra path.yaml → dots.lua y luego se vuelve a migrar (hipotético round-trip), el dots.lua no cambia.
- **Implementación**: El migrador debe generar Lua canónico (formateado consistentemente).

#### E21: `dots edit --root` para editar `init.lua`

- **Qué pasa hoy**: No existe el concepto. `dots edit` abre el módulo o su path.yaml.
- **Qué debe pasar**: `dots edit --root` abre `init.lua` en el editor. Útil para cambiar `module_paths`, `plugins`, etc.
- **Implementación**: En `edit.go`, flag `--root` que ignora el argumento del módulo y abre `<RepoRoot>/init.lua`.

#### E22: Módulo con directorio pero sin `dots.lua` ni `path.yaml`

- **Qué pasa hoy**: No es un módulo (no tiene path.yaml → ignorado).
- **Qué debe pasar**: Se ignora igual. No es un módulo hasta que tenga `dots.lua`. `dots edit ModuloSinConfig` da error "module not found" como hoy.
- **Implementación**: `FindModules()` filtra directorios sin dots.lua (y sin path.yaml como fallback).

---

## 5. Fases e Hitos

### Fase 0: Runtime Lua embebido + API de files

**DURACIÓN ESTIMADA**: 2 días
**OBJETIVO**: `gopher-lua` integrado en el binario, capaz de cargar un `dots.lua` con `file()/dir()/glob()` y devolver structs Go.

#### Tareas

- [ ] Añadir dependencia: `go get github.com/yuin/gopher-lua`
- [ ] Crear `internal/lua/types.go`:
  - `ModuleConfig`, `RootConfig`, `FileOp`, `DepOp`, `FileOpType`
- [ ] Crear `internal/lua/vm.go`:
  - `NewLuaVM() (*lua.LState, error)`
  - `LoadModuleConfig(path string) (*ModuleConfig, error)`
  - `LoadRootConfig(path string) (*RootConfig, error)`
- [ ] Implementar registro de API de files en Go (api_files.go):
  - `file(source, dest)` → table con metatable `__index` para `:when(os)`, `:per_os(table)`
  - `dir(source)` → objeto con `:to(dest)`, `:into(dest)`
  - `glob(pattern)` → objeto con `:into(dest)`
- [ ] Implementar parseModuleConfig() que convierte tabla Lua → ModuleConfig Go
- [ ] Tests:
  - Carga de `dots.lua` válido y verifica structs Go
  - Error en Lua malformed
  - `:when("linux")` filtra correctamente
  - `:per_os({linux="...", mac="..."})` devuelve destinos correctos
  - `dir():to()` vs `dir():into()` producen FileOpType distintos
  - `glob()` produce FileOpGlob

#### Edge-cases cubiertos en tests

- [ ] E4: Lua malformed → error estructurado
- [ ] E5: dots.lua no retorna tabla → error
- [ ] E6: `dir()` con source inexistente (validación semántica en checker, no en parse)
- [ ] E7: `dir():into()` vacío → sin errores

#### Criterio de Aceptación

- [ ] `go test ./internal/lua/...` pasa
- [ ] VM carga dots.lua con file(), dir(), glob() y devuelve datos correctos

---

### Fase 1: API de dependencias + plugin loader

**DURACIÓN ESTIMADA**: 2 días
**OBJETIVO**: Registro de funciones `pkg()`, `curl()`, `git()` en el VM Lua, y loader de plugins vía `require()`.

#### Tareas

- [ ] Implementar `internal/lua/api_deps.go`:
  - `pkg(name)` → objeto con `:on(table)`, `:fallback(dep)`, `:post(cmd)`, `:bin(name)`
  - `curl(url)` → objeto con `:extract(path)`, `:to(dest)`, `:version(v)`, `:arch(table)`
  - `git(url)` → objeto con `:to(dest)`, `:at(ref)`, `:post(cmd)`
- [ ] Implementar `internal/lua/loader.go`:
  - `RegisterPluginLoader(vm, builtinPlugins)`
  - Plugins built-in: `dots.http`, `dots.archive`, `dots.git`
  - Búsqueda: built-in → `dots/` en repo → error
  - Plugins built-in son strings Lua empaquetados con `//go:embed`
- [ ] Crear `internal/lua/plugins/http.lua`, `archive.lua`, `git.lua`
- [ ] Tests:
  - `pkg("ripgrep")` → DepOp package
  - `pkg("fd"):on({pacman="fd"})` → managers map
  - `curl(...):extract("binary"):to("~/.local/bin/x")` → DepOp completo
  - `git(...):to("~/plugins/p10k"):at("v1.19.0")` → DepOp git con ref
  - `require("dots.http")` carga plugin built-in
  - E10: require circular → error
  - E11: require inexistente → error con paths

#### Criterio de Aceptación

- [ ] `go test ./internal/lua/...` pasa
- [ ] Todos los métodos encadenables funcionan
- [ ] Plugin loader resuelve built-in y filesystem

---

### Fase 2: Config raíz (`init.lua`) + loader de módulos

**DURACIÓN ESTIMADA**: 1 día
**OBJETIVO**: `dots` detecta `init.lua` como marker, carga módulos, y coexiste con YAML.

#### Tareas

- [ ] Modificar `internal/config/config.go`:
  - `IsDotfilesRepo()` detecta también `init.lua`
  - Nuevo tipo `RepoType`: `RepoTypeYAML`, `RepoTypeLua`, `RepoTypeLegacy`
  - `IsLuaRepo(path) bool` — busca `init.lua`
  - Adaptar `Load()` para devolver tipo de repo
- [ ] **Añadir field `ModulePaths` al struct `RootConfig`**: string o `[]string` opcional. Se lee directamente de la tabla Lua retornada por `init.lua`. No se necesita función global ni variable preludio.
- [ ] Implementar `internal/lua/loader_modules.go`:
  - `FindModules(repoRoot string, initCfg *RootConfig) ([]ModuleDir, error)` — escanea módulos
  - Por defecto: `filepath.Walk` buscando `dots.lua` en la raíz del repo (skip .git, .dots, node_modules)
  - Si `initCfg.ModulePaths` tiene rutas: busca SOLO ahí, no en la raíz
  - Si módulo tiene path.yaml y NO dots.lua → YAML mode (coexistencia)
  - Si módulo tiene ambos → prioridad dots.lua con warning
- [ ] Modificar `internal/config/config.go:GetModuleDirs()`:
  - Si repo es Lua, usa `FindModules()` en vez de scan de path.yaml
  - Si repo es YAML, comportamiento actual
- [ ] Tests:
  - E1: Directorio vacío se ignora
  - E2: Ambos dots.lua + path.yaml → prioridad Lua con warning
  - E3: Módulos anidados se detectan recursivamente
  - Repo con init.lua se detecta como Lua
  - Repo sin init.lua pero con `.dots/config.yaml` sigue funcionando

#### Criterio de Aceptación

- [ ] `go test ./...` pasa
- [ ] Repo con `init.lua` funciona como dotfiles repo
- [ ] Módulos Lua y YAML coexisten en `dots status`

---

### Fase 3a: Resolver + comandos de solo lectura (status, list)

**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: El resolver soporta módulos Lua y YAML. `dots status` y `dots list` funcionan con ambos.

#### Tareas

- [ ] Adaptar `internal/resolver/resolver.go`:
  - `ResolveModules()` maneja `FileOp` (Lua) además de `DotFileMapping` (YAML)
  - `GetModuleVariantInfo()` adaptado para `[]FileOp`
  - `GetActiveVariant()` adaptado para módulos Lua
  - `resolveModuleMappings()` bifurca según tipo de módulo
  - Las funciones de abajo (`resolveSingleState`, `shortPath`) se reutilizan sin cambios
- [ ] Modificar `internal/config/config.go:GetModuleDirs()`:
  - Bifurca internamente según `RepoType`: si es Lua, llama a `FindModules()`; si es YAML, mantiene el scan de path.yaml
  - Así el TUI selector y el checker no necesitan saber el formato — es transparente
- [ ] Adaptar `internal/cli/status.go` y `list.go`:
  - Usan `ModuleConfig` de Lua (resuelto vía el resolver adaptado)
  - No necesitan cambios estructurales — solo funcionan con los nuevos tipos de datos
- [ ] Tests:
  - E13: Variant detection sobre `[]FileOp`
  - E15: Módulos mixtos Lua + YAML en status
  - `dots status` con módulo Lua (file, dir:to, dir:into, glob)
  - `dots status` con módulo YAML (coexistencia)

#### Criterio de Aceptación

- [ ] `go test ./...` pasa
- [ ] `dots status` funciona con módulos Lua y YAML simultáneamente
- [ ] `dots list` funciona con módulos Lua
- [ ] Variants se detectan correctamente en módulos Lua
- [ ] `GetModuleDirs()` funciona transparentemente para ambos formatos

---

### Fase 3b: Comandos de mutación (link, install)

**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: `dots link` y `dots install` funcionan con módulos Lua.

**⚠️ Prerrequisito**: Fase 1 debe estar completa (plugins built-in http, archive, git funcionales, no solo el loader).

#### Tareas

- [ ] Adaptar `internal/cli/link.go`:
  - `runLink()` usa el resolver adaptado (transparente)
  - Soporta `FileOpDirTo`: symlink del directorio entero (source → destination)
  - Soporta `FileOpDirInto`: por cada archivo dentro del source, crea symlink individual en destination
  - Soporta `FileOpGlob`: expande glob y crea symlinks individuales
  - Variant selection y auto-swap funcionan con módulos Lua
- [ ] Adaptar `internal/cli/install.go`:
  - Usa `DepOp` de Lua para resolver dependencias
  - Plugins built-in (http, archive, git) ejecutan la instalación
  - La deduplicación por `name` funciona igual que hoy
- [ ] Tests:
  - E6: `dir():to()` con source inexistente → error
  - E7: `dir():into()` vacío → sin error
  - E8: `glob()` sin matches → sin error
  - E9: destinos duplicados entre módulos → se comporta como hoy
  - E19: Transacciones con dir():to() y dir():into()

#### Criterio de Aceptación

- [ ] `go test ./...` pasa
- [ ] `dots link` funciona con módulos Lua (file, dir:to, dir:into, glob)
- [ ] `dots install` con dependencias Lua
- [ ] Variants funcionan con módulos Lua
- [ ] Comandos de mutación y solo lectura coexisten sin duplicación

---

### Fase 4: Comandos restantes (init, adopt, edit, backup) + checker

**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: Todos los comandos funcionan con dots.lua. Checker adaptado.

#### Tareas

- [ ] Adaptar `internal/cli/init.go`:
  - `runInit()` crea `init.lua` con template
  - E17: Si existe `.dots/config.yaml`, ofrece migrar
  - E18: En repo vacío, crea init.lua con template
  - Detecta módulos existentes (path.yaml y dots.lua) y los incluye en init.lua
- [ ] Adaptar `internal/cli/adopt.go`:
  - E12: Detecta si el módulo es Lua o YAML
  - Si es Lua: escribe `file()` en dots.lua
  - Si es YAML: comportamiento actual
- [ ] Adaptar `internal/cli/edit.go`:
  - `--config` abre `dots.lua` en vez de `path.yaml` para módulos Lua
  - E21: `--root` abre `<RepoRoot>/init.lua` en el editor (ignora el argumento del módulo)
  - E22: Módulo sin dots.lua ni path.yaml → error "module not found"
- [ ] Adaptar `internal/checker/checker.go`:
  - E16: `checkLuaModule()` — carga dots.lua en VM, captura errores
  - Validación semántica: source existe? destinations seguras? deps válidas?
  - Deprecation warning para path.yaml
- [ ] Tests:
  - `dots init` crea init.lua correcto
  - `dots adopt` escribe en dots.lua
  - Checker reporta errores de Lua
  - YAML muestra deprecation warning

#### Criterio de Aceptación

- [ ] `go test ./...` pasa
- [ ] `dots init` crea init.lua
- [ ] `dots adopt` modifica dots.lua
- [ ] Checker valida módulos Lua y YAML
- [ ] YAML muestra deprecation warning

---

### Fase 5: Comando `migrate` (YAML → Lua) + limpieza

**DURACIÓN ESTIMADA**: 2 días
**OBJETIVO**: Tooling para migrar path.yaml → dots.lua automáticamente.

#### Tareas

- [ ] Implementar migración en `internal/lua/migrate.go`:
  - `MigrateModule(modPath string) error` — lee path.yaml, genera dots.lua
  - `MigrateRootConfig(repoRoot string) error` — migra `.dots/config.yaml` a `init.lua`
  - Mapeo completo de fields (ver tabla en 5.4)
  - E14: `/*` en destination → `dir():into()`
  - E20: Output canónico, idempotente
- [ ] Integrar en `internal/cli/migrate.go`:
  - `dots migrate --to-lua [modules...]`
  - `--dry-run` preview
  - `--all` todos los módulos
- [ ] Limpieza de código YAML:
  - Eliminar `internal/yaml/`, `internal/template/`
  - Eliminar `gopkg.in/yaml.v3` de go.mod
- [ ] Tests:
  - E13: Migración con variants
  - E14: Migración con `/*` destination
  - E20: Idempotencia del output
  - Todas las variantes sintácticas de path.yaml

#### Criterio de Aceptación

- [ ] `go test ./...` pasa
- [ ] `dots migrate --to-lua --all` migra todos los módulos
- [ ] Módulo migrado funciona con `dots link`
- [ ] YAML parser eliminado del binario

---

### Fase 6: Testing integral + documentación

**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: Cobertura de tests, edge cases, documentación.

#### Tareas

- [ ] Tests de integración:
  - Todos los edge-cases E1-E20
  - Módulos mixtos Lua + YAML (E15)
  - Transacciones con operaciones de directorio (E19)
  - Cross-module dedup de dependencias
- [ ] Documentación:
  - `docs/dots-lua-reference.md`
  - `docs/init-lua-reference.md`
  - `docs/migration-guide.md`
  - `docs/plugin-system.md`
  - Actualizar README.md y AGENTS.md
- [ ] Benchmarks: Lua vs YAML startup time

#### Criterio de Aceptación

- [ ] `go test ./...` pasa al 100%
- [ ] Cobertura >80% en `internal/lua/`
- [ ] Documentación completa

---

### Fase 7: Release v1.0.0

**DURACIÓN ESTIMADA**: 0.5 días
**OBJETIVO**: Release con el nuevo sistema Lua.

#### Tareas

- [ ] Bump version a "1.0.0"
- [ ] Release notes
- [ ] Tag + GitHub release

---

## 6. Especificaciones Técnicas Detalladas

### 6.1 Estructura de archivos nuevos

```
internal/
└── lua/
    ├── vm.go              # NewLuaVM, LoadModuleConfig, LoadRootConfig
    ├── types.go           # ModuleConfig, RootConfig, FileOp, DepOp structs
    ├── api_files.go       # file(), dir(), glob() — funciones Go registradas en Lua
    ├── api_deps.go        # pkg(), curl(), git() — funciones Go registradas en Lua
    ├── api_helpers.go     # :when(), :to(), :into(), :per_os(), :on(), etc.
    ├── loader.go          # require() — plugin loader (built-in + filesystem)
    ├── loader_modules.go  # FindModules() — escanea módulos con dots.lua
    ├── migrate.go         # path.yaml → dots.lua converter (Fase 5)
    └── plugins/
        ├── http.lua       # Plugin HTTP built-in
        ├── archive.lua    # Plugin tar/zip built-in
        └── git.lua        # Plugin git clone built-in
```

### 6.2 Migraciones de datos

#### path.yaml → dots.lua (mapeo completo)

| YAML (path.yaml)                                                                 | Lua (dots.lua)                                            |
| -------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `type: minimal`                                                                  | `type = "minimal"`                                        |
| `files: [{source: x, destination: y}]`                                           | `files = { file("x", "y") }`                              |
| `files: [{source: x, destination: y, os: [linux]}]`                              | `files = { file("x", "y"):when("linux") }`                |
| `files: [{source: x, destination: y, per-os: {linux: a, mac: b}}]`               | `files = { file("x", "y"):per_os({linux="a", mac="b"}) }` |
| `files: [{source: dir, destination: "~/.config/x/*"}]` (expansión de directorio) | `files = { dir("dir"):into("~/.config/x") }`              |
| Directorio como source                                                           | `dir("x"):to("dest")` o `dir("x"):into("dest")`           |
| `dependencies: [git]`                                                            | `dependencies = { pkg "git" }`                            |
| `dependencies: [{name: x, type: package, managers: {pacman: y}}]`                | `dependencies = { pkg("x"):on({pacman="y"}) }`            |
| `dependencies: [{name: x, type: binary, url: u, dest: d, extract: e}]`           | `dependencies = { curl("u"):extract("e"):to("d") }`       |
| `dependencies: [{name: x, type: git, url: u, dest: d, ref: r}]`                  | `dependencies = { git("u"):to("d"):at("r") }`             |
| `fallback: {type: binary, ...}`                                                  | `:fallback(curl("..."))`                                  |
| `bin: batcat`                                                                    | `pkg("bat"):bin("batcat"):on({apt="bat"})`                |
| Variants (múltiples sources → mismo dest)                                        | Múltiples `file()` → mismo destino (se mantiene igual)    |

---

## 7. Testing

| Tipo                       | Cobertura                                          | Edge-cases                                |
| -------------------------- | -------------------------------------------------- | ----------------------------------------- |
| **Unit tests**             | lua/vm.go, lua/api_files, lua/api_deps, lua/loader | E4, E5, E10, E11                          |
| **Integration tests**      | Carga + ejecución link/install                     | E1, E2, E3, E6, E7, E8, E9, E12, E15, E19 |
| **Migration tests**        | path.yaml → dots.lua                               | E13, E14, E20                             |
| **Coexistence tests**      | Lua + YAML mismo repo                              | E15                                       |
| **Checker tests**          | Validación Lua y YAML                              | E16                                       |
| **Smoke tests**            | Todos los comandos                                 | E17, E18                                  |
| **Module discovery tests** | `module_paths`, auto-detect, múltiples rutas       | E22                                       |

---

## 8. Plan de Rollback

1. **Detección**: Error en comando crítico con módulos Lua
2. **Acción**: `git revert HEAD` en dev → publicar hotfix v0.11.1
3. **Tiempo**: ~10 minutos

---

## 9. Riesgos y Mitigaciones

| Riesgo                                         | Prob  | Impacto | Mitigación                              |
| ---------------------------------------------- | ----- | ------- | --------------------------------------- |
| gopher-lua metatables para encadenamiento      | Media | Alto    | Prototipar API en Fase 0                |
| Coexistencia YAML+Lua introduce bugs           | Alta  | Medio   | Tests de integración con ambos formatos |
| path.yaml complejos (variants, fallbacks, /\*) | Alta  | Medio   | Migrador debe cubrir todos los casos    |
| Plugins built-in no son flexibles              | Media | Bajo    | Son Lua, el usuario puede overridearlos |
| `dir():to()` con source inexistente            | Media | Bajo    | Validación en checker + link time       |

---

## 10. Criterios de Éxito Globales

- [ ] `dots` funciona con repositorio basado en `init.lua` + `dots.lua`
- [ ] Todos los comandos existentes funcionan idéntico con el nuevo formato
- [ ] `path.yaml` → `dots.lua` migración automática funcional con `--dry-run`
- [ ] Operaciones de directorio (`dir():to()`, `dir():into()`) funcionan
- [ ] Plugins vía `require()` cargan correctamente
- [ ] Edge-cases E1-E20 cubiertos en tests
- [ ] `go test ./...` pasa al 100%
- [ ] Coexistencia con YAML durante migración

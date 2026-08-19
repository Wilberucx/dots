# init.lua Unificado — Spec de Implementación

> **Versión**: 0.1 — Julio 2026
> **Rama**: `feature/unified-init`
> **Estado**: ⏳ Planificación — debate arquitectónico completado
> **Release target**: v0.16.0

---

## Resumen Ejecutivo

Transformar `init.lua` de un archivo de solo-metadata a un **entry point único** capaz de:

1. Declarar `files` y `dependencies` directamente (config simple en 1 archivo)
2. Modularizar vía `require("nombre")` (archivos sueltos o directorios con init.lua)
3. Auto-descubrir módulos vía `require("directorio/")` (FindModules() como helper interno)
4. Componer todo con `dots.merge()` (helper explícito, no magia)

**No hay tres mecanismos separados. Hay uno solo:** `init.lua` ejecuta Lua, construye una tabla con `{ files = {...}, dependencies = {...} }`, y eso es todo.

---

## Filosofía

```
Neovim:   init.lua → require("mod") → require("otro") → return { ... }
dots:     init.lua → require("mod") → require("dir/") → return dots.merge({...})
```

`dots` extiende el `require()` de Lua para que:
- `require("zsh")` → busca `zsh.lua` o `zsh/init.lua` (user module)
- `require("packages/")` → detecta trailing `/`, ejecuta `FindModules()`, devuelve mapa
- `require("dots.http")` → busca plugin (con `dots.` prefix)

**FindModules() deja de ejecutarse automáticamente.** Solo se invoca cuando
`init.lua` hace `require("directorio/")`.

---

## Fases de Implementación

Las fases están ordenadas de **menor a mayor impacto**, para probar cada
cambio antes de avanzar al siguiente.

---

### Fase 1: RootConfig acepta Files/Dependencies

> **Impacto**: 🟢 Bajo
> **Archivos**: 4
> **Dependencias**: Ninguna

#### Cambios

**`internal/lua/types.go`**
```go
// RootConfig ahora embebe ModuleConfig para heredar Files y Dependencies
type RootConfig struct {
    Name        string
    ModulePaths []string
    Plugins     []string
    Output      *OutputConfig
    ModuleConfig              // ← NUEVO: embed: hereda Files, Dependencies, Type
}
```

**`internal/lua/vm.go` — parseRootConfig()**
```go
func parseRootConfig(tbl *lua.LTable) (*RootConfig, error) {
    cfg := &RootConfig{
        Name: lvToString(tbl.RawGetString("name")),
    }
    // ... module_paths, plugins, output (existe) ...

    // NUEVO: reusar parseModuleConfig para files/dependencies
    modCfg := parseModuleConfig(tbl)
    cfg.Files = modCfg.Files
    cfg.Dependencies = modCfg.Dependencies
    if cfg.Name == "" {
        cfg.Name = "dotfiles"
    }
    return cfg, nil
}
```

**`internal/config/config.go` — RootConfig mirror (SIN import circular)**

> ⚠️ `config` NO puede importar `luacfg` (evitar circular imports).
> Definir structs ligeras directamente en `config/config.go`:

```go
// FileOpConfig es una versión liviana de luacfg.FileOp para config.
// Evita import circular: config → lua no está permitido.
type FileOpConfig struct {
    Type        int               // FileOpFile, FileOpDirTo, etc.
    Source      string
    Destination string
    Pattern     string
    OSFilter    string
    PerOS       map[string]string
    VariantName string
    Module      string
}

type DepOpConfig struct {
    Name        string
    Type        string
    URL         string
    Destination string
    // ... campos necesarios
}

type RootConfig struct {
    Name        string
    ModulePaths []string
    Plugins     []string
    Output      *OutputConfig
    Files       []FileOpConfig   // ← NUEVO (struct liviana, sin import circular)
    Dependencies []DepOpConfig   // ← NUEVO
}
```

**Traducción luacfg → config en `cli/root.go`:**
```go
// helper para convertir
func toConfigFileOps(src []luacfg.FileOp) []config.FileOpConfig {
    dst := make([]config.FileOpConfig, len(src))
    for i, f := range src {
        dst[i] = config.FileOpConfig{
            Type:        int(f.Type),
            Source:      f.Source,
            Destination: f.Destination,
            // ...
        }
    }
    return dst
}
```

**`internal/cli/root.go` — propagación initCfg → config**
```go
internalCfg := &config.RootConfig{
    Name:        initCfg.Name,
    ModulePaths: initCfg.ModulePaths,
    Plugins:     initCfg.Plugins,
    Files:       initCfg.Files,  // ← NUEVO
    Deps:        initCfg.Dependencies,  // ← NUEVO
}
```

#### Tests
- init.lua con `return { files = { file("x", "~/.x") } }` → RootConfig.Files no vacío
- init.lua sin files → RootConfig.Files nil (backward compat)
- init.lua con `return { dependencies = { pkg "ripgrep" } }` → RootConfig.Deps no vacío
- init.lua con `return { name = "test" }` (sin files) → funciona igual que hoy

#### Criterio de éxito
- `go test ./internal/lua/...` pasa
- `go build ./...` compila
- Un init.lua con files directos no rompe nada existente

---

### Fase 2: Extender require() para user modules

> **Impacto**: 🟡 Moderado
> **Archivos**: 1-2 (loader.go + tests)
> **Dependencias**: Fase 1

#### Cambios

**`internal/lua/loader.go`**

> ⚠️ **Plugin loading order:** Los plugins declarados vía `return { plugins = {...} }`
> NO pueden cargarse antes de ejecutar init.lua (el return es el resultado).
> **Solución:** init.lua carga plugins vía `require("dots.http")` EXPLÍCITO.
> El campo `plugins` en init.lua se depreca (WARN si se usa).
> Los plugins se declaran con `require()` donde se necesiten.

Renombrar `RegisterPluginLoader` → `RegisterLoader` (más genérico).
Extender el search path del `require()`:

```
require("x") — SIN dots. prefix, SIN trailing /
  1. <repo_root>/x.lua              — user module file
  2. <repo_root>/x/init.lua         — user module directory
  3. <repo_root>/dots/x.lua         — custom plugin (FALLBACK con WARN)
  4. Error

require("dots.x") — CON dots. prefix
  1. Built-in: plugins/x.lua        — built-in plugin
  2. <repo_root>/dots/x.lua         — custom plugin
  3. Error

require("x/") — CON trailing /      (Fase 3, preparar stub aquí)
  → Auto-discovery (FindModules)
```

**WARN para fallback:**
```
[WARN] require("helpers") resolved as plugin in dots/helpers.lua.
  → Use require("dots.helpers") for plugins.
  → This fallback will be removed in v0.20.0.
```

**Flujo de carga de init.lua actualizado:**
```go
func loadLuaInitConfig(repoRoot string) (*luacfg.RootConfig, error) {
    vm := luacfg.NewLuaVM()
    defer vm.Close()

    // 1. Registrar loader para user modules + plugins
    //    Esto permite require() dentro de init.lua
    luacfg.RegisterLoader(vm.L, repoRoot)

    // 2. Cargar init.lua
    //    Dentro, el usuario hace require("dots.http") explícito si necesita plugins
    initPath := filepath.Join(repoRoot, "init.lua")
    cfg, err := vm.LoadRootConfig(initPath)

    // 3. Cargar plugins del return (solo para compat con módulos dots.lua)
    //    WARN si se usa: "use require() explícito en init.lua en vez de plugins = {}"
    if cfg != nil && len(cfg.Plugins) > 0 {
        fmt.Fprintf(os.Stderr, "[WARN] plugins field in init.lua is deprecated.\n")
        LoadModulePlugins(vm, cfg, repoRoot)
    }

    return cfg, err
}
```

#### Tests
- `require("zsh")` con `zsh.lua` existente → carga correctamente
- `require("zsh")` con `zsh/init.lua` existente → carga correctamente
- `require("dots.http")` → carga built-in plugin (sin cambios)
- `require("helpers")` con solo `dots/helpers.lua` → fallback con WARN
- `require("inexistente")` → error claro con paths buscados

#### Criterio de éxito
- Los tests de loader.go existentes siguen pasando (built-in plugins)
- `dots doctor` con init.lua que tiene require funciona

---

### Fase 3: require("dir/") con auto-discovery

> **Impacto**: 🟡 Moderado
> **Archivos**: 3-4
> **Dependencias**: Fase 2

#### Cambios

**`internal/lua/loader.go` — detectar trailing `/`**

Cuando `require("packages/")`:
1. Detectar que el nombre termina en `/`
2. Quitar el trailing `/`
3. Resolver el path absoluto: `<repo_root>/packages/`
4. Ejecutar `FindModules()` con ese directorio como search root
5. Para cada módulo encontrado: cargar su `dots.lua` → `ModuleConfig`
6. Devolver MAPA: `{ Zsh = { files = {...} }, Nvim = { files = {...} } }`

**Problema:** `require()` en Lua debe devolver un valor. Si el directorio
está vacío o no existe, debe devolver una tabla vacía o error.

**Comportamiento:**
- Directorio no existe → error: `require("x/"): directory not found`
- Directorio existe pero sin subdirectorios con dots.lua → tabla vacía `{}`
- Directorio con módulos → mapa con nombre → ModuleConfig

**`internal/lua/types.go` — Module string en FileOp y LinkStatus**

```go
// FileOp — en el paquete lua
type FileOp struct {
    Type        FileOpType
    Source      string
    Destination string
    Pattern     string
    OSFilter    string
    PerOS       map[string]string
    VariantName string
    Module      string  // ← NUEVO: nombre del módulo de origen
}
```

**`internal/resolver/resolver.go` — LinkStatus también necesita Module:**

```go
type LinkStatus struct {
    Source       string
    Destination  string
    State        LinkState
    Detail       string
    BackupPath   string
    ConfigSource string
    ConfigDest   string
    OpType       string
    Module       string // ← NUEVO: para agrupar en la UI
}
```

**`internal/lua/vm.go` — parser etiqueta FileOps**

```go
// parseModuleConfig con nombre de módulo
func parseModuleConfig(tbl *lua.LTable, moduleName string) (*ModuleConfig, error) {
    cfg := &ModuleConfig{}
    // ... parse type, files, dependencies ...
    // Etiquetar cada FileOp con el módulo
    for i := range cfg.Files {
        cfg.Files[i].Module = moduleName
    }
    return cfg, nil
}
```

**Registrar `dots.merge()` helper en el VM:**

```lua
-- dots.merge() toma un mapa de módulos y devuelve { files, dependencies }
-- Cada FileOp queda etiquetado con su módulo de origen
function dots.merge(modules)
  local result = { files = {}, dependencies = {} }
  for name, mod in pairs(modules) do
    for _, f in ipairs(mod.files or {}) do
      f.module = name  -- etiqueta
      table.insert(result.files, f)
    end
    for _, d in ipairs(mod.dependencies or {}) do
      table.insert(result.dependencies, d)
    end
  end
  return result
end
```

**Implementación Go del helper:**

`internal/lua/vm.go` — en `NewLuaVM()`:
```go
L.SetGlobal("dots", vm.L.NewTable())
// dots.merge
dotsTable := vm.L.NewTable()
dotsTable.RawSetString("merge", L.NewFunction(vm.apiDotsMerge))
L.SetGlobal("dots", dotsTable)
```

#### Tests
- `require("modules/")` con 2 módulos → mapa con 2 entries
- `require("modules/")` con 0 módulos → tabla vacía
- `dots.merge()` combina correctamente
- FileOp.Module se setea correctamente después de merge
- init.lua directo (sin require) → FileOp.Module = cfg.Name

#### Criterio de éxito
- `dots status` muestra módulos agrupados correctamente (Module tag preservado)
- init.lua con `return dots.merge({ require("mods/") })` funciona
- `go test ./internal/...` pasa

---

### Fase 4: Resolver procesa RootConfig.Files

> **Impacto**: 🟡 Moderado
> **Archivos**: 2-3
> **Dependencias**: Fase 1, Fase 3

#### Cambios

**NOTA:** init.lua NO es un "pseudo módulo". Los files de init.lua se
mergean en el resultado final sin crear un módulo artificial. Cada FileOp
ya tiene su `Module` tag (seteado en Fase 3) que identifica su origen.

**`internal/resolver/resolver.go` — ResolveModules**

Después de procesar módulos descubiertos por `FindModules()`, procesar
`cfg.InitCfg.Files` (si existen) y agregarlos a los resultados existentes.
Si el FileOp tiene `Module` tag, se usa ese; si no, se usa el nombre del repo.

```go
func ResolveModules(cfg, modules, types, variant) (map[string][]LinkStatus, error) {
    results := make(map[string][]LinkStatus)

    // 1. Módulos descubiertos (FindModules) — flujo existente
    modDirs, _ := cfg.GetModuleDirs(modules, types)
    for _, mod := range modDirs {
        // ... exactamente como hoy ...
    }

    // 2. Files directos de init.lua — NUEVO
    //    NO se crea un pseudo-módulo. Cada FileOp ya tiene Module tag
    //    (seteado en Fase 3: parser etiqueta FileOp con su origen)
    if cfg.InitCfg != nil && len(cfg.InitCfg.Files) > 0 {
        for _, f := range cfg.InitCfg.Files {
            moduleName := f.Module
            if moduleName == "" {
                moduleName = cfg.InitCfg.Name // "dotfiles" por defecto
            }
            // Usar resolveLuaFileOp directamente con el FileOp
            rootMod := config.ModuleDir{
                Name: moduleName,
                Path: cfg.RepoRoot,
            }
            homeDir := system.HomeDir()
            sts := resolveLuaFileOp(cfg, rootMod, toLuaFileOp(f), homeDir)
            results[moduleName] = append(results[moduleName], sts...)
        }
    }

    return results, nil
}
```

**HELPER: convertir config.FileOpConfig → luacfg.FileOp**
```go
func toLuaFileOp(f config.FileOpConfig) luacfg.FileOp {
    return luacfg.FileOp{
        Type:        luacfg.FileOpType(f.Type),
        Source:      f.Source,
        Destination: f.Destination,
        Pattern:     f.Pattern,
        OSFilter:    f.OSFilter,
        PerOS:       f.PerOS,
        VariantName: f.VariantName,
        Module:      f.Module,
    }
}
```

#### Tests
- init.lua con `files = { file("a", "~/.a") }` → aparece bajo el nombre del repo
- init.lua + módulos descubiertos → ambos aparecen
- init.lua con `return dots.merge({ require("mods/") })` → módulos con sus nombres

#### Criterio de éxito
- `dots link` procesa files de init.lua
- `dots status` muestra todo agrupado (Module tag preservado)
- `dots plan` incluye acciones de init.lua files
- Sin cambios en salida para configs existentes

---

### Fase 5: dots adopt actualizado

> **Impacto**: 🔴 Alto (toca CLI adopt)
> **Archivos**: 2-3
> **Dependencias**: Fase 4

#### Cambios

**`internal/cli/adopt.go` — resolución de destino**

```
dots adopt ~/.gitconfig

Orden de prioridad para decidir dónde adoptar:
  1. --target flag explícito → usar ese directorio
  2. init.lua con require("X/") único → adoptar dentro de X/
  3. init.lua con múltiples require("X/") → preguntar interactivamente
  4. init.lua sin requires de directorio → appendea file() a init.lua
```

**Implementación (SIN análisis estático):**

> ⚠️ `extractDirRequires` con regex es frágil. En vez de parsear init.lua
> estáticamente, **ejecutar init.lua y trackear requires en runtime**:
> 1. Cargar init.lua (con el loader registrado)
> 2. El loader, cuando resuelve `require("algo/")`, registra el directorio
> 3. Después de ejecutar init.lua, consultar los directorios registrados

```go
var dirRequires []string  // package-level, set por el loader

func RegisterLoader(L *lua.LState, repoRoot string) {
    // ... mismo loader de Fase 2/3 ...
    // Pero adicionalmente:
    //  - Si el require termina en /, agregar a dirRequires
}

func resolveAdoptTarget(cfg *config.DotsConfig, targetFlag string) (string, error) {
    if targetFlag != "" {
        return targetFlag, nil
    }
    // dirRequires ya fue poblado cuando se ejecutó init.lua en loadConfig()
    switch len(dirRequires) {
    case 0:
        return "", nil  // appendea a init.lua
    case 1:
        return dirRequires[0], nil
    default:
        return ui.RunPicker("¿Dónde adoptar?", dirRequires)
    }
}
```

**Adoptar en init.lua (target vacío):** Appendea un `file()` al return:

```go
func appendFileToInitLua(initPath, source, destination string) error {
    // Similar a appendLuaFileEntry() pero para init.lua
    // Busca "return {" y agrega file() dentro de files = { ... }
    // Si no existe files = { ... }, lo crea
}
```

**Nuevo flag:**
```go
adoptCmd.Flags().StringP("target", "t", "", "Target directory for adoption (e.g. packages/)")
```

#### Tests
- `dots adopt --target packages/ ~/.config` → crea packages/Nombre/ con dots.lua
- `dots adopt ~/.config` con init.lua que tiene require("packs/") → adopt en packs/
- `dots adopt ~/.config` con init.lua SIN requires → appendea file() a init.lua
- `dots adopt --dry-run` → muestra dónde adoptaría sin ejecutar

#### Criterio de éxito
- `dots adopt` siempre sabe dónde escribir (no pregunta en caso no-ambiguo)
- init.lua no se rompe sintácticamente después de appendFileToInitLua

---

### Fase 6: Checker y doctor extendidos

> **Impacto**: 🟡 Moderado
> **Archivos**: 1-2
> **Dependencias**: Fase 1

#### Cambios

**`internal/checker/checker.go`**

Hoy valida `dots.lua` files de cada módulo. Extender para validar init.lua:

```go
func RunSyntaxCheck(cfg, opts) *Result {
    result := &Result{}

    // 1. Validar módulos descubiertos (existe)
    // ...

    // 2. NUEVO: validar files directos de init.lua
    if cfg.InitCfg != nil && len(cfg.InitCfg.Files) > 0 {
        for _, f := range cfg.InitCfg.Files {
            srcPath := filepath.Join(cfg.RepoRoot, f.Source)
            if _, err := os.Stat(srcPath); os.IsNotExist(err) {
                result.AddError("init.lua", ErrSourceMissing, f.Source)
            }
        }
    }

    return result
}
```

**`internal/cli/doctor.go`**

Agregar chequeo:
- init.lua tiene files/deps? → mostrarlos en doctor
- init.lua tiene `module_paths`? → WARN de deprecación
- init.lua usa require("algo") sin dots. prefix para plugin? → WARN

#### Tests
- init.lua con file() apuntando a source que no existe → checker reporta error
- init.lua válido con files → checker no reporta error
- `dots doctor` con module_paths → muestra WARN

#### Criterio de éxito
- `dots link` bloquea si init.lua reference source files que no existen
- `dots doctor` es informativo

---

### Fase 7: Timeout, error handling, deprecación, migrate

> **Impacto**: 🟡 Moderado
> **Archivos**: 4-5
> **Dependencias**: Fase 2, Fase 3

#### Cambios

**Timeout en init.lua**

```go
func (vm *LuaVM) LoadRootConfig(path string, timeout time.Duration) (*RootConfig, error) {
    type result struct {
        cfg *RootConfig
        err error
    }
    ch := make(chan result, 1)
    go func() {
        cfg, err := vm.loadRootConfigInternal(path)
        ch <- result{cfg, err}
    }()
    select {
    case r := <-ch:
        return r.cfg, r.err
    case <-time.After(timeout):
        vm.L.Close()
        return nil, fmt.Errorf("init.lua execution timed out after %v", timeout)
    }
}
```

**Default:** 5 segundos.

> ⚠️ `exec_timeout` NO se configura desde init.lua (catch-22: si timeout,
> no llegas a parsear exec_timeout).
> Configurable vía:
> - Variable de entorno: `DOTS_INIT_TIMEOUT=10`
> - Flag CLI: `dots --init-timeout 10 status`

**Mejores errores en require():**

```go
case 3:
    // User module file not found
    searchedPaths := fmt.Sprintf(
        "%s.lua, %s/init.lua",
        filepath.Join(repoRoot, name),
        filepath.Join(repoRoot, name),
    )
    L.RaiseError("require(%q) failed: file not found\n  Searched: %s\n  → Create the file or use 'dots adopt' to import existing configs.",
        name, searchedPaths)
```

**Deprecación de module_paths:**

En `loadConfig()` (cli/root.go), después de cargar init.lua:
```go
if initCfg != nil && len(initCfg.ModulePaths) > 0 {
    fmt.Fprintf(os.Stderr,
        "[WARN] module_paths is deprecated in favor of require(%q/).\n"+
            "  → Replace 'module_paths = ...' with:\n"+
            "       local mods = require(%q/)\n"+
            "       return dots.merge({ mods })\n"+
            "  → module_paths will be removed in v0.20.0\n",
        initCfg.ModulePaths[0], initCfg.ModulePaths[0])
}
```

**Deprecación de plugin fallback:**

En `loader.go`, cuando se resuelve `require("x")` via `dots/x.lua`:
```go
fmt.Fprintf(os.Stderr,
    "[WARN] require(%q) resolved as plugin in dots/%s.lua.\n"+
        "  → Use require(\"dots.%s\") for plugins.\n"+
        "  → This fallback will be removed in v0.20.0.\n",
    name, name, name)
```

**Comando `dots migrate`:**

```
dots migrate — Migrate dotfiles config to current format

Detecta:
  - module_paths → sugiere require("dir/")
  - plugins sin dots. prefix → actualiza
  - YAML (path.yaml) → migra a dots.lua (ya existe)
  - init.lua sin files/deps → agrega estructura base
```

#### Tests
- init.lua con loop infinito → timeout después de 5s
- require() con archivo inexistente → error informativo
- module_paths en init.lua → WARN en stderr

#### Criterio de éxito
- `dots doctor` con init.lua legacy muestra WARNs accionables
- Error messages son útiles (no solo "not found")
- `dots migrate` migra configs legacy sin intervención manual

---

## Timeline vs Versiones

| Fase | Descripción | Release |
|------|-------------|---------|
| **1** | RootConfig acepta Files/Deps | **v0.16.0** |
| **2** | require() para user modules | **v0.16.0** |
| **3** | require("dir/") + dots.merge() | **v0.16.0** |
| **4** | Resolver procesa RootConfig.Files | **v0.16.0** |
| **5** | dots adopt actualizado | **v0.16.0** |
| **6** | Checker/doctor extendidos | **v0.16.0** |
| **7** | Timeout, error handling, migrate | **v0.16.0** |
| - | Estabilización y bugs | **v0.17.0** |
| - | FindModules() legacy desactivado por defecto | **v0.18.0** |
| - | Última release con soporte legacy | **v0.19.0** |
| - | YAML + module_paths + FindModules legacy eliminados | **v0.20.0** |

---

## Ejemplos de init.lua (post-implementación)

### Caso 1: TODO en init.lua (config simple)

```lua
return {
  name = "user/dotfiles",
  files = {
    file(".zshrc", "~/.zshrc"),
    file(".gitconfig", "~/.gitconfig"),
    dir("scripts"):into("~/.local/bin"),
  },
  dependencies = {
    pkg "ripgrep",
    pkg "starship",
  },
}
```

### Caso 2: init.lua + require() a archivos sueltos

```lua
-- init.lua
local zsh = require("zsh")
local nvim = require("nvim")

return dots.merge({
  zsh,
  nvim,
  {
    name = "user/dotfiles",
    files = {
      file(".gitconfig", "~/.gitconfig"),
    },
  },
})
```

```lua
-- zsh.lua (en la raíz del repo)
return {
  files = { file(".zshrc", "~/.zshrc") },
  dependencies = { pkg "zsh" },
}
```

### Caso 3: init.lua + require("dir/") con auto-discovery

```lua
-- init.lua
local apps = require("packages/")
-- apps = { Zsh = { files = {...} }, Nvim = { files = {...} } }

return dots.merge({
  apps,
  {
    name = "user/dotfiles",
    files = {
      file(".gitconfig", "~/.gitconfig"),
    },
  },
})
```

```
dotfiles/
├── init.lua              ← entry point
├── .gitconfig            ← file directo
├── packages/             ← auto-descubierto via require("packages/")
│   ├── Zsh/
│   │   ├── dots.lua
│   │   └── .zshrc
│   └── Nvim/
│       ├── dots.lua
│       └── init.lua
└── dots/
    └── helpers.lua       ← plugin custom
```

---

## Prueba en usos reales

Cada fase debe ser probada con un repositorio real de dotfiles antes de pasar
a la siguiente. El canal `dev` permitirá que usuarios interesados prueben las
fases incompletas.

**Script de prueba sugerido por fase:**

```bash
# Fase 1: init.lua con files directos
mkdir -p /tmp/dotfiles-test
cd /tmp/dotfiles-test
echo 'return { name = "test", files = { file("x", "~/.x") } }' > init.lua
touch x
dots --path /tmp/dotfiles-test status
# → Debe mostrar "test (1 pending)"

# Fase 2: require() simple
echo 'return { files = { file("y", "~/.y") } }' > zsh.lua
# init.lua: `local z = require("zsh"); return z`
dots --path /tmp/dotfiles-test status
# → Debe mostrar "zsh (1 pending)"

# Fase 3: require("dir/")
mkdir -p packages/Alacritty
echo 'return { files = { file("a.yml", "~/.config/alacritty.yml") } }' > packages/Alacritty/dots.lua
touch packages/Alacritty/a.yml
# init.lua: `local p = require("packages/"); return dots.merge({ p })`
dots --path /tmp/dotfiles-test status
# → Debe mostrar "Alacritty (1 pending)"
```

---

## Nota: Fases 1 y 2 son paralelizables

La Fase 1 (RootConfig acepta files) y Fase 2 (require() extendido) NO tienen
dependencias entre sí:
- Fase 1: types.go + config.go + root.go (Go-side, no toca require)
- Fase 2: loader.go (Lua require, no toca RootConfig)

Pueden implementarse en paralelo o en cualquier orden.

---

## Notas de Implementación

1. **gopher-lua y require()**: gopher-lua tiene soporte para `package.loaders`.
   El loader se registra modificando la tabla `package.loaders` (existe en
   gopher-lua). Ver `loader.go` para la implementación actual.

2. **FindModules() y dependencia cíclica**: FindModules() se ejecuta dentro
   del require() de Lua, que se ejecuta dentro de LoadRootConfig(). Si
   algún dots.lua dentro de FindModules() intenta un require() que vuelva
   al mismo directorio, hay dependencia cíclica. gopher-lua no protege contra
   esto — se resolverá en una fase posterior con detección de ciclos.

3. **El orden de la tabla `package.loaders`**: El loader de dots debe ir
   PRIMERO en la tabla, para que tenga prioridad sobre el loader por defecto
   de Lua (que busca en paths estándar). Ya se usa `table.insert(..., 1, fn)`.

4. **Formatos de retorno**: Todos los user modules deben devolver una tabla
   con la misma estructura que dots.lua: `{ type?, files?, dependencies? }`.
   No hay un formato diferente para init.lua vs zsh.lua vs dots.lua — todos
   son iguales.

# 📋 Plan de Implementación: Migración de `dots` de Python a Go

_Versión: 1 — Generado: 2026-05-21_
_Estado: PENDIENTE DE APROBACIÓN_

---

## 1. Resumen Ejecutivo

Migrar `dots` — un gestor de dotfiles CLI actualmente escrito en Python (3,900 LOC fuente + 2,200 LOC tests, 26 archivos) — a Go. El objetivo principal es eliminar la latencia de startup (~600ms por comando) que experimentas a diario. Go producirá un **binario estático único** con startup ~2ms, instalación vía `curl + chmod +x`, y un ecosistema TUI moderno (Charm/Bubbletea). La arquitectura se mantiene esencialmente igual, traduciendo cada módulo Python a su equivalente Go. **Tiempo estimado total: 2-3 sprints intensivos (~2-3 semanas).**

---

## 2. Contexto y Estado Actual

### Situación actual
- `dots` es un CLI maduro escrito en Python 3.10+
- Stack: `typer` + `rich` + `pyyaml` + `InquirerPy` + `requests`
- ~6,150 líneas de Python totales (fuente + tests)
- 9 comandos principales: `init`, `link`, `unlink`, `status`, `list`, `adopt`, `install`, `edit`, `backup` + `migrate`
- Instalación: `pipx install` (requiere Python 3.10+, pipx, venv)

### Problema
- **Startup overhead de ~600ms** incluso para comandos triviales (`dots --help`, `dots list`)
- Instalación compleja: el `install.sh` tiene que bootstrappear pipx
- El usuario ejecuta estos comandos a diario — la latencia se acumula

### Suposiciones
- Se mantiene la interfaz CLI exacta (flags, subcomandos, comportamiento)
- Los archivos `path.yaml` y la estructura del repositorio de dotfiles NO cambian
- `dots.toml` como marker legacy se mantiene soportado
- Compatibilidad con Linux, macOS (y Windows si aplica)

### Dependencias externas
- Go 1.22+ como herramienta de build
- Charm Libraries (Lipgloss, Bubbletea) para output TUI
- `gopkg.in/yaml.v3` para YAML
- `github.com/spf13/cobra` para CLI framework
- `github.com/go-git/go-git/v5` para operaciones git (alternativa a subprocess)
- `github.com/hashicorp/go-version` para comparación de versiones

---

## 3. Arquitectura / Diseño de la Solución

### Mapeo Python → Go

```
Python (src/dots/)              →  Go (internal/)
├── __main__.py                  →  cmd/dots/main.go
├── commands/                    →  internal/cli/
│   ├── link.py                  →  link.go
│   ├── status.py                →  status.go
│   ├── list.py                  →  list_module.go
│   ├── init.py                  →  init.go
│   ├── adopt.py                 →  adopt.go
│   ├── install.py               →  install.go
│   ├── unlink.py                →  unlink.go
│   ├── backup.py                →  backup.go
│   ├── migrate.py               →  migrate.go
│   └── edit.py                  →  edit.go
├── core/                        →  internal/
│   ├── config.py                →  config/config.go
│   ├── resolver.py              →  resolver/resolver.go
│   ├── services.py              →  resolver/service.go
│   ├── yaml_parser.py           →  yaml/parser.go
│   ├── schema.py                →  yaml/schema.go
│   ├── transaction.py           →  transaction/transaction.go
│   ├── system.py                →  system/system.go
│   ├── template.py              →  template/template.go
│   ├── module_writer.py         →  writer/module_writer.go
│   └── updates.py               →  update/update.go
├── ui/                          →  internal/ui/
│   ├── output.py                →  output.go  (Lipgloss)
│   ├── selector.py              →  selector.go (Bubbletea)
│   └── theme.py                 →  theme.go
└── plugins/                     →  internal/manager/
    └── managers.py              →  manager.go
```

### Decisiones de Arquitectura Clave

```
DECISIÓN: CLI Framework
OPCIONES: Cobra / standard library `flag`
ELECCIÓN: Cobra
RAZÓN: `dots` tiene 9+ subcomandos con flags complejos (--module repeatable, --type, --format enum, subcommittees en backup). Cobra es el estándar de facto y maneja esto elegantemente.
TRADE-OFFS: Dependencia externa, pero es la más usada y mantenida en el ecosistema Go.

DECISIÓN: Operaciones Git
OPCIONES: `go-git/go-git` / `os/exec` llamando a git CLI
ELECCIÓN: `os/exec` llamando a git CLI
RAZÓN: El código actual llama a `subprocess.run(["git", ...])` y asume git instalado. `go-git` tiene edge cases con auth, worktrees, y submodules. Para backup/push/pull es más robusto delegar al git del sistema.
TRADE-OFFS: Dependencia de tener git instalado (lo mismo que hoy).

DECISIÓN: Terminal Output
OPCIONES: Charm Lipgloss / fatih/color / manual ANSI
ELECCIÓN: Charm Lipgloss + Bubbletea
RAZÓN: Es el equivalente moderno de rich. El selector interactivo actual (InquirerPy) se mapea naturalmente a Bubbletea. Lipgloss da estilos ricos, tablas, colores 256.
TRADE-OFFS: Dependencias Charm (pero son libraries Go, no runtimes).

DECISIÓN: Interactive Selection (InquirerPy → Bubbletea)
OPCIONES: Bubbletea TUI / survey library
ELECCIÓN: Bubbletea para selectores simples, survey para confirmaciones
RAZÓN: Bubbletea ofrece más control visual. survey es más simple para confirmaciones sí/no.

DECISIÓN: Package Manager Detection
OPCIONES: Buscar binarios en PATH / /etc/os-release
ELECCIÓN: `os/exec` + `exec.LookPath` (igual que hoy)
RAZÓN: Funciona cross-platform y es lo que ya hace el código Python.
```

### Modelo de Datos (Go structs)

```go
// DotsConfig — inmutable, runtime config
type DotsConfig struct {
    RepoRoot  string // ~/dotfiles
    CurrentOS string // "linux" | "mac" | "windows"
    HomeDir   string
    CLIDir    string
}

// DotFileMapping — un source → destination
type DotFileMapping struct {
    Source      string
    Destination string
}

// LinkStatus — estado de un mapping
type LinkStatus struct {
    Source      string
    Destination string
    State       string // "linked" | "conflict" | "pending" | "missing" | "unsafe"
    Detail      string
    BackupPath  string
}

// Dependency — una dependencia a instalar
type Dependency struct {
    Name       string
    Type       string // "package" | "git" | "binary"
    URL        string
    Dest       string
    Version    string
    Ref        string
    Arch       map[string]string
    Managers   map[string]string
    Extract    string
    Fallback   *Dependency
    PostInstall string
    Bin        string
}

// TransactionLog — operaciones con rollback
type TransactionLog struct {
    actions   []LinkAction
    committed bool
}
```

---

## 4. Fases e Hitos

### Fase 0: Setup del Proyecto Go + Core Types
**DURACIÓN ESTIMADA**: 1 día
**OBJETIVO**: Proyecto Go compilable, paquete core types, YAML parser funcionando.
**ENTREGABLE**: `go build ./cmd/dots` produce binario, `dots --help` funciona, YAML parser pasa tests.

#### Tareas
- [ ] Inicializar módulo Go: `go mod init github.com/cantoarch/dots`
- [ ] Crear estructura de directorios: `cmd/dots/`, `internal/`
- [ ] Implementar `internal/system/system.go`: `DetectOS()`, `IsSafePath()`, `ExpandPath()`
- [ ] Implementar `internal/config/config.go`: `DotsConfig`, marker detection (`.dots/config.yaml`, `dots.toml`)
- [ ] Implementar `internal/yaml/parser.go`: `ParsePathYAML()`, `ParseDependencies()`, `DetectVariants()`, `FilterByVariant()`
- [ ] Implementar `internal/yaml/schema.go`: validación (v2 → v3, campos requeridos)
- [ ] Implementar `internal/template/template.go`: URL template rendering
- [ ] Implementar `cmd/dots/main.go` + `internal/cli/root.go` con Cobra
- [ ] Añadir comandos vacíos para los 9 subcomandos (esqueletos)
- [ ] Tests: `TestDetectOS`, `TestIsSafePath`, `TestParsePathYAML`, `TestDetectVariants`

#### Criterio de Aceptación
- [ ] `go build ./cmd/dots` produce binario
- [ ] `./dots --help` se imprime instantáneamente
- [ ] YAML parser lee path.yaml correctamente
- [ ] `go test ./internal/...` pasa

---

### Fase 1: Core Engine — resolver + transaction + service
**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: Resolver de symlinks, transaction log, y service layer.
**ENTREGABLE**: `dots status` funcional con output básico.

#### Tareas
- [ ] Implementar `internal/resolver/resolver.go`:
  - `ResolveModules()` — escanea módulos, resuelve estado de symlinks
  - `GetActiveVariant()` — detecta variante activa
  - `ExpandPath()` — expande ~ y rutas
- [ ] Implementar `internal/resolver/service.go`: `DotsService` con `RefreshModules()`, `RefreshBackups()`
- [ ] Implementar `internal/transaction/transaction.go`: `TransactionLog` con `Symlink()`, `Backup()`, `Mkdir()`, `Unlink()`, `Commit()`, `Rollback()`
- [ ] Implementar `internal/ui/output.go` con Lipgloss: `PrintHeader()`, `PrintSuccess()`, `PrintError()`, `PrintWarning()`, `PrintInfo()`
- [ ] Implementar comando `status` completo (default, table, JSON output)
- [ ] Tests unitarios para resolver (mock filesystem)
- [ ] Tests unitarios para transaction log

#### Criterio de Aceptación
- [ ] `dots status` muestra árbol de módulos con colores
- [ ] `dots status --format json` output válido
- [ ] `dots status --state linked` filtra correctamente
- [ ] Transaction log hace rollback en error

---

### Fase 2: Comandos link + unlink + list
**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: Operaciones principales de symlinks.
**ENTREGABLE**: `dots link`, `dots unlink`, `dots list` funcionales.

#### Tareas
- [ ] Implementar comando `link`:
  - `--module`, `--type`, `--dry-run`, `--force`, `--interactive`, `--variant`
  - Trees con estado por módulo
  - Manejo de conflictos y `.orig` backups
  - Variant auto-swap
  - Transaction rollback en error
- [ ] Implementar `internal/ui/selector.go` con Bubbletea: selección interactiva de módulos
- [ ] Implementar comando `unlink`: simétrico a link
- [ ] Implementar comando `list`: `--linked`, `--unlinked`, `--broken`, `--variant`, `--backups`
- [ ] Tests: integración mock para link/unlink

#### Criterio de Aceptación
- [ ] `dots link -m Zsh -m Nvim` enlaza módulos específicos
- [ ] `dots link -i` abre selector interactivo
- [ ] `dots link --dry-run` no hace cambios
- [ ] `dots link --force` sobrescribe conflictos
- [ ] `dots unlink -m Zsh` remueve symlinks
- [ ] `dots list --linked` lista módulos enlazados

---

### Fase 3: Comandos init + adopt + edit
**DURACIÓN ESTIMADA**: 1 día
**OBJETIVO**: Gestión del repositorio de dotfiles.
**ENTREGABLE**: `dots init`, `dots adopt`, `dots edit` funcionales.

#### Tareas
- [ ] Implementar `internal/writer/module_writer.go`: `LoadModuleData()`, `IsDestinationDeclared()`, `AppendFileEntry()`
- [ ] Implementar comando `init`:
  - Detección de shell (Zsh, Bash, Fish)
  - Creación de `.dots/config.yaml`
  - Migración desde legacy `dots.toml`
  - Prompt para añadir `DOTS_REPO` al shell config
- [ ] Implementar comando `adopt`:
  - Mueve archivo al repo y registra en path.yaml
  - Soporte para variants
  - `--dry-run`, `--name`
- [ ] Implementar comando `edit`: abre módulo en `$EDITOR`
- [ ] Implementar `internal/ui/confirm.go` con survey (yes/no prompts)

#### Criterio de Aceptación
- [ ] `dots init` crea `.dots/config.yaml`
- [ ] `dots adopt ~/.zshrc` mueve archivo y registra
- [ ] `dots adopt ~/.zshrc` con variant existente ofrece crear variante
- [ ] `dots edit Zsh` abre el directorio del módulo

---

### Fase 4: Comando install + package managers
**DURACIÓN ESTIMADA**: 1.5 días
**OBJETIVO**: Instalación de dependencias.
**ENTREGABLE**: `dots install` funcional con package managers, git, y binarios.

#### Tareas
- [ ] Implementar `internal/manager/manager.go`:
  - `GetPackageManager()` detecta pacman/apt/brew/pkg
  - `PackageManager` struct con `InstallCommand()`, `NeedsSudo`
- [ ] Implementar instalación de paquetes del sistema (`type: package`)
- [ ] Implementar instalación de bins remotos (`type: binary`):
  - Descarga HTTP con `net/http`
  - Extracción tar.gz/zip
  - Template rendering de URL
- [ ] Implementar clonado git (`type: git`):
  - `os/exec` con `git clone`
  - Checkout de ref
- [ ] Implementar post-install hooks
- [ ] Implementar fallback mechanism (package → binary)
- [ ] Implementar comando `install` con `--module`, `--type`, `--dry-run`
- [ ] Tests: mock HTTP server, mock package manager

#### Criterio de Aceptación
- [ ] `dots install` instala dependencias de todos los módulos
- [ ] `dots install -m Zsh` solo de Zsh
- [ ] `dots install --dry-run` muestra comandos sin ejecutar
- [ ] Binarios se descargan y extraen correctamente
- [ ] Fallback package → binary funciona

---

### Fase 5: Comando backup + version updates
**DURACIÓN ESTIMADA**: 1 día
**OBJETIVO**: Git backup, sync con remote, y notificación de actualizaciones.
**ENTREGABLE**: `dots backup run`, `backup list`, `backup diff`, y notificación de versión.

#### Tareas
- [ ] Implementar operaciones git con `os/exec`:
  - `git add .`, `git commit`, `git push`
  - `git fetch`, `git pull --autostash --rebase`
  - Detección de upstream, conflictos
- [ ] Implementar resolución interactiva de conflictos en rebase
- [ ] Implementar comando `backup run` con `--push/--no-push`, `--no-sync`, `--message`
- [ ] Implementar comando `backup list` (`-n` limit)
- [ ] Implementar comando `backup diff` (ref argument)
- [ ] Implementar `internal/update/update.go`:
  - HTTP request a GitHub API (tags endpoint)
  - Caché en `~/.cache/dots/update.json`
  - Check asíncrono en goroutine
  - `NotifyIfNeeded()` en exit
  - Versión en `--version` flag
- [ ] Tests: mock git commands, mock HTTP

#### Criterio de Aceptación
- [ ] `dots backup run` hace commit + push
- [ ] `dots backup run --no-push` solo commit
- [ ] `dots backup list` muestra historial
- [ ] `dots backup diff HEAD~3` muestra diff
- [ ] Notificación de versión funciona

---

### Fase 6: Comando migrate (v2 → v3) + docs
**DURACIÓN ESTIMADA**: 0.5 días
**OBJETIVO**: Migración de schema path.yaml y documentación.
**ENTREGABLE**: `dots migrate`, README, y todo funcional.

#### Tareas
- [ ] Implementar comando `migrate`:
  - Escanea path.yaml files
  - Migra field names v2 → v3
  - Migra `destination-linux/mac` → `per-os`
  - `--dry-run`, `--yes`
- [ ] Integración final: todos los comandos conectados
- [ ] Actualizar README con nueva instalación
- [ ] Crear `install.sh` simplificado (curl + chmod)
- [ ] CI/CD: GitHub Actions para Go build + test

#### Criterio de Aceptación
- [ ] `dots migrate --dry-run` preview cambios
- [ ] `dots migrate -y` aplica migración
- [ ] `go test ./...` pasa al 100%

---

### Fase 7: Testing + Polishing
**DURACIÓN ESTIMADA**: 1 día
**OBJETIVO**: Cobertura de tests, edge cases, rendimiento.
**ENTREGABLE**: Suite de tests completa, benchmark comparativo.

#### Tareas
- [ ] Tests unitarios para todos los paquetes internos
- [ ] Tests de integración (mock filesystem + git)
- [ ] Benchmarks comparativos Python vs Go
- [ ] Edge cases: TOCTOU races, permisos, symlinks rotos
- [ ] Cross-compile test: `GOOS=linux`, `GOOS=darwin`, `GOOS=windows`
- [ ] Verificar `dots --help` instantáneo

#### Criterio de Aceptación
- [ ] Cobertura >80% en paquetes core
- [ ] Benchmarks muestran <10ms startup
- [ ] Cross-compile exitoso para 3 plataformas

---

## 5. Especificaciones Técnicas Detalladas

### 5.1 Instalación / Configuración de entorno

```bash
# Inicializar proyecto Go
mkdir -p cmd/dots internal/{cli,config,resolver,yaml,transaction,system,template,writer,update,ui,manager}
cd dots
go mod init github.com/cantoarch/dots

# Dependencias principales
go get github.com/spf13/cobra
go get gopkg.in/yaml.v3
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
go get github.com/AlecAivazis/survey/v2

# Dependencias de test
go get github.com/stretchr/testify
```

### 5.2 Estructura final de archivos

```
cmd/
└── dots/
    └── main.go
internal/
├── cli/
│   ├── root.go            # Comando raíz + flags globales
│   ├── link.go            # dots link
│   ├── status.go          # dots status
│   ├── list_module.go     # dots list
│   ├── init.go            # dots init
│   ├── adopt.go           # dots adopt
│   ├── install.go         # dots install
│   ├── unlink.go          # dots unlink
│   ├── edit.go            # dots edit
│   ├── backup.go          # dots backup {run,list,diff}
│   └── migrate.go         # dots migrate
├── config/
│   └── config.go          # DotsConfig, marker detection
├── resolver/
│   ├── resolver.go        # ResolveModules, LinkStatus, variants
│   └── service.go         # DotsService
├── yaml/
│   ├── parser.go          # ParsePathYAML, ParseDependencies, VariantInfo
│   └── schema.go          # Validación v3
├── transaction/
│   └── transaction.go     # TransactionLog, LinkAction
├── system/
│   └── system.go          # DetectOS, IsSafePath, ExpandPath
├── template/
│   └── template.go        # URL template rendering
├── writer/
│   └── module_writer.go   # AppendFileEntry, LoadModuleData
├── update/
│   └── update.go          # Version check, cache, notify
├── ui/
│   ├── output.go          # Lipgloss styles, Print helpers
│   ├── theme.go           # Colors, theme constants
│   └── selector.go        # Bubbletea TUI selector
└── manager/
    └── manager.go         # PackageManager detection
```

### 5.3 Snippets de Código Críticos

#### Entry point (cmd/dots/main.go)

```go
package main

import "github.com/cantoarch/dots/internal/cli"

func main() {
    cli.Execute()
}
```

#### Root Cobra command (internal/cli/root.go)

```go
package cli

import (
    "fmt"
    "os"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "dots",
    Short: "dots — dotfile manager",
    Long:  "Declarative, symlink-based dotfile manager for Linux, macOS, and Windows.",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        // Check for updates in background goroutine
        go checkForUpdates()
        return nil
    },
    RunE: func(cmd *cobra.Command, args []string) error {
        return cmd.Help()
    },
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    notifyIfNeeded()
}
```

#### YAML Parser (internal/yaml/parser.go)

```go
package yaml

import (
    "os"
    "gopkg.in/yaml.v3"
)

type DotFileMapping struct {
    Source      string
    Destination string
}

func ParsePathYAML(path string, currentOS string) ([]DotFileMapping, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var raw map[string]any
    if err := yaml.Unmarshal(data, &raw); err != nil {
        return nil, err
    }

    filesRaw, ok := raw["files"].([]any)
    if !ok {
        return nil, nil
    }

    var mappings []DotFileMapping
    for _, item := range filesRaw {
        f, ok := item.(map[string]any)
        if !ok {
            continue
        }

        source, _ := f["source"].(string)
        if source == "" {
            continue
        }

        // OS filtering
        if osList, ok := f["os"].([]any); ok {
            found := false
            for _, o := range osList {
                if o.(string) == currentOS {
                    found = true
                    break
                }
            }
            if !found {
                continue
            }
        }

        // per-os resolution
        var dest string
        if perOS, ok := f["per-os"].(map[string]any); ok {
            if d, ok := perOS[currentOS].(string); ok {
                dest = d
            }
        }
        if dest == "" {
            dest, _ = f["destination"].(string)
        }
        if dest == "" {
            continue
        }

        mappings = append(mappings, DotFileMapping{
            Source:      source,
            Destination: dest,
        })
    }

    return mappings, nil
}
```

#### Transaction log (internal/transaction/transaction.go)

```go
package transaction

import (
    "os"
    "path/filepath"
)

type ActionType int

const (
    ActionSymlink ActionType = iota
    ActionBackup
    ActionMkdir
    ActionUnlink
)

type LinkAction struct {
    Type       ActionType
    Path       string
    Target     string
    BackupPath string
}

type TransactionLog struct {
    actions   []LinkAction
    committed bool
}

func (t *TransactionLog) Symlink(path, target string) {
    os.Symlink(target, path)
    t.actions = append(t.actions, LinkAction{
        Type:   ActionSymlink,
        Path:   path,
        Target: target,
    })
}

func (t *TransactionLog) Backup(path, backupPath string) {
    if _, err := os.Lstat(path); err != nil {
        return // Already gone, TOCTOU safe
    }
    os.Rename(path, backupPath)
    t.actions = append(t.actions, LinkAction{
        Type:       ActionBackup,
        Path:       path,
        BackupPath: backupPath,
    })
}

func (t *TransactionLog) Commit() {
    t.committed = true
}

func (t *TransactionLog) Rollback() {
    if t.committed {
        return
    }
    for i := len(t.actions) - 1; i >= 0; i-- {
        act := t.actions[i]
        switch act.Type {
        case ActionSymlink:
            os.Remove(act.Path)
        case ActionBackup:
            os.Rename(act.BackupPath, act.Path)
        case ActionUnlink:
            os.Symlink(act.Target, act.Path)
        }
    }
}
```

#### Lipgloss output helper (internal/ui/output.go)

```go
package ui

import (
    "fmt"
    "github.com/charmbracelet/lipgloss"
)

var (
    HeaderStyle = lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("39")) // Cyan

    SuccessStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("76")) // Green

    ErrorStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")) // Red

    WarningStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("214")) // Yellow

    DimStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("243")) // Grey

    DividerStyle = lipgloss.NewStyle().
            Foreground(lipgloss.Color("236")) // Dark grey
)

func PrintHeader(msg string) {
    fmt.Println(HeaderStyle.Render("─── " + msg + " ───"))
}

func PrintSuccess(msg string) {
    fmt.Println(SuccessStyle.Render("✔ " + msg))
}

func PrintError(msg string) {
    fmt.Println(ErrorStyle.Render("✘ " + msg))
}

func PrintWarning(msg string) {
    fmt.Println(WarningStyle.Render("⚠ " + msg))
}

func PrintInfo(msg string) {
    fmt.Println("ℹ " + msg)
}
```

---

## 6. Testing y Validación

| Tipo de test | Cobertura | Herramienta | Criterio |
|---|---|---|---|
| **Unit tests** | Core: config, yaml, transaction, resolver, template, system | `go test` + testify | >80% cobertura en paquetes core |
| **Integration tests** | Link/unlink cycle con mock filesystem | `go test` + temp dirs | Happy path + edge cases |
| **Smoke tests** | Todos los comandos | Script shell | Exit code 0, output esperado |
| **Benchmark** | Startup time vs Python | `hyperfine` (externo) | <10ms vs ~600ms |

---

## 7. Plan de Despliegue

```
PRE-DEPLOY:
- [ ] Último commit en la versión Python (`dev` branch)
- [ ] `git tag v0.9.0-python` para referencia histórica
- [ ] Instalar Go 1.22+ en CI

DEPLOY:
- [ ] Go build para linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- [ ] Publicar binarios en GitHub Releases
- [ ] Actualizar install.sh para: `curl -L ... | tar xz && chmod +x dots`
- [ ] Actualizar README

POST-DEPLOY:
- [ ] Smoke test en Linux
- [ ] Smoke test en macOS
- [ ] Verificar dots list < 10ms
```

---

## 8. Plan de Rollback

1. **Detección**: Error en comando crítico (link, backup) que impida gestión de dotfiles
2. **Decisión**: Si falla más de un comando, revertir a versión Python
3. **Pasos**:
   - `pipx install dots==0.9.0-python` (última versión Python)
   - Reportar bug con output del comando Go
4. **Tiempo estimado**: 5 minutos

---

## 9. Riesgos y Mitigaciones

| Riesgo | Probabilidad | Impacto | Mitigación |
|---|---|---|---|
| **Edge cases en symlinks** (TOCTOU, permisos, NTFS junctions) | Media | Alto | Tests exhaustivos + misma lógica que Python |
| **Diferencia en YAML parsing** (orden de keys, comentarios) | Baja | Medio | Tests con path.yaml reales del usuario |
| **Bubbletea selector no idéntico a InquirerPy** | Media | Bajo | Iterar sobre feedback del usuario |
| **go-git vs os/exec edge cases** | Media | Medio | Usar os/exec (delegar a git CLI) |
| **Cross-compile Windows** | Media | Bajo | CI con Windows, o soporte "best-effort" |

---

## 10. Criterios de Éxito Globales

- [ ] `dots --help` se imprime en <10ms (vs ~630ms hoy)
- [ ] `dots list` se ejecuta en <10ms (vs ~570ms hoy)
- [ ] Todos los comandos y flags existentes funcionan idéntico
- [ ] `go test ./...` pasa al 100%
- [ ] Instalación vía un solo comando sin Python
- [ ] Path.yaml files existentes no requieren cambios

---

## 11. Recursos y Referencias

- **Código actual**: `src/` (Python) — toda la lógica de negocio está aquí
- **Tests actuales**: `tests/` — ~2,200 LOC de tests para guiar la migración
- **Path.yaml spec**: `docs/path-yaml-reference.md`
- **Skills relacionadas**: `gentleman-bubbletea` (Bubbletea TUI patterns para Go)
- **Charm Libraries Docs**: https://github.com/charmbracelet
- **Cobra CLI Docs**: https://github.com/spf13/cobra

---

## 12. Primeros pasos al aprobar

1. Ejecutar `go mod init github.com/cantoarch/dots` y crear estructura de directorios
2. Implementar `internal/system/system.go` y `internal/config/config.go` (Fase 0)
3. Implementar `internal/yaml/parser.go` (el parser de path.yaml es la pieza más crítica — todo depende de él)

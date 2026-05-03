# Schema v3 — path.yaml

> **Versión**: 3.0.0
> **Status**: Active
> **Scope**: Estandarización del schema de `dependencies` y `files` en `path.yaml`

## Principio

Cada fase del migration es deployable de forma independiente. Si el proceso se detiene en cualquier fase, `dots` sigue funcionando.

---

## Dependencies

### Tipos soportados

| Type | Descripción | Campos requeridos |
|------|-------------|-------------------|
| `package` | Paquete del sistema (pacman, apt, brew) | `name` |
| `git` | Clonar repositorio git | `url`, `dest` |
| `binary` | Descargar binario/tarball | `url`, `dest` |

### Campos

| Campo v3 | Campo v2 (legacy) | Tipo | Requerido | Descripción |
|----------|-------------------|------|-----------|-------------|
| `name` | `name` | string | siempre | Nombre de la dependencia |
| `type` | `type` | string | no | Default: `package` |
| `bin` | — | string | no | Nombre del ejecutable si difiere de `name` |
| `url` | `source` | string | git, binary | URL del recurso |
| `dest` | `target` | string | git, binary | Destino en el sistema |
| `version` | `version` | string | no | Versión para templating |
| `ref` | `ref` | string | no | Git ref (tag/branch/commit) |
| `arch` | `arch_map` | dict | no | Mapeo de arquitecturas |
| `managers` | `package-managers` | dict | no | Nombre de paquete por PM |
| `extract` | `extract-path` | string | no | Path dentro del tarball |
| `fallback` | `fallback` | dict | no | Dep inline si PM no tiene el paquete |
| `post-install` | `post-install` | string | no | Comando a ejecutar después |

### Ejemplos

```yaml
# type: package — paquete del sistema
- name: ripgrep
  type: package
  managers:
    pacman: ripgrep
    apt: ripgrep
    brew: ripgrep

# type: package con binario de nombre diferente
- name: bat
  type: package
  bin: batcat  # opcional — nombre del ejecutable cuando difiere de name
  managers:
    apt: bat
    brew: bat

# type: package con fallback
- name: fd
  type: package
  managers:
    brew: fd
  fallback:
    type: binary
    url: https://github.com/sharkdp/fd/releases/download/{{version}}/fd-{{version}}-{{arch}}-unknown-linux-gnu.tar.gz
    dest: ~/.local/bin/fd
    version: v8.7.0

# type: git — clonar repositorio
- name: powerlevel10k
  type: git
  url: https://github.com/romkatv/powerlevel10k.git
  dest: ~/.local/share/zsh/plugins/powerlevel10k
  ref: v1.19.0

# type: binary — descargar y extraer de tarball
- name: gh
  type: binary
  url: https://github.com/cli/cli/releases/download/{{version}}/gh_{{version}}_linux_{{arch}}.tar.gz
  dest: ~/.local/bin/gh
  version: v2.50.0
  extract: gh_{{version}}_linux_{{arch}}/bin/gh
  arch:
    x86_64: amd64
    aarch64: arm64
```

---

## Files

### Campos

| Campo v3 | Campo v2 (legacy) | Tipo | Descripción |
|----------|-------------------|------|-------------|
| `source` | `source` | string | **Requerido** — Path relativo dentro del módulo |
| `destination` | `destination` | string | Destino fallback (si no hay per-os) |
| `per-os` | `destination-override` | dict | Destinos por sistema operativo |
| `os` | `os` | list | Filtrar por SO |

### Resolución de destino (orden)

1. `per-os[current_os]` — si existe
2. `destination` — fallback genérico
3. Skip — si ninguno aplica

### Ejemplos

```yaml
files:
  # Caso básico
  - source: nvim
    destination: ~/.config/nvim

  # Filtrar por OS
  - source: .zshrc
    destination: ~/.zshrc
    os: [linux]

  # Destination por OS (v3 — único mecanismo)
  - source: config.toml
    destination: ~/.config/tool/config.toml   # fallback
    per-os:
      mac: ~/Library/Preferences/tool/config.toml
      linux: ~/.config/tool/config.toml

  # Variants (sin cambios)
  - source: nvim-stdlib
    destination: ~/.config/nvim
  - source: nvim-lazy
    destination: ~/.config/nvim
```

---

## Templating

### Variables disponibles

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `{{arch}}` | Arquitectura del sistema | `x86_64`, `aarch64` |
| `{{version}}` | Versión de la dependencia | `v2.50.0` |

### Resolución de arquitectura

1. Si existe `arch`, usar el mapeo
2. Si no, usar el valor bruto de `platform.machine()`

```yaml
arch:
  x86_64: amd64   # transforma x86_64 -> amd64 en la URL
  aarch64: arm64  # transforma aarch64 -> arm64
```

---

## v2 Detection

Si se detecta uso de schema v2, dots falla rápido con mensaje claro:

```
[path.yaml] Schema v2 detected in dependencies[0]. Run 'dots migrate' to upgrade to v3 automatically.
```

Los campos detectados:

- Dependencies: `source`, `target`, `extract-path`, `arch_map`, `package-managers`
- Files: `destination-override`, `destination-linux`, `destination-mac`

---

## Backward Compatibility

**No se mantiene backward compatibility con schema v2.**

Los campos v2 (`source`, `target`, `extract-path`, `arch_map`, `package-managers`, `destination-override`, `destination-linux`, `destination-mac`) ya no son reconocidos.

Para migrar existente -> usar comando `dots migrate`.

---

## Phases

| Phase | Descripción | Estado |
|-------|-------------|--------|
| 0 | Spec final (este documento) | ✅ Done |
| 1 | Validación de schema al parsear | ✅ Done |
| 2 | Template engine como módulo propio | ✅ Done |
| 3 | Renombrar campos (parser + dataclass) | ✅ Done |
| 4 | Estandarizar sección files | ✅ Done |
| 5 | Comando `dots migrate` | ✅ Done |
| 6 | Cleanup y bump de versión | ✅ Done |

---

## Fuera de scope (por ahora)

- Shorthand syntax en files (`- nvim → ~/.config/nvim`)
- Glob patterns en source (`configs/*`)
- Templating `{{variant}}` en destination
- Variables `$HOME` en destinations (ya funciona, solo falta documentar)
- `type: script` y `type: curl` (van al roadmap como planned)
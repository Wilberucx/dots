# Sintaxis Lua de `dots`

> **Versión**: 1.0 — Junio 2026
>
> Guía completa de la sintaxis Lua para configurar módulos en `dots`,
> incluyendo el sistema de plugins y cómo extender sus capacidades.
>
> _English version: [docs/lua-syntax.md](lua-syntax.md)_

---

## Índice

1. [Arquitectura general](#1-arquitectura-general)
2. [`init.lua` — Configuración raíz](#2-initlua--configuración-raíz)
3. [`dots.lua` — Configuración de módulo](#3-dotslua--configuración-de-módulo)
4. [API de operaciones de archivos](#4-api-de-operaciones-de-archivos)
5. [API de dependencias](#5-api-de-dependencias)
6. [Descubrimiento de módulos](#6-descubrimiento-de-módulos)
7. [Sistema de plugins](#7-sistema-de-plugins)
8. [Migración desde YAML](#8-migración-desde-yaml)
9. [Referencia rápida](#9-referencia-rápida)

---

## 1. Arquitectura general

`dots` utiliza **dos tipos de archivos Lua** para la configuración:

| Archivo | Rol | Ubicación |
|---------|-----|-----------|
| `init.lua` | Configuración raíz del repositorio | Raíz del repo (`~/dotfiles/`) |
| `dots.lua` | Configuración de un módulo individual | Dentro de cada módulo (`~/dotfiles/Zsh/dots.lua`) |

Cada archivo Lua **debe retornar una tabla** (`return { ... }`). El motor Lua
embebido (`gopher-lua`) ejecuta el script y extrae la configuración.

```
dotfiles/                     ← raíz del repo
├── init.lua                  ← archivo marker + configuración raíz
├── Zsh/
│   ├── dots.lua              ← configuración del módulo Zsh
│   └── .zshrc                ← archivo fuente
├── Nvim/
│   ├── dots.lua
│   └── init.lua
├── scripts/                   ← plugins compartidos (opcional)
│   ├── helpers.lua
│   └── colors.lua
└── .gitignore
```

> **Nota**: `init.lua` también funciona como **archivo marker**: si existe en un
> directorio, `dots` reconoce ese directorio como un repositorio de dotfiles
> (junto con los formatos legacy `.dots/config.yaml` y `dots.toml`).

---

## 2. `init.lua` — Configuración raíz

### 2.1 Campos disponibles

El archivo `init.lua` se coloca en la raíz del repositorio y define la
configuración global. Todos los campos son **opcionales**.

```lua
return {
  -- Nombre del repositorio (para identificación)
  -- Por defecto: "dotfiles"
  name = "cantoarch/dotfiles",

  -- Rutas donde buscar módulos (opcional)
  -- Por defecto: busca en la raíz del repo
  -- string: busca solo en esa ruta
  -- table:  busca en múltiples rutas
  module_paths = "modules/",
  -- module_paths = { "packages/", "configs/" },

  -- Plugins a cargar (opcional)
  -- Los plugins se cargan vía require() y quedan disponibles como globales
  plugins = { "dots.http", "dots.archive", "dots.git" },
}
```

### 2.2 Campo `name`

Identifica el repositorio. Se usa para mensajes informativos y como nombre por
defecto del repo.

```lua
return { name = "usuario/dotfiles" }
```

Si se omite, el valor por defecto es `"dotfiles"`.

### 2.3 Campo `module_paths`

Controla **dónde** busca `dots` los módulos. Es una redirección — no suma
rutas, sino que **reemplaza** la raíz del repo como lugar de búsqueda.

**Sin `module_paths`** (comportamiento por defecto):

```lua
-- Busca módulos en todos los directorios de la raíz del repo
return { name = "dotfiles" }
-- → Escanea: Zsh/, Nvim/, Kitty/, ...
```

**Con `module_paths` como string**:

```lua
-- Busca módulos SOLO dentro de modules/
return {
  name = "dotfiles",
  module_paths = "modules/",
}
-- → Escanea: modules/Zsh/, modules/Nvim/, ...
```

**Con `module_paths` como tabla**:

```lua
-- Busca módulos SOLO dentro de packages/ y configs/
return {
  name = "dotfiles",
  module_paths = { "packages/", "configs/" },
}
-- → Escanea: packages/Zsh/, configs/Nvim/, ...
```

> **Importante**: Si un directorio no está dentro de las rutas especificadas,
> no se considera un módulo. Esto permite tener directorios auxiliares en el
> repo que no son módulos de dotfiles.

#### Advertencias

- Si `module_paths` apunta a un directorio que no existe, `dots` muestra un
  aviso (`[WARN] module_paths "..." does not exist — skipping`) y continúa
  con las siguientes rutas.
- Si dos módulos tienen el mismo nombre en distintas rutas, el primero
  encontrado (por orden de `module_paths`) gana — el segundo se ignora.

### 2.4 Campo `plugins`

Lista de plugins a cargar en el VM de Lua. Los plugins pueden ser:

- **Plugins integrados** (embebidos en el binario): `dots.http`, `dots.archive`,
  `dots.git`
- **Plugins personalizados**: colocados en `<repo_root>/dots/<nombre>.lua`

Al cargarse, cada plugin se ejecuta y su valor de retorno se asigna como una
variable global con el nombre limpio (sin prefijo `dots.`):

```lua
plugins = { "dots.http", "dots.archive", "dots.git" }
-- → http, archive, git quedan disponibles como globales en Lua
```

Ver [Sistema de plugins](#7-sistema-de-plugins) para más detalle.

---

## 3. `dots.lua` — Configuración de módulo

Cada módulo de dotfiles tiene un archivo `dots.lua` en su directorio raíz.
Este archivo define los archivos a enlazar y las dependencias a instalar.

### 3.1 Estructura básica

```lua
return {
  -- Tipo del módulo (opcional)
  -- Valores comunes: "minimal", "full", "editor", "terminal", etc.
  type = "minimal",

  -- Lista de operaciones de archivos (opcional)
  files = {
    -- ... operaciones file(), dir(), glob()
  },

  -- Lista de dependencias (opcional)
  dependencies = {
    -- ... operaciones pkg(), curl(), git()
  },
}
```

### 3.2 El campo `type`

Es una etiqueta descriptiva que puedes usar para filtrar módulos vía
`dots status --type editor`, `dots link --type terminal`, etc. No tiene
efecto en el comportamiento.

### 3.3 El campo `files`

Array de operaciones que producen symlinks. Cada operación es creada por una
de las funciones `file()`, `dir()`, o `glob()`.

```lua
files = {
  file(".zshrc", "~/.zshrc"),
  dir("config"):to("~/.config/alacritty"),
  glob("*.toml"):into("~/.config/"),
}
```

Cada elemento se procesa en orden y produce uno o más `LinkStatus` (estados
de symlink: `linked`, `conflict`, `pending`, `missing`, `unsafe`).

### 3.4 El campo `dependencies`

Array de dependencias a instalar. Cada dependencia es creada por una de
las funciones `pkg()`, `curl()`, o `git()`.

```lua
dependencies = {
  pkg "ripgrep",
  pkg("starship"):on({ pacman = "starship", brew = "starship" }),
  curl("https://example.com/tool.tar.gz"):extract("tool"):to("~/.local/bin/tool"),
  git("https://github.com/user/repo.git"):to("~/repo"):at("v1.0"),
}
```

Las dependencias se deduplican por nombre: la primera definición gana.

---

## 4. API de operaciones de archivos

### 4.1 `file(source, destination)`

Crea un symlink de un archivo individual.

```lua
file(".zshrc", "~/.zshrc")
```

**Parámetros:**

| Parámetro | Tipo | Obligatorio | Descripción |
|-----------|------|-------------|-------------|
| `source` | string | sí | Ruta relativa al directorio del módulo |
| `destination` | string | sí | Ruta de destino (soporta `~` → `$HOME`) |

#### Métodos encadenables

##### `:when(os)`

Filtra la operación por sistema operativo.

```lua
file(".zshenv", "~/.zshenv"):when("linux")
file(".mac-config", "~/.config/mac"):when("mac")
```

Valores válidos: `"linux"`, `"mac"`, `"windows"`.

Si el OS actual no coincide con el filtro, la operación se omite
silenciosamente.

##### `:variant(name)`

Asigna un nombre explícito de variante a una operación de archivo para
permitir el cambio de configuración basado en variantes.

```lua
file("work/gitconfig", "~/.gitconfig"):variant("work")
file("personal/gitconfig", "~/.gitconfig"):variant("personal")
```

Consulta el [Sistema de Variantes](variants.md) para más detalles sobre
cómo funcionan las variantes.

##### `:per_os({ ... })`

Define destinos diferentes según el sistema operativo.

```lua
file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
  linux = "~/.config/alacritty/linux.toml",
  mac   = "~/Library/Application Support/alacritty/mac.toml",
  windows = "~/AppData/alacritty/win.toml",
})
```

El segundo argumento de `file()` se usa como destino por defecto (fallback)
para OS no listados en la tabla `per_os`.

**Combinación de `:when()` y `:per_os()`:**

```lua
-- :when() filtra todo el file; :per_os() elige destino
file("app.conf", "~/.config/app.conf"):when("linux"):per_os({
  linux = "~/.config/linux.conf",
  mac   = "~/Library/mac.conf",
})
```

---

### 4.2 `dir(source)`

Operaciones con directorios. Requiere uno de los métodos `:to()` o `:into()`
para completarse.

#### `:to(destination)`

Crea un symlink del directorio completo hacia el destino.

```lua
dir("config"):to("~/.config/alacritty")
-- → ~/.config/alacritty → ~/dotfiles/Alacritty/config/ (symlink de directorio)
```

Todo el contenido del directorio fuente aparece en el destino como un solo
symlink. Útil para configuraciones que esperan un directorio completo.

#### `:into(destination)`

Expande el contenido del directorio: cada archivo dentro del directorio fuente
se enlaza individualmente al destino.

```lua
dir("scripts"):into("~/.local/bin")
-- Si scripts/ contiene foo.sh y bar.sh:
-- → ~/.local/bin/foo.sh → ~/dotfiles/Scripts/scripts/foo.sh
-- → ~/.local/bin/bar.sh → ~/dotfiles/Scripts/scripts/bar.sh
```

A diferencia de `:to()`, que crea un único symlink, `:into()` crea N symlinks
(uno por archivo hijo).

> **Nota**: Si el directorio está vacío, no se crea ningún symlink (éxito
> silencioso).

> **⚠️ Error común**: Usar `:into()` cuando quieres `:to()` es el error
> más frecuente con `dir()`.
>
> ```lua
> -- INCORRECTO: Si nvim/ es un directorio y quieres enlazarlo entero:
> dir("nvim"):into("~/.config/nvim")  -- ❌ intenta crear symlinks individuales:
>                                    -- ~/.config/nvim/init.lua en vez de:
>
> -- CORRECTO: Symlink del directorio completo
> dir("nvim"):to("~/.config/nvim")   -- ✅ ~/.config/nvim → dotfiles/Nvim/nvim
> ```
>
> **Regla práctica:**
>
> | Si quieres…                               | Usa…                     |
> |-------------------------------------------|--------------------------|
> | `~/.config/nvim` → `dotfiles/Nvim/nvim/`  | `dir("nvim"):to("~/.config/nvim")` |
> | `~/.config/nvim/init.lua` → `dotfiles/Nvim/nvim/init.lua` | `dir("nvim"):into("~/.config/nvim")` |
> | `~/.config/nvim/config/foo.lua` → `dotfiles/Nvim/config/foo.lua` | `file("config/foo.lua", "~/.config/nvim/config/foo.lua")` |

#### `:when(os)` y `:per_os({ ... })`

También están disponibles para objetos `dir()`:

```lua
dir("scripts"):into("~/.local/bin"):when("linux")
dir("config"):to("~/.config/app"):per_os({
  linux = "~/.config/app",
  mac   = "~/Library/Application Support/app",
})
```

---

### 4.3 `glob(pattern)`

Empareja archivos por patrón glob y los enlaza individualmente al destino.

```lua
glob("*.toml"):into("~/.config/")
-- → ~/.config/alacritty.toml → ~/dotfiles/Configs/alacritty.toml
-- → ~/.config/kitty.toml     → ~/dotfiles/Configs/kitty.toml
-- (readme.txt NO se enlaza porque no matchea *.toml)
```

**Parámetros:**

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `pattern` | string | Patrón glob relativo al directorio del módulo |

El patrón se resuelve con `filepath.Glob()` desde el directorio del módulo.

> **Nota**: Si ningún archivo matchea el patrón, no se crea ningún symlink
> (éxito silencioso).

#### Métodos

- **`:into(destination)`** — Obligatorio. Define el directorio donde se
  colocarán los symlinks (cada archivo matcheado mantiene su nombre base).
- **`:when(os)`** — Opcional. Filtra por OS.
- **`:per_os({ ... })`** — Opcional. Destinos por OS.

---

### 4.4 Resumen de métodos encadenables

| Función | Métodos disponibles |
|---------|-------------------|
| `file()` | `:when(os)`, `:per_os({...})`, `:variant(name)` |
| `dir()` | `:to(dest)`, `:into(dest)`, `:when(os)`, `:per_os({...})`, `:variant(name)` |
| `glob()` | `:into(dest)`, `:when(os)`, `:per_os({...})`, `:variant(name)` |

---

## 5. API de dependencias

### 5.1 `pkg(name)`

Declara una dependencia de paquete del sistema.

```lua
-- Forma simple (solo nombre)
pkg "ripgrep"

-- Forma explícita con opciones
pkg("fd"):on({ pacman = "fd", apt = "fd-find", brew = "fd" })

-- Con fallback a binary
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/release.tar.gz"):extract("starship"))
```

**Parámetros:**

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `name` | string | Nombre del paquete (obligatorio) |

#### Métodos

##### `:on({ managers })`

Define el nombre del paquete por gestor de paquetes. Las claves son los
nombres de los gestores (`pacman`, `apt`, `brew`, etc.) y los valores son
los nombres del paquete en ese gestor.

```lua
pkg("fd"):on({
  pacman = "fd",
  apt    = "fd-find",
  brew   = "fd",
})
```

##### `:bin(name)`

Especifica el nombre del binario resultante (útil cuando el binario se llama
distinto al paquete).

```lua
pkg("nodejs"):bin("node")
```

##### `:post(command)`

Comando a ejecutar después de instalar el paquete.

```lua
pkg("neovim"):post("pip install pynvim")
```

##### `:fallback(dep)`

Define una dependencia de respaldo si el gestor de paquetes no está
disponible. El argumento debe ser otro objeto de dependencia (normalmente
`curl()` para descargar un binario).

```lua
pkg("starship"):on({ pacman = "starship", brew = "starship" })
  :fallback(curl("https://github.com/starship/release.tar.gz"):extract("starship"))
```

---

### 5.2 `curl(url)`

Declara una dependencia binaria descargada vía HTTP.

```lua
curl("https://github.com/eza-community/eza/releases/latest/eza.tar.gz")
  :extract("eza")
  :to("~/.local/bin/eza")
  :version("v0.10.0")
  :arch({ x86_64 = "amd64", aarch64 = "arm64" })
```

**Parámetros:**

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `url` | string | URL del archivo a descargar (obligatorio) |

#### Métodos

##### `:extract(member)`

Extrae un miembro específico del archivo descargado (tar.gz, zip).

```lua
curl("https://example.com/tool.tar.gz"):extract("tool/bin/tool")
```

##### `:to(destination)`

Define la ruta final donde colocar el binario extraído.

```lua
curl("https://example.com/tool.tar.gz"):extract("tool"):to("~/.local/bin/tool")
```

##### `:version(version)`

Especifica la versión para interpolación en la URL (reemplaza `{{version}}`).

```lua
curl("https://example.com/tool-v{{version}}.tar.gz"):version("v1.2.3")
```

##### `:arch({ arch_map })`

Define el mapeo de arquitectura del sistema a nombres en el binario
(reemplaza `{{arch}}`).

```lua
curl("https://example.com/tool-{{arch}}.tar.gz"):arch({
  x86_64  = "amd64",
  aarch64 = "arm64",
})
```

##### `:bin(name)`

Nombre del binario (para identificación).

##### `:post(command)`

Comando a ejecutar post-instalación.

---

### 5.3 `git(url)`

Declara una dependencia de repositorio git.

```lua
git("https://github.com/romkatv/powerlevel10k.git")
  :to("~/.local/share/zsh/plugins/p10k")
  :at("v1.19.0")
  :post("git -C ~/p10k submodule update --init")
```

**Parámetros:**

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| `url` | string | URL del repositorio git (obligatorio) |

#### Métodos

##### `:to(destination)`

Ruta donde clonar el repositorio.

##### `:at(ref)`

Referencia a checkout (tag, branch, commit hash).

```lua
git("https://github.com/user/repo.git"):to("~/repo"):at("v2.0.0")
git("https://github.com/user/repo.git"):to("~/repo"):at("main")
```

##### `:bin(name)`

Nombre del binario (para identificación).

##### `:post(command)`

Comando a ejecutar post-clonación.

---

### 5.4 Tabla resumen de métodos

| Función | Métodos disponibles |
|---------|-------------------|
| `pkg()` | `:on({...})`, `:bin()`, `:post()`, `:fallback()` |
| `curl()` | `:extract()`, `:to()`, `:version()`, `:arch({...})`, `:bin()`, `:post()` |
| `git()` | `:to()`, `:at()`, `:bin()`, `:post()` |

---

## 6. Descubrimiento de módulos

### 6.1 Reglas de descubrimiento

1. Se lee `init.lua` (si existe) para obtener `module_paths`
2. Si hay `module_paths`: se escanean SOLO esas rutas (de forma recursiva)
3. Si NO hay `module_paths`: se escanea la raíz del repo (de forma recursiva)
4. Dentro de las rutas de búsqueda, cualquier directorio que contenga
   `dots.lua` o `path.yaml` se considera un módulo

### 6.2 Directorios excluidos

Los siguientes directorios se excluyen automáticamente:

- `.git/` — repositorio git
- `.dots/` — directorio legacy de dots
- `node_modules/` — dependencias de Node.js
- `cli/` — CLI interno (generado)
- Cualquier directorio que comience con `.` (oculto)

### 6.3 Prioridad Lua vs YAML

Si un directorio contiene tanto `dots.lua` como `path.yaml`:

- **`dots.lua` tiene prioridad** con un aviso:
  `[WARN] Module 'X' has both dots.lua and path.yaml. Using dots.lua.`
- El módulo se trata como tipo Lua (ignorando `path.yaml`)

### 6.4 Módulos sin configuración

Los directorios sin `dots.lua` ni `path.yaml` se ignoran silenciosamente.

### 6.5 Orden de los módulos

Los módulos se devuelven ordenados alfabéticamente por nombre para
consistencia.

---

## 7. Sistema de plugins

### 7.1 Arquitectura

El sistema de plugins permite extender las capacidades de `dots` mediante
scripts Lua que se cargan en el mismo VM donde se ejecutan las
configuraciones.

```
require("dots.http")
     │
     ├── 1. Buscar en plugins integrados (embebidos)
     │   └── internal/lua/plugins/http.lua  ── compilado en el binario
     │
     ├── 2. Buscar en <repo_root>/dots/http.lua
     │
     └── 3. Error: plugin no encontrado
```

El orden de búsqueda es:

1. **Plugins integrados** — embebidos en el binario (`internal/lua/plugins/`)
2. **Plugins personalizados** — en `<repo_root>/dots/<nombre>.lua`

### 7.2 Plugins integrados

`dots` incluye 3 plugins integrados:

#### `dots.http` → global `http`

```lua
-- Cargar en init.lua
plugins = { "dots.http" }

-- Usar en dots.lua
http.download("https://example.com/file.tar.gz", "/tmp/file.tar.gz")
```

**API:**

- `http.download(url, dest)` — Descarga una URL a una ruta local. Usa `curl`
  o `wget` (el que esté disponible).

→ Ver: `internal/lua/plugins/http.lua`

#### `dots.archive` → global `archive`

```lua
-- Cargar en init.lua
plugins = { "dots.archive" }

-- Usar en dots.lua
archive.extract_tar("/tmp/file.tar.gz", "/tmp/extracted", "bin/tool")
```

**API:**

- `archive.extract_tar(archive, dest, member)` — Extrae un miembro específico
  de un tar.gz, o todo si `member` es nil/vacío.
- `archive.extract_zip(archive, dest)` — Extrae un zip completo.

→ Ver: `internal/lua/plugins/archive.lua`

#### `dots.git` → global `git`

```lua
-- Cargar en init.lua
plugins = { "dots.git" }

-- Usar en dots.lua
git.clone("https://github.com/user/repo.git", "~/repo")
git.checkout("v1.0", "~/repo")
```

**API:**

- `git.clone(url, dest)` — Clona un repositorio git.
- `git.checkout(ref, dir)` — Hace checkout de una ref específica.

→ Ver: `internal/lua/plugins/git.lua`

### 7.3 Plugins personalizados

Puedes crear tus propios plugins colocando archivos `.lua` en el directorio
`dots/` de la raíz de tu repositorio de dotfiles.

#### Crear un plugin

```lua
-- ~/dotfiles/dots/helpers.lua
local helpers = {}

function helpers.greet(name)
  print("Hello, " .. name .. "!")
end

function helpers.is_installed(bin)
  local handle = io.popen("command -v " .. bin .. " 2>/dev/null")
  local result = handle:read("*a")
  handle:close()
  return result:match("%S") ~= nil
end

return helpers
```

#### Registrarlo en `init.lua`

```lua
-- ~/dotfiles/init.lua
return {
  name = "usuario/dotfiles",
  plugins = { "dots.http", "helpers" },
}
```

> **Nota**: Los plugins sin prefijo `dots.` se buscan en
> `<repo_root>/dots/<nombre>.lua`. Si no existen, dan error.

#### Usarlo en `dots.lua`

Los plugins quedan registrados como **variables globales** en el mismo VM de
Lua donde se ejecuta `dots.lua`. Esto significa que están accesibles desde
cualquier `dots.lua` sin necesidad de importarlos explícitamente:

```lua
-- ~/dotfiles/Zsh/dots.lua — los plugins están disponibles como globales
if helpers.is_installed("starship") then
  print("starship is ready!")
end
```

> La función `require()` también está disponible como mecanismo alternativo.
> El searcher personalizado registrado en `package.loaders` resuelve plugins
> tanto integrados como personalizados.

### 7.4 Limitaciones del sistema de plugins

- Los plugins se cargan **antes** de ejecutar `dots.lua` de cualquier módulo
- Los plugins tienen acceso a `io`, `os.execute`, y otras APIs estándar de Lua
  (gopher-lua soporta un subconjunto de la biblioteca estándar de Lua 5.4)
- No hay aislamiento entre plugins — pueden modificar globales
- Los plugins no pueden extender la sintaxis de `dots.lua` (no puedes agregar
  nuevas funciones como `file()` o `pkg()`) porque esas están registradas en
  Go antes de ejecutar cualquier script Lua
- Para agregar nuevas funciones a la API de dots.lua, se necesita modificar el
  código Go en `internal/lua/api_files.go` y `internal/lua/api_deps.go`

---

## 8. Migración desde YAML

### 8.1 Migración manual

> **Nota**: El comando `dots migrate` CLI está en desarrollo y no está
> disponible en esta versión. La migración se realiza manualmente usando
> la función `MigrateModule()` desde Go, o editando directamente los archivos.

El migrador convierte módulos existentes de `path.yaml` a `dots.lua`.
Puedes migrar manualmente usando el template como guía.

### 8.2 Mapeo YAML → Lua

| YAML (`path.yaml`) | Lua (`dots.lua`) |
|-------------------|------------------|
| `source: x` + `destination: ~/x` | `file("x", "~/x")` |
| `os: [linux]` | `:when("linux")` |
| `per-os: { linux: ~/x, mac: ~/y }` | `:per_os({ linux = "~/x", mac = "~/y" })` |
| `destination: ~/dir/*` (expansión) | `dir("src"):into("~/dir")` |
| Source sin extensión (directorio) | `dir("src"):to("~/dest")` |
| `dependencies: ["pkg"]` | `pkg "pkg"` |
| `type: binary` + `url:` | `curl(url):extract():to()` |
| `type: git` + `url:` | `git(url):to():at()` |
| `managers: { pacman: x }` | `:on({ pacman = "x" })` |
| `fallback:` | `:fallback(curl(...))` |

### 8.3 Ejemplo de migración

**Antes** (`path.yaml`):

```yaml
type: full
files:
  - source: init.lua
    destination: ~/.config/nvim/init.lua
  - source: alacritty.yml
    per-os:
      linux: ~/.config/alacritty.yml
      mac: ~/Library/alacritty.yml
  - source: scripts
    destination: ~/.local/bin/*
dependencies:
  - neovim
  - name: fd
    type: binary
    url: https://example.com/fd.tar.gz
    dest: ~/.local/bin/fd
    extract: fd/fd
```

**Después** (`dots.lua`):

```lua
-- dots.lua — generated from path.yaml
return {
  type = "full",

  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("alacritty.yml", "~/.config/alacritty.yml"):per_os({
      linux = "~/.config/alacritty.yml",
      mac = "~/Library/alacritty.yml",
    }),
    dir("scripts"):into("~/.local/bin"),
  },

  dependencies = {
    pkg "neovim",
    curl("https://example.com/fd.tar.gz"):bin("fd"):extract("fd/fd"):to("~/.local/bin/fd"),
  },
}
```

### 8.4 Coexistencia

Durante la transición, ambos formatos coexisten:

- Módulos con `dots.lua` usan el nuevo sistema
- Módulos con solo `path.yaml` usan el sistema legacy
- Si ambos existen, `dots.lua` gana (con advertencia)

---

## 9. Referencia rápida

### 9.1 `init.lua` — Campos

```lua
return {
  name         = "string",        -- Nombre del repo (default: "dotfiles")
  module_paths = "path/" | {...}, -- Rutas de búsqueda (default: raíz)
  plugins      = { "dots.http" }, -- Plugins a cargar
}
```

### 9.2 `dots.lua` — Campos

```lua
return {
  type         = "string",        -- Etiqueta del módulo (opcional)
  files        = { ... },         -- Operaciones de archivo (opcional)
  dependencies = { ... },         -- Dependencias (opcional)
}
```

### 9.3 Operaciones de archivo

```lua
-- Archivo individual
file("source", "~/.dest")

-- Con filtro OS
file("source", "~/.dest"):when("linux")

-- Con destino por OS
file("source", "~/.dest"):per_os({ linux = "~/.a", mac = "~/.b" })

-- Con declaración de variante
file("work/config", "~/.config/app"):variant("work")
file("personal/config", "~/.config/app"):variant("personal")

-- Directorio completo (symlink)
dir("carpeta"):to("~/.dest")

-- Expandir contenido del directorio
dir("carpeta"):into("~/.dest")

-- Patrón glob
glob("*.toml"):into("~/.config/")
```

### 9.4 Dependencias

```lua
-- Paquete del sistema (simple)
pkg "ripgrep"

-- Paquete con gestores
pkg("fd"):on({ pacman = "fd", apt = "fd-find" })

-- Paquete con fallback binary
pkg("starship"):on({ pacman = "starship" })
  :fallback(curl("https://..."):extract("starship"))

-- Binario descargado
curl("https://..."):extract("tool"):to("~/.local/bin/tool"):version("v1.0"):arch({ x86_64 = "amd64" })

-- Repositorio git
git("https://github.com/user/repo.git"):to("~/repo"):at("v1.0"):post("make install")
```

### 9.5 Plugins

```lua
-- init.lua: cargar plugins integrados y personalizados
return {
  plugins = { "dots.http", "dots.archive", "dots.git", "helpers" },
}

-- dots.lua: usar plugins
http.download("https://example.com/file.tar.gz", "/tmp/file.tar.gz")
archive.extract_tar("/tmp/file.tar.gz", "/tmp/out", "bin/tool")
git.clone("https://github.com/user/repo.git", "~/repo")
```

---

## 10. Notas sobre compatibilidad

### 10.1 `config.lua` como marker alternativo

Además de `init.lua`, `dots` también reconoce `config.lua` como archivo
marker para repositorios Lua. Si existe `config.lua` pero no `init.lua`,
el repo se detecta igualmente como repositorio Lua.

> `init.lua` tiene prioridad sobre `config.lua` si ambos existen.

### 10.2 Coexistencia con formatos legacy

`dots` soporta tres formatos de repositorio simultáneamente:

| Marker | Formato | Estado |
|--------|---------|--------|
| `init.lua` | Lua | **Recomendado** |
| `.dots/config.yaml` | YAML (v3) | Soportado |
| `dots.toml` | TOML | Legacy |

Los módulos individuales también pueden coexistir: módulos con `dots.lua`
(Lua) y módulos con `path.yaml` (YAML) en el mismo repo.

---

## Apéndice: Ejemplos completos

### Ejemplo 1: Editor minimalista

```lua
-- ~/dotfiles/Nvim/dots.lua
return {
  type = "editor",
  files = {
    file("init.lua", "~/.config/nvim/init.lua"),
    file("lazy-lock.json", "~/.config/nvim/lazy-lock.json"):when("linux"),
    file("neovide.toml", "~/.config/neovide/config.toml"):per_os({
      linux = "~/.config/neovide/config.toml",
      mac   = "~/Library/Application Support/neovide/config.toml",
    }),
  },
  dependencies = {
    pkg "neovim",
    pkg("lazygit"):on({ pacman = "lazygit", brew = "lazygit" }),
  },
}
```

### Ejemplo 2: Terminal con temas

```lua
-- ~/dotfiles/Kitty/dots.lua
return {
  type = "terminal",
  files = {
    file("kitty.conf", "~/.config/kitty/kitty.conf"),
    dir("themes"):into("~/.config/kitty/themes"),
  },
  dependencies = {
    pkg "kitty",
    curl("https://github.com/dexpota/kitty-themes/archive/master.tar.gz")
      :extract("kitty-themes-master/themes")
      :to("~/.config/kitty/themes")
      :version("latest"),
  },
}
```

### Ejemplo 3: Configuración multi-OS

```lua
-- ~/dotfiles/Alacritty/dots.lua
return {
  type = "terminal",
  files = {
    -- Archivo con destino diferente en cada OS
    file("alacritty.toml", "~/.config/alacritty/alacritty.toml"):per_os({
      linux   = "~/.config/alacritty/linux.toml",
      mac     = "~/Library/Application Support/alacritty/mac.toml",
      windows = "~/AppData/Roaming/alacritty/win.toml",
    }),
    -- Solo en Linux
    dir("linux-scripts"):into("~/.local/bin"):when("linux"),
    -- Solo en Mac
    file("mac-fonts.xml", "~/Library/Fonts/jetbrains.xml"):when("mac"),
  },
  dependencies = {
    pkg("alacritty"):on({
      pacman = "alacritty",
      brew   = "alacritty",
    }),
  },
}
```

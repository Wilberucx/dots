# dots

Gestor de dotfiles — declarativo, basado en symlinks, tuyo.

> **English version**: [README.md](README.md)

## Instalación

### Estable (recomendado)

```bash
curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash
```

Cero dependencias — solo curl o wget. Descarga el binario Go precompilado desde GitHub Releases y lo instala en `~/.local/bin/dots`.

### Via Go

```bash
go install github.com/Wilberucx/dots/cmd/dots@latest
```

Requiere el toolchain de Go.

### Desarrollo

```bash
cd ~/Work/dots
go build -o /tmp/dots ./cmd/dots/
/tmp/dots --help
```

## Configuración inicial

```bash
cd ~/dotfiles
dots init
dots link
```

Eso es todo. Dos comandos y tus dotfiles están enlazados.

---

## Comandos

| Comando             | Descripción                                         |
| ------------------- | --------------------------------------------------- |
| `dots init`         | Inicializa el repo — crea `init.lua` o `.dots/config.yaml` |
| `dots link`         | Crea symlinks para todos los módulos                |
| `dots plan`         | Muestra lo que haría `dots link` sin modificar nada |
| `dots unlink`       | Elimina symlinks                                    |
| `dots status`       | Muestra el estado de enlace agrupado               |
| `dots list`         | Lista módulos o backups con filtros                 |
| `dots edit`         | Abre la carpeta del módulo o archivo de configuración en $EDITOR |
| `dots adopt <path>` | Importa una configuración existente al repo         |
| `dots install`      | Instala dependencias desde los archivos de configuración |
| `dots doctor`       | Diagnóstico profundo de tu configuración de dotfiles |
| `dots backup`       | Git commit y push opcional                          |
| `dots backup run`   | Ejecuta un backup — git add, commit y push opcional |
| `dots backup list`  | Lista backups recientes del historial de git        |
| `dots backup diff`  | Muestra diff desde el último backup o un ref específico |
| `dots completion`   | Genera scripts de completado para el shell (bash/zsh/fish/powershell) |
| `dots version`      | Muestra la versión                                   |

## Ejemplos rápidos

```bash
# Enlazar todo
dots link

# Enlazar módulos específicos
dots link -m Zsh -m Nvim

# Verificar qué está enlazado
dots status

# Filtrar por estado
dots status --state unlinked

# Filtrar por tipo
dots status --type editor

# Diagnóstico profundo
dots doctor

# Importar una configuración existente
dots adopt ~/.zshrc

# Instalar todas las dependencias
dots install

# Instalar solo las de un módulo
dots install -m Zsh

# Vista previa sin ejecutar
dots link --dry-run
dots plan

# Listar módulos
dots list
dots list --variant

# Editar un módulo
dots edit Zsh
dots edit Nvim --config   # abre el archivo de configuración directamente

# Backup
dots backup run
dots backup list --limit 5
dots backup diff --ref HEAD~3

# Salida para scripting
dots status --porcelain
dots status --format json
dots plan --porcelain
dots plan --format table

# Completado para el shell
dots completion bash > /etc/bash_completion.d/dots
```

---

## Flags

| Flag                 | Descripción                                                                       |
| -------------------- | --------------------------------------------------------------------------------- |
| `-m / --module`      | Filtrar por nombre de módulo (repetible)                                           |
| `-t / --type`        | Filtrar por tipo de módulo (repetible)                                             |
| `-s / --state`       | Filtrar por estado: `linked`, `unlinked`, `broken`, `missing`, `unsafe` (repetible) |
| `-f / --format`      | Formato de salida: `default`, `table`, `json`, `porcelain` (`status`, `plan`)     |
| `--backups`          | Mostrar solo archivos con backup .orig (`status`, `list`)                          |
| `--linked`           | Mostrar módulos enlazados (solo para `list`)                                      |
| `--unlinked`         | Mostrar módulos desenlazados (solo para `list`)                                   |
| `--broken`           | Mostrar módulos rotos (solo para `list`)                                          |
| `--force`            | Sobrescribir symlinks existentes en conflicto (`link`, `plan`)                     |
| `--variant`          | Seleccionar variante para módulos con múltiples variantes (`link`, `list`, `plan`) |
| `-i / --interactive` | Seleccionar módulos interactivamente para enlazar/desenlazar (`link`, `unlink`)    |
| `-y / --yes`         | Saltar confirmación (`install`)                                                    |
| `-m / --message`     | Mensaje de commit (`backup run`)                                                   |
| `--ref`             | Ref de git para comparar contra HEAD (`backup diff`) — default: `HEAD~1`           |
| `-n / --limit`      | Número de backups a mostrar (`backup list`) — default: 10                          |
| `--no-push`         | Saltar push al remoto después del commit (`backup run`)                            |
| `--no-sync`         | Saltar verificación de sincronización remota (`backup run`)                        |
| `--no-verify`       | Saltar hooks de git durante el commit (`backup run`)                               |
| `-C / --config`      | Editar el archivo de configuración del módulo (`dots.lua`/`path.yaml`) en vez de la carpeta (`edit`) |
| `--porcelain`        | Salida tabulada apta para scripting (`status`, `plan`) — invalida `--format`       |
| `--hints`            | Mostrar sugerencias de migración (YAML → Lua) (`doctor`) — default: true           |
| `--dry-run`          | Vista previa sin ejecutar                                                          |
| `--no-hints`         | Suprimir sugerencias de migración del syntax checker (persistente)                |

---

## Configuración via `init.lua`

Puedes personalizar comportamientos por defecto desde `init.lua` en la raíz de tu repo.
Todos los campos son opcionales.

```lua
-- ~/dotfiles/init.lua
return {
  name = "usuario/dotfiles",

  -- Formato de salida por defecto por comando
  -- Los flags CLI (--format, --porcelain) siempre sobreescriben estas configuraciones
  output = {
    status = "table",     -- "default" | "table" | "json" | "porcelain"
    plan   = "json",      -- "default" | "table" | "json" | "porcelain"
  },
}
```

Ver [Referencia de sintaxis Lua](docs/sintaxis-lua.md) para la guía completa.

---

## Resolución de conflictos

| Situación                          | Comportamiento                              |
| ---------------------------------- | ------------------------------------------- |
| El archivo existe, no es symlink   | Crea archivo `.orig`, luego enlaza          |
| El symlink existe, apunta a otro lado | Reemplaza con el symlink correcto         |
| El symlink ya es correcto          | Omite — ya está enlazado                    |
| El archivo `.orig` ya existe       | Bloquea — se requiere intervención manual   |

---

## Documentación

### Lua (recomendado)

- [Referencia de sintaxis Lua](docs/sintaxis-lua.md) — configuración `init.lua` / `dots.lua`
- [Sistema de plugins](docs/sintaxis-lua.md#7-sistema-de-plugins) — extender dots con scripts Lua

### YAML heredado (⚠️ Deprecado — eliminar en v0.20.0)

- [Referencia de path.yaml](docs/path-yaml-reference.md) — estructura de módulos, tipos de dependencias
- [Schema v3](docs/schema-v3.md) — especificación del esquema actual
- [Dependencias](docs/dependencies.md) — tipos de dependencias (git, binary, package)

---

## Flujo de trabajo Git

```
main  ← estable, releases con tag
dev   ← desarrollo activo
```

```bash
# Empezar a trabajar
git checkout dev

# Publicar release
git checkout main
git merge dev
# Actualizar versión
git tag v0.x.x
git push origin main --tags
```

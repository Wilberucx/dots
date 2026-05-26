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
| `dots unlink`       | Elimina symlinks                                    |
| `dots status`       | Muestra el estado de enlace agrupado               |
| `dots list`         | Lista módulos o backups con filtros                 |
| `dots edit`         | Abre la carpeta del módulo o archivo de configuración en $EDITOR |
| `dots adopt <path>` | Importa una configuración existente al repo         |
| `dots install`      | Instala dependencias desde los archivos de configuración |
| `dots migrate`      | Migra configuración entre versiones de esquema      |
| `dots backup`       | Git commit y push opcional                          |

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

# Importar una configuración existente
dots adopt ~/.zshrc

# Instalar todas las dependencias
dots install

# Instalar solo las de un módulo
dots install -m Zsh

# Vista previa sin ejecutar
dots link --dry-run

# Listar módulos
dots list
dots list --variant

# Editar un módulo
dots edit Zsh
dots edit Nvim --config   # abre el archivo de configuración directamente
```

---

## Flags

| Flag                 | Descripción                                                                       |
| -------------------- | --------------------------------------------------------------------------------- |
| `-m / --module`      | Filtrar por nombre de módulo (repetible)                                           |
| `-t / --type`        | Filtrar por tipo de módulo (repetible)                                             |
| `-s / --state`       | Filtrar por estado: `linked`, `unlinked`, `broken`, `missing`, `unsafe` (repetible) |
| `-f / --format`      | Formato de salida: `default`, `table`, `json` (solo para `status`)                 |
| `--force`            | Sobrescribir symlinks existentes en conflicto (solo para `link`)                   |
| `--variant`          | Seleccionar variante para módulos con múltiples variantes (solo para `link`)       |
| `-i / --interactive` | Seleccionar módulos interactivamente para enlazar/desenlazar (`link`, `unlink`)    |
| `--dry-run`          | Vista previa sin ejecutar                                                          |

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

### YAML heredado

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

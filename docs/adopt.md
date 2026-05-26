# Comando `adopt`

Importa un archivo de configuración existente al repositorio de dots y lo registra en la configuración del módulo (`dots.lua` o `path.yaml` legacy).

## 1. Qué es `adopt`

`adopt` toma un archivo de configuración que ya existe en tu sistema (ej: `~/.zshrc`, `~/.config/alacritty.yml`) y lo mueve al repositorio de dots, registrándolo para que dots pueda gestionarlo con symlinks.

**Antes de adopt:**
```
~/.zshrc  ← archivo disperso en tu HOME
```

**Después de adopt:**
```
~/dots/Zsh/zshrc          ← archivo centralizado en el repo
~/.zshrc → ~/dots/Zsh/zshrc  ← symlink gestionado por dots
```

## 2. Flujo básico

```bash
# Adopción simple (se crea módulo automáticamente)
dots adopt ~/.zshrc

# Especificar nombre del módulo
dots adopt ~/.zshrc --name Zsh

# Adopción de directorio completo
dots adopt ~/.config/alacritty/
```

**Output esperado (módulo Lua nuevo):**
```
Adopting Configuration
✓ Moved zshrc → Zsh/
✓ Created Zsh/dots.lua
ℹ Run dots link -m Zsh to create the symlink.
```

> **Nota**: Si el módulo usa `path.yaml` legacy, el output dirá `Created Zsh/path.yaml` y `Updated Zsh/path.yaml`.

Si el módulo ya existía, el output será:
```
Adopting Configuration
✓ Moved zshrc → Zsh/
✓ Updated Zsh/dots.lua
ℹ Run dots link -m Zsh to create the symlink.
```

**`dots.lua` resultante:**
```lua
return {
  type = "minimal",
  files = {
    file("zshrc", "~/.zshrc"),
  },
}
```

> Si el módulo usa `path.yaml` legacy, el archivo generado será:
> ```yaml
> files:
>   - source: zshrc
>     destination: ~/.zshrc
> ```

## 3. Qué hace internamente

Cada vez que ejecutás `adopt`, el comando realiza estos pasos en orden:

1. **Validación de seguridad**: Verifica que el archivo esté dentro de `$HOME`. Si está fuera, pide confirmación explícita.

2. **Detección de symlinks existentes**: Si el archivo ya es un symlink apuntando al repo, no lo mueve — solo agrega una entrada a la configuración del módulo.

3. **Determinación del módulo**: Si no especificás `--name`, pide el nombre interactivamente (default: `path.name` capitalizado — solo el nombre del archivo, no la ruta completa). Por ejemplo, si hacés `dots adopt ~/.config/alacritty/alacritty.yml`, el default sería `Alacritty.yml`.

4. **Detección del destino**: Convierte la ruta absoluta en una ruta relativa a `~` (ej: `/home/user/.zshrc` → `~/.zshrc`).

5. **Movimiento del archivo**: Mueve el archivo desde su ubicación original al directorio del módulo en el repositorio.

6. **Registro en `dots.lua`** (o `path.yaml` legacy): Agrega una entrada `file()` al archivo Lua del módulo.

7. **Transacción segura**: Si algo falla durante el proceso, se hace rollback (el archivo vuelve a su lugar original).

## 4. `--dry-run` para previsualizar

Si no estás seguro de qué va a pasar, usá `--dry-run`:

```bash
dots adopt ~/.zshrc --name Zsh --dry-run
```

**Output:**
```
Adopting Configuration
ℹ [DRY] Would create directory: /home/user/dots/Zsh
ℹ [DRY] Would move /home/user/.zshrc → /home/user/dots/Zsh/zshrc
ℹ [DRY] Would create Zsh/dots.lua with file("zshrc", "~/.zshrc")
```

`--dry-run` no toca nada. Solo te muestra exactamente qué haría el comando.

## 5. Adopt inteligente: cuando el destino ya está declarado

Este es el comportamiento "inteligente" de `adopt`.

### 5.1 Detecta conflicto

Cuando ejecutás `adopt` y el módulo ya existe **y** ese módulo ya declara el mismo destino en su `dots.lua` (o `path.yaml` legacy):

```bash
# Supongamos que el módulo Zsh ya declara ~/.zshrc
dots adopt ~/.zshrc --name Zsh
```

**dots detecta:**
- El módulo `Zsh` existe
- `~/.zshrc` ya está declarado como destination en `Zsh/dots.lua`

### 5.2 Ofrece crear variant

```
Adopting Configuration
ℹ Module Zsh already declares ~/.zshrc as a destination.
? Create a new variant in 'Zsh' for this file? [Y/n]:
```

Si respondés `n`, el comando se cancela con el mensaje `Adoption cancelled.`. Si respondés `Y` (default), continúa.

### 5.3 Flujo de creación de variant

```
? Variant name (will be used as subfolder): [default: zshrc]
```

El default es el stem del nombre del archivo. Por ejemplo, si adoptaste `~/.config/alacritty/alacritty.yml`, el variant default sería `alacritty`.

**Dry-run con variant:**
```bash
dots adopt ~/.zshrc --name Zsh --dry-run
```

**Output:**
```
Adopting Configuration
ℹ Module Zsh already declares ~/.zshrc as a destination.
ℹ [DRY] Would create: Zsh/work/
ℹ [DRY] Would move /home/user/.zshrc → /home/user/dots/Zsh/work/zshrc
ℹ [DRY] Would add variant entry to Zsh/dots.lua: file("work/zshrc", "~/.zshrc"):variant("work")
```

**Estructura resultante:**
```
Zsh/
├── zshrc              ← archivo original (variant por defecto)
└── work/
    └── zshrc          ← nuevo variant "work"
```

**`dots.lua` actualizado:**
```lua
return {
  type = "minimal",
  files = {
    file("zshrc", "~/.zshrc"),
    file("work/zshrc", "~/.zshrc"):variant("work"),
  },
}
```

**Output final:**
```
✓ Moved zshrc → Zsh/work/
ℹ Added variant 'work' to Zsh/dots.lua
ℹ Run dots link -m Zsh --variant work to activate this variant.
```

> **Nota**: Con `path.yaml` legacy, la variante se agrega como un segundo entry `source` con el mismo `destination` (variante implícita).

## 6. Ejemplo completo del flujo

### Paso 1: Estado inicial

Tenés un archivo disperso en tu HOME:

```bash
$ ls -la ~/.zshrc
-rw-r--r--  1 user  user  1234 Jan 15 10:00  /home/user/.zshrc
```

### Paso 2: Adopt normal

```bash
$ dots adopt ~/.zshrc --name Zsh
```

**Output:**
```
Adopting Configuration
✓ Moved zshrc → Zsh/
✓ Created Zsh/dots.lua
ℹ Run dots link -m Zsh to create the symlink.
```

**`dots.lua`:**
```lua
return {
  type = "minimal",
  files = {
    file("zshrc", "~/.zshrc"),
  },
}
```

**Estructura del repositorio:**
```
dots/
└── Zsh/
    ├── zshrc
    └── dots.lua
```

### Paso 3: Link para activar

```bash
$ dots link -m Zsh
```

**Output:**
```
Linking Zsh
  ✓ ~/.zshrc → ~/dots/Zsh/zshrc
```

### Paso 4: En otra máquina o contexto, adoptás un segundo zshrc

```bash
$ dots adopt ~/work_zshrc --name Zsh
```

**dots detecta el conflicto y pregunta:**
```
Adopting Configuration
ℹ Module Zsh already declares ~/.zshrc as a destination.
? Create a new variant in 'Zsh' for this file? [Y/n]: Y
? Variant name (will be used as subfolder): work
```

**Output final:**
```
✓ Moved work_zshrc → Zsh/work/
ℹ Added variant 'work' to Zsh/dots.lua
ℹ Run dots link -m Zsh --variant work to activate this variant.
```

**`Zsh/dots.lua` actualizado:**
```lua
return {
  type = "minimal",
  files = {
    file("zshrc", "~/.zshrc"),
    file("work/zshrc", "~/.zshrc"):variant("work"),
  },
}
```

**Estructura del repositorio:**
```
dots/
└── Zsh/
    ├── dots.lua
    ├── zshrc
    └── work/
        └── zshrc
```

### Paso 5: Estado con variants

```bash
$ dots status
```

**Output:**
```
✔ Linked (1 modules)
  Zsh
    ● zshrc  ← active
    ○ work
```

## 7. Casos edge

### Edge 1: Archivo no existe

```bash
$ dots adopt ~/.no_existe --name Test
```

**Error:**
```
Error: Argument 'path': Path '~/.no_existe' does not exist.
```

El archivo debe existir antes de hacer adopt. No hay forma de adoptar algo que no existe.

### Edge 2: Módulo no existe → se crea automáticamente

```bash
$ dots adopt ~/.config/htop/htoprc --name Htop
```

**Output:**
```
Adopting Configuration
✓ Moved htoprc → Htop/
✓ Created Htop/dots.lua
ℹ Run dots link -m Htop to create the symlink.
```

No necesitás crear el módulo previamente. `adopt` lo crea automáticamente.

### Edge 3: El archivo ya está en el repositorio

```bash
$ dots adopt ~/dots/Zsh/zshrc --name Zsh
```

**Error:**
```
Error: /home/user/dots/Zsh/zshrc already exists in repo.
```

No se puede adoptar un archivo que ya está siendo gestionado por dots.

### Edge 4: Archivo fuera de HOME

```bash
$ dots adopt /etc/some_config.conf --name System
```

**Warning:**
```
⚠ /etc/some_config.conf is outside HOME.
? Proceed anyway? [y/N]:
```

dots advertirá, pero te permite proceder si confirmás. Archivos fuera de HOME no pueden ser restaurados via symlink a su ubicación original (porque `~` no aplica).

### Edge 5: Variant ya existe

```bash
# Intentás crear un variant "work" cuando ya existe
$ dots adopt ~/another_work_zshrc --name Zsh
# (respondés Y a crear variant, ponés "work" como nombre)
```

**Error:**
```
Error: /home/user/dots/Zsh/work/zshrc already exists in repo.
```

No se sobrescriben archivos existentes. Tenés que usar otro nombre de variant.

### Edge 6: `dots.lua` o `path.yaml` malformado

Si el archivo de configuración del módulo tiene errores, `adopt` trata de cargarlo. Si falla, usa una estructura vacía `{"files": []}` y sobrescribe el archivo. **Recomendación**: mantené backups de tu configuración de módulo si tiene contenido complejo.

## 8. Compatibilidad con módulos legacy

`adopt` detecta automáticamente si el módulo usa `dots.lua` (Lua) o `path.yaml` (YAML legacy):

| Config del módulo | Output de adopt |
|---|---|
| `dots.lua` | Agrega `file("source", "~/.dest")` al array `files` |
| `path.yaml` (legacy) | Agrega `source` / `destination` al YAML |
| Ninguno (módulo nuevo) | Crea `dots.lua` (recomendado) |

Para migrar un módulo legacy a Lua, usá `dots migrate -m <module>`.

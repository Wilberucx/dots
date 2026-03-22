# Comando `adopt`

Importa un archivo de configuración existente al repositorio de dots y lo registra en `path.yaml`.

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

**Output esperado:**
```
Adopting Configuration
✓ Moved zshrc → Zsh/
✓ Created Zsh/path.yaml
ℹ Run dots link -m Zsh to create the symlink.
```

Si el módulo ya existía, el output será:
```
Adopting Configuration
✓ Moved zshrc → Zsh/
✓ Updated Zsh/path.yaml
ℹ Run dots link -m Zsh to create the symlink.
```

**`path.yaml` resultante:**
```yaml
files:
  - source: zshrc
    destination: ~/.zshrc
```

## 3. Qué hace internamente

Cada vez que ejecutás `adopt`, el comando realiza estos pasos en orden:

1. **Validación de seguridad**: Verifica que el archivo esté dentro de `$HOME`. Si está fuera, pide confirmación explícita.

2. **Determinación del módulo**: Si no especificás `--name`, pide el nombre interactivamente (default: `path.name` capitalizado — solo el nombre del archivo, no la ruta completa). Por ejemplo, si hacés `dots adopt ~/.config/alacritty/alacritty.yml`, el default sería `Alacritty.yml`.

3. **Detección del destino**: Convierte la ruta absoluta en una ruta relativa a `~` (ej: `/home/user/.zshrc` → `~/.zshrc`).

4. **Movimiento del archivo**: Mueve el archivo desde su ubicación original al directorio del módulo en el repositorio.

5. **Registro en `path.yaml`**: Agrega una entrada `source` / `destination` al archivo YAML del módulo.

6. **Transacción segura**: Si algo falla durante el proceso, se hace rollback (el archivo vuelve a su lugar original).

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
ℹ [DRY] Would create Zsh/path.yaml with destination=~/.zshrc
```

`--dry-run` no toca nada. Solo te muestra exactamente qué haría el comando.

## 5. Adopt inteligente: cuando el destino ya está declarado

Este es el comportamiento "inteligente" de `adopt`.

### 5.1 Detecta conflicto

Cuando ejecutás `adopt` y el módulo ya existe **y** ese módulo ya declara el mismo destino en su `path.yaml`:

```bash
# Supongamos que el módulo Zsh ya declara ~/.zshrc
dots adopt ~/.zshrc --name Zsh
```

**dots detecta:**
- El módulo `Zsh` existe
- `~/.zshrc` ya está declarado como destination en `Zsh/path.yaml`

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
ℹ [DRY] Would add variant entry to Zsh/path.yaml: source=work/zshrc, destination=~/.zshrc
```

**Estructura resultante:**
```
Zsh/
├── zshrc              ← archivo original (variant por defecto)
└── work/
    └── zshrc          ← nuevo variant "work"
```

**`path.yaml` actualizado:**
```yaml
files:
  - source: zshrc
    destination: ~/.zshrc
  - source: work/zshrc   # ← variant creado por adopt
    destination: ~/.zshrc
```

**Output final:**
```
✓ Moved zshrc → Zsh/work/
ℹ Added variant 'work' to Zsh/path.yaml
ℹ Run dots link -m Zsh --variant work/zshrc to activate this variant.
```

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
✓ Created Zsh/path.yaml
ℹ Run dots link -m Zsh to create the symlink.
```

**`Zsh/path.yaml`:**
```yaml
files:
  - source: zshrc
    destination: ~/.zshrc
```

**Estructura del repositorio:**
```
dots/
└── Zsh/
    └── zshrc
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
ℹ Added variant 'work' to Zsh/path.yaml
ℹ Run dots link -m Zsh --variant work/zshrc to activate this variant.
```

**`Zsh/path.yaml` actualizado:**
```yaml
files:
  - source: zshrc
    destination: ~/.zshrc
  - source: work/zshrc
    destination: ~/.zshrc
```

**Estructura del repositorio:**
```
dots/
└── Zsh/
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
✓ Created Htop/path.yaml
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

### Edge 6: `path.yaml` está malformado

Si el `path.yaml` del módulo tiene errores de sintaxis YAML:

```bash
$ dots adopt ~/.zshrc --name BrokenModule
```

`adopt` trata de cargar el YAML. Si falla, usa una estructura vacía `{"files": []}` y sobrescribe el archivo. **Recomendación**: mantené backups de `path.yaml` si tiene contenido complejo.

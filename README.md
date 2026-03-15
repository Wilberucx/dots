# dots

CLI for managing dotfiles across Linux, macOS, and Windows.

---

## Requirements

- Python 3.10+
- [`pipx`](https://pipx.pypa.io) (for stable installation)

---

## Setup: dotfiles repository

`dots` needs a **marker file** at the root of your dotfiles repository to locate it. You can generate this automatically:

```bash
cd ~/your-dotfiles
dots init
```

This creates a `dots.toml` file in that directory, and also **writes a `~/.dotsrc` file in your home directory** storing the absolute path to your repo. From then on, you can run `dots` from within that directory (or any subdirectory).

> **Global Usage**: Because `dots init` creates a `~/.dotsrc` pointing to your repo, you can run `dots` from **anywhere on your system**. If you ever move your dotfiles repo, simply run `dots init` again in the new location. You can also override the path dynamically by setting the `DOTS_REPO` environment variable.

---

## Installation

### Stable (production) — via pipx

Install once. Only updates when you explicitly run `pipx upgrade`.
Your dotfiles are **never affected** by development work you do in the `dev` branch.

```bash
pipx install /home/cantoarch/Work/dots
```

To upgrade after a new release (merged to `main` and tagged):

```bash
pipx upgrade dots
# or reinstall from local path:
pipx reinstall dots
```

### Development — via local venv

```bash
cd ~/Work/dots
python3 -m venv .venv
.venv/bin/pip install -e ".[dev]"

# Use the dev binary explicitly:
.venv/bin/dots --help
```

---

## Git Workflow

```
main  ←── stable, tagged releases (v0.1.0, v0.2.0, ...)
dev   ←── active development
```

### Starting new work

```bash
git checkout dev
# ... make changes, run tests ...
.venv/bin/pytest tests/ -v
```

### Releasing a new version

```bash
git checkout main
git merge dev
# Bump version in pyproject.toml
git tag v0.x.x
git push origin main --tags

# Update stable installation:
pipx reinstall dots
```

---

## Usage

```bash
dots --help
dots init
dots status
dots link
dots unlink
```

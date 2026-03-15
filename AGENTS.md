# Agent Guidelines (AGENTS.md)

This document provides context for AI developer agents modifying this repository.

## Branching & Release Strategy

We follow a strict `dev` / `main` isolation strategy:

- **`main` branch**: Contains only **STABLE** code. Code merged to `main` must be tagged as a release (e.g., `v0.1.0`, `v0.2.0`). The end user's system installs from this branch globally using `pipx`.
- **`dev` branch**: All development MUST happen here. This is the integration branch where new features, bug fixes, and refactors are introduced.

### Development Workflow for Agents

1. **Verify you are in `dev`**: 
   - Never commit directly to `main` unless you are instructed to perform a release merge.
   - Run `git branch` to confirm. If not in `dev`, `git checkout dev`.

2. **Making Changes**:
   - Write code.
   - Guarantee the CLI dependencies and code are robust (Python 3.10+).
   - Use the local virtual environment `.venv` to run arbitrary test commands (e.g. `.venv/bin/pytest`).

3. **Releasing (When asked to finish/release)**:
   - Make sure all development is committed in `dev`.
   - `git checkout main` -> `git merge dev`
   - Update `pyproject.toml` version if necessary.
   - Tag the release: `git tag vX.Y.Z`
   - Prompt the user to update their global binary (`pipx reinstall dots` or `pipx upgrade dots`).

## Core Architecture Guidelines

- `dots` does NOT rely on hardcoded application names (e.g., "Alacritty", "Zsh").
- The location of the user's dotfiles repository is identified exclusively by a marker file named `dots.toml`.
- Any filesystem search algorithm must respect `dots.toml` and should not make assumptions about directory layouts.
- Read `src/dots/core/config.py` for context on configuration.

**Do not deviate from these architectural rules.**

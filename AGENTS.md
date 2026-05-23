# Agent Guidelines (AGENTS.md)

This document provides context for AI developer agents modifying this repository.

## Branching & Release Strategy

We follow a strict `dev` / `main` isolation strategy:

- **`main` branch**: Contains only **STABLE** code. Code merged to `main` must be tagged as a release (e.g., `v0.1.0`, `v0.2.0`). End users install from the latest GitHub Release.
- **`dev` branch**: All development MUST happen here. This is the integration branch where new features, bug fixes, and refactors are introduced.

### Development Workflow for Agents

1. **Verify you are in `dev`**: 
   - Never commit directly to `main` unless you are instructed to perform a release merge.
   - Run `git branch` to confirm. If not in `dev`, `git checkout dev`.

2. **Making Changes**:
   - Write Go code (no Python — the Python port is archived).
   - Run tests with `go test ./...`.
   - Build with `go build ./...`.

3. **Releasing (When asked to finish/release)**:
   - Make sure all development is committed in `dev`.
   - `git checkout main` -> `git merge dev`
   - Update version in `internal/cli/root.go` (the `Version` variable) if necessary.
   - Tag the release: `git tag vX.Y.Z`
   - Trigger the GitHub Action to build and upload binaries to the release.

## Core Architecture Guidelines

- `dots` does NOT rely on hardcoded application names (e.g., "Alacritty", "Zsh").
- The location of the user's dotfiles repository is identified exclusively by a marker file named `dots.toml`.
- Any filesystem search algorithm must respect `dots.toml` and should not make assumptions about directory layouts.
- Read `internal/config/config.go` for context on configuration.

**Do not deviate from these architectural rules.**

## Feature Tracking

Feature tasks and pending implementations are tracked in the Obsidian Vault project note:
`dotfile manager dots.md` (in the Vault at `~/ObsidianVaults/Vault-2026/`).

Do not use a local `pending.md` file. All feature decisions, roadmap items,
and pending work lives in that Vault note and should be kept in sync.

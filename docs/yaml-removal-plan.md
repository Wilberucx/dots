# YAML Removal Plan

> **Objective**: Remove all YAML (`path.yaml`, `.dots/config.yaml`) support from dots.
> **Target release**: v0.20.0
> **Previous releases**: v0.12.0 introduced Lua, v0.16.0–v0.19.0 deprecation period

---

## Timeline

| Release | Action |
|---------|--------|
| **v0.16.0** | `dots doctor` warns for each YAML module. Docs updated with timeline. |
| **v0.17.0** | `dots link/status/plan` show `[WARN]` when processing YAML modules. |
| **v0.18.0** | All commands emit deprecation warnings for YAML usage. |
| **v0.19.0** | Last release with YAML support. `dots doctor` shows blocking warning. |
| **v0.20.0** | **YAML support removed.** Only Lua (`init.lua` / `dots.lua`) recognized. |

---

## Files to Remove (in v0.20.0)

### Phase 1 — Remove the YAML parser package

| File | Reason |
|------|--------|
| `internal/yaml/parser.go` | Core YAML parser (`ParsePathYAML`, `ParseDependencies`) |
| `internal/yaml/parser_test.go` | Tests for parser |
| `internal/yaml/schema.go` | YAML schema validation (`ValidatePathYAML`, `DetectV2Schema`) |
| `internal/yaml/schema_test.go` | Tests for schema |

**Dependency**: Remove `gopkg.in/yaml.v3` from `go.mod`.

### Phase 2 — Remove YAML module writer

| File | Reason |
|------|--------|
| `internal/writer/module_writer.go` | `LoadModuleData`, `AppendFileEntry`, `WriteConfigYAML` |
| `internal/writer/module_writer_test.go` | Tests for module writer |

**Note**: `WriteConfigYAML` is only used by `dots init` (legacy marker creation). Replace with equivalent Lua init.

### Phase 3 — Remove YAML migration helpers

| File | Reason |
|------|--------|
| `internal/cli/migrate.go` | v2→v3 YAML schema migration helpers (unused by production code) |
| `internal/cli/migrate_test.go` | Tests for migration |

**Note**: These already have no production callers. Safe to remove anytime.

### Phase 4 — Remove YAML module discovery

| File | Changes |
|------|---------|
| `internal/lua/loader_modules.go` | Remove `path.yaml` detection (lines ~87–114). Only look for `dots.lua`. Remove `ConvertYAMLDepsToDepOps`, `ParseYAMLFile`. |
| `internal/lua/loader_modules_test.go` | Update tests to not create path.yaml modules |
| `internal/lua/types.go` | Remove `ModuleTypeYAML` constant (zero value becomes invalid) |
| `internal/lua/migrate.go` | Remove `MigrateModule()`, `generateLuaFromYAML()`, `generateFileEntry()`, `generateDepEntry()`. Keep `InitLuaTemplate` and `WriteInitLua`. |
| `internal/lua/migrate_test.go` | Remove YAML migration tests |

### Phase 5 — Remove YAML from resolver

| File | Changes |
|------|---------|
| `internal/resolver/resolver.go` | Remove `resolveYAMLModule()`, `resolveModuleMappings()`. Remove `yaml` import. Remove `OpType == "yaml"` handling. `GetModuleVariantInfo` becomes Lua-only. |
| `internal/resolver/resolver_test.go` | Update tests to use Lua modules instead of path.yaml |
| `internal/resolver/service.go` | Remove `ParsePathYAML` fallback |

### Phase 6 — Remove YAML from config

| File | Changes |
|------|---------|
| `internal/config/config.go` | Remove `getYAMLModuleDirs()`, `ParseModuleMeta()`, `ModuleDir.Type` (type becomes irrelevant). Remove `"gopkg.in/yaml.v3"` import. Only detect Lua repos. |
| `internal/config/config_test.go` | Remove YAML config tests. Remove `MarkerConfig = "config.yaml"`. |

### Phase 7 — Remove YAML from CLI commands

| File | Changes |
|------|---------|
| `internal/cli/adopt.go` | Remove all `path.yaml` references. Only generate `dots.lua`. |
| `internal/cli/edit.go` | Remove `path.yaml` open fallback. Only open `dots.lua`. |
| `internal/cli/install.go` | Remove YAML dependency loading path. Only use Lua deps. |
| `internal/cli/doctor.go` | Remove "module(s) still using path.yaml" check. |
| `internal/cli/init.go` | Remove `WriteConfigYAML` call. Remove YAML init prompt (Lua only). |

### Phase 8 — Remove YAML from checker

| File | Changes |
|------|---------|
| `internal/checker/checker.go` | Remove `checkPathYAML()`, YAML migration hints. Only check `dots.lua`. |

### Phase 9 — Docs and references

| File | Changes |
|------|---------|
| `docs/path-yaml-reference.md` | Delete |
| `docs/schema-v3.md` | Delete |
| `docs/dependencies.md` | Remove YAML references, keep only Lua dep API |
| `docs/variants.md` | Remove "Declaring Variants in YAML" section |
| `docs/lua-syntax.md` | Remove section 8 (YAML→Lua migration) and section 10.2 (legacy coexistence) |
| `docs/sintaxis-lua.md` | Same as above in Spanish |
| `README.md` | Remove Legacy YAML section from docs index |
| `README.es.md` | Same in Spanish |

---

## Dependency Graph

```
Phase 1 (yaml pkg) ← no deps
Phase 2 (writer)   ← depends on Phase 1
Phase 3 (migrate)  ← no deps (already orphaned)
Phase 4 (lua)      ← depends on Phase 1
Phase 5 (resolver) ← depends on Phase 1, Phase 4
Phase 6 (config)   ← depends on Phase 1
Phase 7 (cli)      ← depends on Phase 2, Phase 4, Phase 5, Phase 6
Phase 8 (checker)  ← depends on Phase 1
Phase 9 (docs)     ← independent, can be done anytime
```

**Recommended order**: Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 6 → Phase 5 → Phase 8 → Phase 7 → Phase 9

---

## Test Strategy Per Phase

Each phase must maintain green tests after removal:
- Run `go build ./...` after each phase
- Run `go test ./...` after each phase
- Run `go vet ./...` after each phase
- Remove YAML test fixtures (create Lua equivalents before removing YAML paths)
- Final e2e test: ensure a repo with only `init.lua` + `dots.lua` works end-to-end

---

## Notes for implementers

- `ModuleTypeYAML = iota` (zero value) means removing it changes zero-value semantics. Refactor to `ModuleTypeLua = 1` or use bool `IsLua`.
- `internal/resolver/resolver.go` function `resolveModuleMappings` is used by both YAML and Lua code paths. Extract Lua-specific logic before removing.
- `internal/cli/install.go` function `loadDependencies` has a YAML branch and a Lua branch. Remove YAML branch entirely.
- `go.mod` dependency `gopkg.in/yaml.v3` affects `internal/yaml/`, `internal/config/`, `internal/writer/`. Remove only after all three are clean.
- After removal, `dots init` should only generate `init.lua` — no more `.dots/config.yaml`.

package resolver

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cantoarch/dots/internal/config"
	"github.com/cantoarch/dots/internal/yaml"
)

// ModuleInfo holds aggregated information about a module.
type ModuleInfo struct {
	Name           string
	LinkState      string // "linked", "unlinked", "broken", "missing"
	Statuses       []LinkStatus
	PrimaryDest    string
	LastBackup     string
	HasVariants    bool
	VariantNames   []string
	ActiveVariant  string
}

// DotsService aggregates module state and provides high-level queries.
type DotsService struct {
	Config    *config.DotsConfig
	Modules   map[string]ModuleInfo
	Names     []string
	Backups   []string
}

// NewDotsService creates a new DotsService with the given config.
func NewDotsService(cfg *config.DotsConfig) *DotsService {
	return &DotsService{
		Config:  cfg,
		Modules: make(map[string]ModuleInfo),
	}
}

// RefreshModules scans all modules and populates the service state.
func (s *DotsService) RefreshModules() error {
	results, err := ResolveModules(s.Config, nil, nil, "")
	if err != nil {
		return err
	}

	s.Modules = make(map[string]ModuleInfo)
	nameSet := make(map[string]bool)

	for _, name := range sortedKeys(results) {
		sts := results[name]
		s.Modules[name] = buildModuleInfo(s.Config, name, sts)
		nameSet[name] = true
	}

	// Also include module dirs without statuses
	modDirs, err := s.Config.GetModuleDirs(nil, nil)
	if err == nil {
		for _, d := range modDirs {
			if !nameSet[d.Name] {
				info := ModuleInfo{
					Name:      d.Name,
					LinkState: "unlinked",
				}
				s.Modules[d.Name] = info
				nameSet[d.Name] = true
			}
		}
	}

	s.Names = sortedModuleKeys(s.Modules)
	return nil
}

// sortedModuleKeys returns sorted keys from a ModuleInfo map.
func sortedModuleKeys(m map[string]ModuleInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RefreshBackups loads recent git log entries.
func (s *DotsService) RefreshBackups() {
	cmd := exec.Command("git", "log", "--oneline", "-30")
	cmd.Dir = s.Config.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		s.Backups = []string{"(git unavailable)"}
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var entries []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			entries = append(entries, strings.TrimSpace(l))
		}
	}

	if len(entries) == 0 {
		s.Backups = []string{"(no git history)"}
		return
	}
	s.Backups = entries
}

// buildModuleInfo aggregates status information for a single module.
func buildModuleInfo(cfg *config.DotsConfig, name string, sts []LinkStatus) ModuleInfo {
	info := ModuleInfo{
		Name:     name,
		Statuses: sts,
	}

	// Aggregate link state
	if len(sts) == 0 {
		info.LinkState = "unlinked"
	} else {
		hasConflict := false
		hasLinked := false
		hasPending := false
		for _, st := range sts {
			switch st.State {
			case StateConflict, StateUnsafe:
				hasConflict = true
			case StateLinked:
				hasLinked = true
			case StatePending:
				hasPending = true
			}
		}
		if hasConflict {
			info.LinkState = "broken"
		} else if hasLinked || hasPending {
			info.LinkState = "linked"
		} else {
			info.LinkState = "unlinked"
		}
	}

	// Primary destination
	if len(sts) > 0 {
		info.PrimaryDest = shortPath(sts[0].Destination, cfg.HomeDir)
	}

	// Last backup via git
	cmd := exec.Command("git", "log", "-1", "--format=%s", "--", name)
	cmd.Dir = cfg.RepoRoot
	if out, err := cmd.Output(); err == nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			info.LastBackup = msg
		}
	}

	// Variant info
	yamlPath := filepath.Join(cfg.RepoRoot, name, "path.yaml")
	if mappings, err := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS); err == nil && mappings != nil {
		variantInfo := yaml.DetectVariants(mappings)
		if variantInfo.HasVariants {
			info.HasVariants = true
			info.VariantNames = variantInfo.Variants

			// Deduce active variant from statuses
			activeVars := make(map[string]bool)
			modDir := filepath.Join(cfg.RepoRoot, name)
			for _, st := range sts {
				rel, err := filepath.Rel(modDir, st.Source)
				if err == nil {
					parts := strings.SplitN(rel, string(filepath.Separator), 2)
					if len(parts) > 0 {
						for _, v := range variantInfo.Variants {
							if parts[0] == v || strings.HasPrefix(parts[0], v) {
								activeVars[v] = true
							}
						}
					}
				}
			}
			if len(activeVars) > 0 {
				for v := range activeVars {
					info.ActiveVariant = v
					break
				}
			} else {
				info.ActiveVariant = variantInfo.DefaultVariant
			}
		}
	}

	return info
}

// sortedKeys returns sorted keys from a map.
func sortedKeys(m map[string][]LinkStatus) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

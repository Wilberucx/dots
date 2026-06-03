// Package plan provides a central abstraction for translating resolved link states
// into actionable operations. It decouples state detection (resolver) from
// execution (CLI commands), ensuring that `dots plan`, `dots link --dry-run`,
// and `dots link` all share the same decision logic.
package plan

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/Wilberucx/dots/internal/resolver"
)

// ActionKind describes the kind of filesystem operation to perform.
type ActionKind string

const (
	ActionCreateSymlink  ActionKind = "create_symlink"  // Create a new symlink (dest → src)
	ActionReplaceSymlink ActionKind = "replace_symlink" // Replace an existing symlink that points elsewhere
	ActionBackupFile     ActionKind = "backup_file"     // Move existing file to .orig before linking
	ActionSkipLinked     ActionKind = "skip_linked"     // Symlink already correct — no action needed
	ActionSkipPending    ActionKind = "skip_pending"    // Not linked and no variant active for this module
	ActionErrorConflict  ActionKind = "error_conflict"  // Conflict without --force — must resolve manually
	ActionErrorUnsafe    ActionKind = "error_unsafe"    // Destination outside $HOME — blocked
	ActionRemoveSymlink  ActionKind = "remove_symlink"  // Remove an existing symlink (for unlink)
)

// Action is a single atomic operation within a Plan.
type Action struct {
	Module      string
	Source      string
	Destination string
	Kind        ActionKind
	State       resolver.LinkState
	Detail      string
	BackupPath  string
}

// Plan holds the complete set of actions derived from resolved module states.
type Plan struct {
	Actions []Action
}

// BuildOptions controls how the plan is constructed from resolved states.
type BuildOptions struct {
	// Force overwrites conflicting symlinks without asking.
	Force bool
	// VariantSwaps indicates modules where a variant swap is happening (implies Force for those modules).
	VariantSwaps map[string]bool
}

// BuildLinkPlan converts resolved module statuses into a Plan of link actions.
// This centralizes the decision logic previously spread across cli/link.go.
func BuildLinkPlan(modules map[string][]resolver.LinkStatus, opts BuildOptions) *Plan {
	p := &Plan{}

	for modName, statuses := range modules {
		// Per-module effective force: global force or variant swap
		effectiveForce := opts.Force
		if opts.VariantSwaps != nil && opts.VariantSwaps[modName] {
			effectiveForce = true
		}

		for _, st := range statuses {
			act := actionFromLinkStatus(modName, st, effectiveForce)
			p.Actions = append(p.Actions, act)
		}
	}

	return p
}

// actionFromLinkStatus translates a single LinkStatus into an Action.
// effectiveForce is true if --force was passed OR this module is being variant-swapped.
func actionFromLinkStatus(modName string, st resolver.LinkStatus, effectiveForce bool) Action {
	base := Action{
		Module:      modName,
		Source:      st.Source,
		Destination: st.Destination,
		State:       st.State,
		BackupPath:  st.BackupPath,
	}

	switch st.State {
	case resolver.StateLinked:
		base.Kind = ActionSkipLinked
		base.Detail = "already linked"

	case resolver.StateUnsafe:
		base.Kind = ActionErrorUnsafe
		base.Detail = st.Detail
		if base.Detail == "" {
			base.Detail = "path outside home directory"
		}

	case resolver.StateMissing:
		base.Kind = ActionSkipPending
		base.Detail = "source file missing"

	case resolver.StateConflict:
		if effectiveForce {
			base.Kind = ActionReplaceSymlink
			base.Detail = "replacing existing symlink"
		} else {
			base.Kind = ActionErrorConflict
			if st.Detail != "" {
				base.Detail = st.Detail
			} else {
				base.Detail = "symlink points elsewhere — use --force to overwrite"
			}
		}

	case resolver.StatePending:
		if st.Detail == "backup needed" {
			backupPath := st.BackupPath
			if backupPath == "" {
				backupPath = st.Destination + ".orig"
			}

			// Use Lstat to detect ANY filesystem entry (file, symlink, broken symlink)
			if fi, err := os.Lstat(backupPath); err == nil {
				// .orig exists (including broken symlink) — cannot proceed
				detail := ".orig backup already exists at " + filepath.Base(backupPath)
				if fi.Mode()&os.ModeSymlink != 0 {
					if _, readErr := os.Stat(backupPath); readErr != nil {
						detail += " (broken symlink)"
					}
				}
				base.Kind = ActionErrorConflict
				base.Detail = detail
				base.BackupPath = backupPath
			} else if !os.IsNotExist(err) {
				// Unexpected stat error — treat as conflict with explicit detail
				base.Kind = ActionErrorConflict
				base.Detail = "cannot check .orig path: " + err.Error()
				base.BackupPath = backupPath
			} else {
				base.Kind = ActionBackupFile
				base.Detail = "backup needed"
				base.BackupPath = backupPath
			}
		} else {
			base.Kind = ActionCreateSymlink
			base.Detail = "will create"
		}
	}

	return base
}

// BuildUnlinkPlan converts resolved module statuses into a Plan of unlink actions.
func BuildUnlinkPlan(modules map[string][]resolver.LinkStatus) *Plan {
	p := &Plan{}

	for modName, statuses := range modules {
		for _, st := range statuses {
			act := Action{
				Module:      modName,
				Source:      st.Source,
				Destination: st.Destination,
				State:       st.State,
				BackupPath:  st.BackupPath,
			}

			switch st.State {
			case resolver.StateLinked:
				act.Kind = ActionRemoveSymlink
				act.Detail = "will remove"

			case resolver.StateConflict, resolver.StateUnsafe:
				act.Kind = ActionErrorConflict
				act.Detail = "conflict or unsafe — skipping"

			case resolver.StatePending, resolver.StateMissing:
				act.Kind = ActionSkipPending
				act.Detail = "not linked"

			default:
				act.Kind = ActionSkipPending
				act.Detail = "no action"
			}

			p.Actions = append(p.Actions, act)
		}
	}

	return p
}

// CountByKind returns a map of action kinds to their count.
func (p *Plan) CountByKind() map[ActionKind]int {
	counts := make(map[ActionKind]int)
	for _, a := range p.Actions {
		counts[a.Kind]++
	}
	return counts
}

// FilterMutatingActions returns only actions that change the filesystem
// (create, replace, or backup). Skips errors, skips, and already-linked.
func (p *Plan) FilterMutatingActions() []Action {
	var result []Action
	for _, a := range p.Actions {
		switch a.Kind {
		case ActionCreateSymlink, ActionReplaceSymlink, ActionBackupFile:
			result = append(result, a)
		}
	}
	return result
}

// ModuleNames returns the sorted list of module names in the plan.
func (p *Plan) ModuleNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, a := range p.Actions {
		if !seen[a.Module] {
			seen[a.Module] = true
			names = append(names, a.Module)
		}
	}
	sort.Strings(names)
	return names
}

// ActionsByModule groups actions by module name.
func (p *Plan) ActionsByModule() map[string][]Action {
	result := make(map[string][]Action)
	for _, a := range p.Actions {
		result[a.Module] = append(result[a.Module], a)
	}
	return result
}

package plan

import (
	"os"
	"testing"

	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/stretchr/testify/assert"
)

func TestBuildLinkPlan_Linked(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateLinked},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionSkipLinked, p.Actions[0].Kind)
	assert.Equal(t, "already linked", p.Actions[0].Detail)
}

func TestBuildLinkPlan_Pending(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StatePending, Detail: "will create"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionCreateSymlink, p.Actions[0].Kind)
}

func TestBuildLinkPlan_BackupNeeded(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StatePending, Detail: "backup needed"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionBackupFile, p.Actions[0].Kind)
}

func TestBuildLinkPlan_BackupNeeded_OrigExists(t *testing.T) {
	tempDir := t.TempDir()
	destPath := tempDir + "/.zshrc"
	origPath := destPath + ".orig"

	// Create a regular file at .orig
	if err := os.WriteFile(origPath, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}

	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: destPath, State: resolver.StatePending, Detail: "backup needed", BackupPath: origPath},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionErrorConflict, p.Actions[0].Kind, "existing .orig should cause conflict")
	assert.Contains(t, p.Actions[0].Detail, "already exists")
}

func TestBuildLinkPlan_BackupNeeded_BrokenSymlinkOrig(t *testing.T) {
	tempDir := t.TempDir()
	destPath := tempDir + "/.zshrc"
	origPath := destPath + ".orig"

	// Create a broken symlink as .orig
	if err := os.Symlink("/nonexistent/target", origPath); err != nil {
		t.Fatal(err)
	}

	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: destPath, State: resolver.StatePending, Detail: "backup needed", BackupPath: origPath},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionErrorConflict, p.Actions[0].Kind, "broken symlink .orig should cause conflict")
	assert.Contains(t, p.Actions[0].Detail, "broken symlink")
}

func TestBuildLinkPlan_BackupNeeded_LstatError(t *testing.T) {
	tempDir := t.TempDir()
	destPath := tempDir + "/.zshrc"
	origPath := destPath + ".orig"

	// Create a directory at origPath to make Lstat succeed (it's a dir, not an error)
	// Instead, use a path with permission error: not easily testable in unit test,
	// but we can verify the code path by passing a path that doesn't exist
	// (which should still return backup needed)
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: destPath, State: resolver.StatePending, Detail: "backup needed", BackupPath: origPath},
		},
	}

	// origPath doesn't exist on disk — should fall through to ActionBackupFile
	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionBackupFile, p.Actions[0].Kind, "non-existent .orig should allow backup")
}

func TestBuildLinkPlan_VariantSwapWithPlan(t *testing.T) {
	// When using --variant and the module has an active variant,
	// the plan should produce replace_symlink, not error_conflict.
	// This test verifies that VariantSwaps in BuildOptions works correctly
	// (the detection of active variant is done by the CLI layer).
	modules := map[string][]resolver.LinkStatus{
		"Nvim": {
			{Source: "/repo/Nvim/work/init.lua", Destination: "/home/user/.config/nvim/init.lua", State: resolver.StateConflict, Detail: "points to /repo/Nvim/laptop/init.lua"},
		},
	}

	// Without variant swap — should be error
	p1 := BuildLinkPlan(modules, BuildOptions{})
	assert.Equal(t, ActionErrorConflict, p1.Actions[0].Kind, "without variant swap, conflict should error")

	// With variant swap — should be replace
	p2 := BuildLinkPlan(modules, BuildOptions{VariantSwaps: map[string]bool{"Nvim": true}})
	assert.Equal(t, ActionReplaceSymlink, p2.Actions[0].Kind, "with variant swap, conflict should become replace")
	assert.Equal(t, "replacing existing symlink", p2.Actions[0].Detail)
}

func TestBuildLinkPlan_ConflictWithoutForce(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateConflict, Detail: "points to /other/target"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionErrorConflict, p.Actions[0].Kind)
}

func TestBuildLinkPlan_ConflictWithForce(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateConflict, Detail: "points to /other/target"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{Force: true})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionReplaceSymlink, p.Actions[0].Kind)
}

func TestBuildLinkPlan_ConflictWithVariantSwap(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/work/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateConflict},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{VariantSwaps: map[string]bool{"Zsh": true}})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionReplaceSymlink, p.Actions[0].Kind, "variant swap should force replace")
}

func TestBuildLinkPlan_ConflictWithVariantSwapOtherModule(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateConflict},
		},
		"Nvim": {
			{Source: "/repo/Nvim/init.lua", Destination: "/home/user/init.lua", State: resolver.StateConflict},
		},
	}

	// Only Zsh is being variant-swapped
	p := BuildLinkPlan(modules, BuildOptions{VariantSwaps: map[string]bool{"Zsh": true}})
	assert.Len(t, p.Actions, 2)

	for _, a := range p.Actions {
		if a.Module == "Zsh" {
			assert.Equal(t, ActionReplaceSymlink, a.Kind, "Zsh variant swap should replace")
		} else {
			assert.Equal(t, ActionErrorConflict, a.Kind, "Nvim without swap should be conflict")
		}
	}
}

func TestBuildLinkPlan_Unsafe(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/tmp/.zshrc", State: resolver.StateUnsafe, Detail: "path outside home directory"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionErrorUnsafe, p.Actions[0].Kind)
}

func TestBuildLinkPlan_MultipleModules(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateLinked},
			{Source: "/repo/Zsh/.zshenv", Destination: "/home/user/.zshenv", State: resolver.StatePending, Detail: "will create"},
		},
		"Nvim": {
			{Source: "/repo/Nvim/init.lua", Destination: "/home/user/.config/nvim/init.lua", State: resolver.StateConflict},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	assert.Len(t, p.Actions, 3)

	names := p.ModuleNames()
	assert.ElementsMatch(t, []string{"Zsh", "Nvim"}, names)
}

func TestBuildUnlinkPlan_Linked(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StateLinked},
		},
	}

	p := BuildUnlinkPlan(modules)
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionRemoveSymlink, p.Actions[0].Kind)
}

func TestBuildUnlinkPlan_NotLinked(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/repo/Zsh/.zshrc", Destination: "/home/user/.zshrc", State: resolver.StatePending},
		},
	}

	p := BuildUnlinkPlan(modules)
	assert.Len(t, p.Actions, 1)
	assert.Equal(t, ActionSkipPending, p.Actions[0].Kind)
}

func TestCountByKind(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/a", Destination: "/b", State: resolver.StateLinked},
			{Source: "/c", Destination: "/d", State: resolver.StatePending, Detail: "will create"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	counts := p.CountByKind()
	assert.Equal(t, 1, counts[ActionSkipLinked])
	assert.Equal(t, 1, counts[ActionCreateSymlink])
}

func TestFilterMutatingActions(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/a", Destination: "/b", State: resolver.StateLinked},
			{Source: "/c", Destination: "/d", State: resolver.StatePending, Detail: "will create"},
			{Source: "/e", Destination: "/f", State: resolver.StateConflict},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{Force: false})
	mutatingActions := p.FilterMutatingActions()
	assert.Len(t, mutatingActions, 1)
	assert.Equal(t, ActionCreateSymlink, mutatingActions[0].Kind)
}

func TestFilterMutatingActionsWithForce(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/a", Destination: "/b", State: resolver.StateLinked},
			{Source: "/c", Destination: "/d", State: resolver.StatePending, Detail: "will create"},
			{Source: "/e", Destination: "/f", State: resolver.StateConflict},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{Force: true})
	mutatingActions := p.FilterMutatingActions()
	assert.Len(t, mutatingActions, 2)
}

func TestActionsByModule(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/a", Destination: "/b", State: resolver.StateLinked},
			{Source: "/c", Destination: "/d", State: resolver.StatePending, Detail: "will create"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	byMod := p.ActionsByModule()
	assert.Len(t, byMod, 1)
	assert.Len(t, byMod["Zsh"], 2)
}

func TestModuleNames_Sorted(t *testing.T) {
	modules := map[string][]resolver.LinkStatus{
		"Zsh": {
			{Source: "/a", Destination: "/b", State: resolver.StateLinked},
		},
		"Nvim": {
			{Source: "/c", Destination: "/d", State: resolver.StatePending, Detail: "will create"},
		},
	}

	p := BuildLinkPlan(modules, BuildOptions{})
	names := p.ModuleNames()
	assert.Equal(t, []string{"Nvim", "Zsh"}, names, "module names should be sorted")
}

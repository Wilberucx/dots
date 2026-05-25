package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wilberucx/dots/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	backupRunCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runBackup(cmd)
	}
	backupListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runBackupList(cmd)
	}
	backupDiffCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runBackupDiff(cmd, args)
	}
}

// ─── backup run ──────────────────────────────────────────────────────────────

func runBackup(cmd *cobra.Command) error {
	ui.PrintHeader("Dots Backup")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	noPush := boolFlag(cmd, "no-push")
	noSync := boolFlag(cmd, "no-sync")
	noVerify := boolFlag(cmd, "no-verify")
	message := stringFlag(cmd, "message")

	commitMsg := message
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("backup: %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	dotsDir := cfg.RepoRoot

	if err := runBackupCore(commitMsg, dotsDir, !noPush, noSync, noVerify); err != nil {
		return err
	}
	return nil
}

// runBackupCore performs the core backup logic: pull → add → commit → push.
// Every step runs regardless of state: syncs remote changes, stages everything,
// commits if there are changes, and pushes local commits to remote.
func runBackupCore(commitMsg, dotsDir string, push, noSync, noVerify bool) error {
	// Step 0: Pre-flight check
	ui.PrintInfo("Performing pre-flight checks...")
	if err := preFlightCheck(dotsDir); err != nil {
		return err
	}
	ui.PrintSuccess("Repository is in a valid state")

	// Step 1: Sync from remote
	if noSync {
		ui.PrintInfo("Remote sync disabled (--no-sync)")
	} else {
		syncResult := syncFromRemote(dotsDir)
		switch syncResult.status {
		case "no_upstream":
			ui.PrintInfo("No upstream branch configured — skipping sync")
		case "clean":
			ui.PrintSuccess("Remote is up to date")
		case "pulled":
			ui.PrintSuccess(fmt.Sprintf("Pulled %d commit(s) from remote", syncResult.ahead))
		case "conflicts":
			resolved := resolveConflictsInteractive(dotsDir, syncResult.conflicts)
			if !resolved {
				return fmt.Errorf("sync failed: conflict resolution aborted")
			}
			ui.PrintSuccess("Conflicts resolved successfully")
		case "error":
			if syncResult.errMsg != "" {
				return fmt.Errorf("sync from remote failed: %s", syncResult.errMsg)
			}
			return fmt.Errorf("sync from remote failed")
		}
	}

	// Step 2: Stage all changes (including deletions, tracked files, new files)
	ui.PrintInfo("Staging changes...")
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = dotsDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", strings.TrimSpace(string(out)))
	}
	ui.PrintSuccess("Changes staged (git add -A)")

	// Step 3: Commit if there are changes
	ui.PrintInfo("Checking for changes to commit...")
	hasChanges, err := hasStagedChanges(dotsDir)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}

	if hasChanges {
		commitArgs := []string{"commit", "-m", commitMsg}
		if noVerify {
			commitArgs = append(commitArgs, "--no-verify")
			ui.PrintInfo("Git hooks disabled (--no-verify)")
		}
		ui.PrintInfo(fmt.Sprintf("Committing with message: \"%s\"...", commitMsg))
		commit := exec.Command("git", commitArgs...)
		commit.Dir = dotsDir
		if out, err := commit.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(out)))
		}
		ui.PrintSuccess("Changes committed successfully")
	} else {
		ui.PrintInfo("No changes to commit — working tree is clean")
	}

	// Step 4: Push (always if push is enabled)
	if push {
		remote := getRemoteName(dotsDir)

		pushArgs := []string{"push"}
		if remote != "" {
			pushArgs = append(pushArgs, remote, "HEAD")
		}

		ui.PrintInfo("Pushing to remote...")
		pushCmd := exec.Command("git", pushArgs...)
		pushCmd.Dir = dotsDir
		if out, err := pushCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git push failed: %s", strings.TrimSpace(string(out)))
		}
		ui.PrintSuccess("Changes pushed to remote")
	} else {
		ui.PrintInfo("Push disabled (--no-push)")
	}

	ui.PrintSuccess("Backup completed successfully")
	return nil
}

// ─── Pre-flight checks ───────────────────────────────────────────────────────

func preFlightCheck(dotsDir string) error {
	// Check 1: Is this a valid git repository?
	gitCmd := exec.Command("git", "rev-parse", "--git-dir")
	gitCmd.Dir = dotsDir
	out, err := gitCmd.Output()
	if err != nil {
		return fmt.Errorf("not a git repository — backup requires a git repo at %s", dotsDir)
	}

	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dotsDir, gitDir)
	}

	// Check 2: Any in-progress operations that would conflict
	// Using git rev-parse --git-path for proper worktree-aware path resolution
	dangerOps := []struct {
		file string
		desc string
	}{
		{"rebase-merge", "rebase in progress"},
		{"rebase-apply", "rebase (apply) in progress"},
		{"MERGE_HEAD", "merge in progress"},
		{"CHERRY_PICK_HEAD", "cherry-pick in progress"},
		{"REVERT_HEAD", "revert in progress"},
		{"BISECT_LOG", "bisect in progress"},
	}

	for _, op := range dangerOps {
		pathCmd := exec.Command("git", "rev-parse", "--git-path", op.file)
		pathCmd.Dir = dotsDir
		pathBytes, err := pathCmd.Output()
		if err != nil {
			continue
		}
		path := strings.TrimSpace(string(pathBytes))
		if !filepath.IsAbs(path) {
			path = filepath.Join(dotsDir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("cannot backup: %s — aborting", op.desc)
		}
	}

	return nil
}

// ─── Staged changes detection ────────────────────────────────────────────────

// hasStagedChanges returns true if there are changes in the index.
// Distinguishes between "clean" (0) and "error" (2) exit codes from git diff.
func hasStagedChanges(dotsDir string) (bool, error) {
	diffCmd := exec.Command("git", "diff", "--cached", "--quiet")
	diffCmd.Dir = dotsDir

	if err := diffCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 1:
				// Changes exist
				return true, nil
			default:
				// Real error (exit code 2+)
				return false, fmt.Errorf("git diff failed with exit code %d", exitErr.ExitCode())
			}
		}
		// Non-exit error (command not found, etc.)
		return false, fmt.Errorf("git diff failed: %w", err)
	}

	return false, nil
}

// ─── Remote sync helpers ─────────────────────────────────────────────────────

type syncResult struct {
	status    string   // "clean", "pulled", "conflicts", "no_upstream", "error"
	conflicts []string
	ahead     int
	errMsg    string // populated on error
}

func getUpstreamBranch(dotsDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = dotsDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getRemoteName extracts the remote name from the upstream branch ref.
// Upstream format is "remote/branch" (e.g. "origin/main").
func getRemoteName(dotsDir string) string {
	upstream := getUpstreamBranch(dotsDir)
	if upstream == "" {
		return ""
	}
	parts := strings.SplitN(upstream, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[0]
}

func getRemoteAheadCount(dotsDir, upstream string) int {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("HEAD..%s", upstream))
	cmd.Dir = dotsDir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var count int
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	return count
}

func getConflictFiles(dotsDir string) []string {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = dotsDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func syncFromRemote(dotsDir string) syncResult {
	upstream := getUpstreamBranch(dotsDir)
	if upstream == "" {
		return syncResult{status: "no_upstream"}
	}

	remote := getRemoteName(dotsDir)
	if remote == "" {
		return syncResult{status: "error", errMsg: "could not determine remote name from upstream " + upstream}
	}

	ui.PrintInfo(fmt.Sprintf("Fetching from remote \"%s\"...", remote))
	fetch := exec.Command("git", "fetch", "--quiet", remote)
	fetch.Dir = dotsDir
	if out, err := fetch.CombinedOutput(); err != nil {
		return syncResult{status: "error", errMsg: strings.TrimSpace(string(out))}
	}
	ui.PrintSuccess(fmt.Sprintf("Fetched from %s", remote))

	ahead := getRemoteAheadCount(dotsDir, upstream)
	if ahead == 0 {
		return syncResult{status: "clean", ahead: 0}
	}

	ui.PrintWarning(fmt.Sprintf("Remote has %d new commit(s) — pulling...", ahead))

	pull := exec.Command("git", "pull", "--autostash", "--rebase")
	pull.Dir = dotsDir
	if out, err := pull.CombinedOutput(); err != nil {
		errMsg := strings.TrimSpace(string(out))
		conflicts := getConflictFiles(dotsDir)
		if len(conflicts) > 0 {
			return syncResult{status: "conflicts", conflicts: conflicts, ahead: ahead, errMsg: errMsg}
		}
		return syncResult{status: "error", ahead: ahead, errMsg: errMsg}
	}

	return syncResult{status: "pulled", ahead: ahead}
}

func resolveConflictsInteractive(dotsDir string, conflicts []string) bool {
	ui.PrintError("Conflicts in:")
	for _, f := range conflicts {
		fmt.Printf("   • %s\n", f)
	}

	if !ui.RunConfirm("Do you want to resolve them in your $EDITOR?", true) {
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		ui.PrintInfo("Rebase aborted — resolve manually then re-run")
		return false
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	editArgs := append([]string{editor}, conflicts...)
	editCmd := exec.Command(editArgs[0], editArgs[1:]...)
	editCmd.Dir = dotsDir
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr
	if err := editCmd.Run(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to open editor: %v", err))
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		return false
	}

	// Check remaining conflicts
	remaining := getConflictFiles(dotsDir)
	if len(remaining) > 0 {
		ui.PrintError("Still unresolved after editing:")
		for _, f := range remaining {
			fmt.Printf("   • %s\n", f)
		}
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		ui.PrintInfo("Rebase aborted — fix all conflicts then re-run")
		return false
	}

	// Mark conflicts as resolved — check for errors
	addCmd := exec.Command("git", append([]string{"add"}, conflicts...)...)
	addCmd.Dir = dotsDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to mark conflicts as resolved: %s", strings.TrimSpace(string(out))))
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		return false
	}

	if !ui.RunConfirm("Continue with rebase?", true) {
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		ui.PrintInfo("Rebase aborted")
		return false
	}

	continueCmd := exec.Command("git", "rebase", "--continue")
	continueCmd.Dir = dotsDir
	if out, err := continueCmd.CombinedOutput(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to continue rebase: %s", strings.TrimSpace(string(out))))
		return false
	}

	return true
}

// ─── backup list ─────────────────────────────────────────────────────────────

func runBackupList(cmd *cobra.Command) error {
	ui.PrintHeader("Backup History")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	limit, _ := cmd.Flags().GetInt("limit")
	if limit < 1 {
		limit = 10
	}

	logCmd := exec.Command("git", "log", fmt.Sprintf("-%d", limit), "--format=%h|%ar|%s")
	logCmd.Dir = cfg.RepoRoot
	out, err := logCmd.Output()
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		ui.PrintInfo("No backups in history")
		return nil
	}

	for _, line := range lines {
		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) == 3 {
				fmt.Printf("  %s %s\n", parts[0], parts[2])
			}
		}
	}

	return nil
}

// ─── backup diff ─────────────────────────────────────────────────────────────

func runBackupDiff(cmd *cobra.Command, args []string) error {
	ref := stringFlag(cmd, "ref")
	if len(args) > 0 {
		ref = args[0]
	}
	if ref == "" {
		ref = "HEAD~1"
	}

	ui.PrintHeader(fmt.Sprintf("Diff: %s → HEAD", ref))

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	diffCmd := exec.Command("git", "diff", "--stat", ref, "HEAD")
	diffCmd.Dir = cfg.RepoRoot
	diffCmd.Stdout = os.Stdout
	diffCmd.Stderr = os.Stderr
	if err := diffCmd.Run(); err != nil {
		return fmt.Errorf("git diff failed: %w", err)
	}

	return nil
}

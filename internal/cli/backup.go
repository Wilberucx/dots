package cli

import (
	"fmt"
	"os"
	"os/exec"
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
	message := stringFlag(cmd, "message")

	commitMsg := message
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("backup: %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	dotsDir := cfg.RepoRoot

	if err := runBackupCore(commitMsg, dotsDir, !noPush, noSync); err != nil {
		return err
	}
	return nil
}

// runBackupCore performs the core backup logic: git add, commit, optionally push.
func runBackupCore(commitMsg, dotsDir string, push, noSync bool) error {
	// git add .
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dotsDir
	addCmd.Stderr = nil
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	ui.PrintSuccess("git add .")

	// Check if there are changes to commit
	diffCmd := exec.Command("git", "diff", "--cached", "--quiet")
	diffCmd.Dir = dotsDir
	if err := diffCmd.Run(); err == nil {
		ui.PrintInfo("No changes to commit")
		return nil
	}

	// git commit
	commit := exec.Command("git", "commit", "-m", commitMsg)
	commit.Dir = dotsDir
	commit.Stderr = nil
	if out, err := commit.Output(); err != nil {
		return fmt.Errorf("git commit failed: %s", string(out))
	}
	ui.PrintSuccess(fmt.Sprintf("git commit -m \"%s\"", commitMsg))

	if push {
		if !noSync {
			syncResult := syncFromRemote(dotsDir)
			switch syncResult.status {
			case "no_upstream":
				ui.PrintInfo("No upstream branch configured — skipping sync")
			case "pulled":
				ui.PrintSuccess(fmt.Sprintf("Pulled %d commit(s) from remote", syncResult.ahead))
			case "conflicts":
				resolved := resolveConflictsInteractive(dotsDir, syncResult.conflicts)
				if !resolved {
					return fmt.Errorf("conflict resolution aborted")
				}
			case "error":
				return fmt.Errorf("sync from remote failed")
			}
		}

		pushCmd := exec.Command("git", "push")
		pushCmd.Dir = dotsDir
		pushCmd.Stderr = nil
		if out, err := pushCmd.Output(); err != nil {
			return fmt.Errorf("git push failed: %s", string(out))
		}
		ui.PrintSuccess("git push")
	}

	return nil
}

// ─── Remote sync helpers ─────────────────────────────────────────────────────

type syncResult struct {
	status    string   // "clean", "pulled", "conflicts", "no_upstream", "error"
	conflicts []string
	ahead     int
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

	fetch := exec.Command("git", "fetch", "--quiet", "origin")
	fetch.Dir = dotsDir
	fetch.Stderr = nil
	if err := fetch.Run(); err != nil {
		return syncResult{status: "error"}
	}

	ahead := getRemoteAheadCount(dotsDir, upstream)
	if ahead == 0 {
		return syncResult{status: "clean", ahead: 0}
	}

	ui.PrintWarning(fmt.Sprintf("Remote has %d new commit(s) — pulling...", ahead))

	pull := exec.Command("git", "pull", "--autostash", "--rebase")
	pull.Dir = dotsDir
	pull.Stderr = nil
	if _, err := pull.CombinedOutput(); err != nil {
		conflicts := getConflictFiles(dotsDir)
		if len(conflicts) > 0 {
			return syncResult{status: "conflicts", conflicts: conflicts, ahead: ahead}
		}
		return syncResult{status: "error", ahead: ahead}
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

	addCmd := exec.Command("git", append([]string{"add"}, conflicts...)...)
	addCmd.Dir = dotsDir
	addCmd.Run()

	if !ui.RunConfirm("Continue with rebase?", true) {
		abort := exec.Command("git", "rebase", "--abort")
		abort.Dir = dotsDir
		abort.Run()
		ui.PrintInfo("Rebase aborted")
		return false
	}

	continueCmd := exec.Command("git", "rebase", "--continue")
	continueCmd.Dir = dotsDir
	continueCmd.Stderr = nil
	if out, err := continueCmd.CombinedOutput(); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to continue rebase: %s: %s", string(out), err))
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

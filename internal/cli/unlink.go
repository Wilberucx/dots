package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/transaction"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	unlinkCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runUnlink(cmd, args)
	}
}

// unlinkStateCount tracks unlink operation statistics.
type unlinkStateCount struct {
	unlinked  int
	notLinked int
	errors    int
}

// unlinkTx wraps TransactionLog with error tracking for rollback.
type unlinkTx struct {
	tx   *transaction.TransactionLog
	fail error
}

func (u *unlinkTx) unlink(path string) {
	if u.fail != nil {
		return
	}
	if err := u.tx.Unlink(path); err != nil {
		u.fail = err
	}
}

func (u *unlinkTx) commit() error {
	if u.fail != nil {
		u.tx.Rollback()
		return u.fail
	}
	u.tx.Commit()
	return nil
}

func runUnlink(cmd *cobra.Command, args []string) error {
	ui.PrintHeader("Unlinking Dotfiles")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	modules := stringSliceFlag(cmd, "module")
	dryRun := boolFlag(cmd, "dry-run")
	interactive := boolFlag(cmd, "interactive")

	// ── Module selection ──────────────────────────────────────────────
	selectedModules := mergeModuleArgs(modules, args)
	if len(selectedModules) > 0 {
		ui.PrintInfo(fmt.Sprintf("Unlinking specified modules: %s", strings.Join(selectedModules, ", ")))
	} else if interactive {
		selectedModules = selectModulesInteractive(cfg, false)
		if len(selectedModules) == 0 {
			ui.PrintInfo("No modules selected.")
			return nil
		}
		ui.PrintInfo(fmt.Sprintf("Selected %d module(s)", len(selectedModules)))
		fmt.Println()
	} else if !dryRun {
		if !ui.RunConfirm("This will unlink ALL modules. Continue?", true) {
			ui.PrintInfo("Cancelled.")
			return nil
		}
		selectedModules = nil // Unlink all modules
	} else {
		selectedModules = nil // Unlink all modules
	}

	// ── Resolve ───────────────────────────────────────────────────────
	allModules, err := resolver.ResolveModules(cfg, selectedModules, nil, "")
	if err != nil {
		return fmt.Errorf("resolving modules: %w", err)
	}
	if len(allModules) == 0 {
		ui.PrintWarning("No modules found.")
		return nil
	}

	// ── Process each module ───────────────────────────────────────────
	stats := &unlinkStateCount{}
	tx := &unlinkTx{tx: &transaction.TransactionLog{}}
	moduleNames := sortedModuleNames(allModules)

	for _, modName := range moduleNames {
		statuses := allModules[modName]
		modStats := &unlinkStateCount{}
		var rows []linkRow

		for _, st := range statuses {
			if tx.fail != nil {
				break
			}

			srcName := filepath.Base(st.Source)
			shortDest := shortDisplayPath(st.Destination, cfg.HomeDir)

			switch st.State {
			case resolver.StateLinked:
				if dryRun {
					rows = append(rows, linkRow{
						icon:   ui.ErrorStyle.Render("✗"),
						src:    srcName,
						dest:   shortDest,
						detail: ui.ErrorStyle.Render("(to be removed)"),
					})
					modStats.unlinked++
				} else {
					tx.unlink(st.Destination)
					if tx.fail != nil {
						rows = append(rows, linkRow{
							icon:   ui.ErrorStyle.Render("✗"),
							src:    srcName,
							dest:   shortDest,
							detail: ui.ErrorStyle.Render(fmt.Sprintf("(error: %v)", tx.fail)),
						})
						modStats.notLinked++
					} else {
						rows = append(rows, linkRow{
							icon:   ui.SuccessStyle.Render("✔"),
							src:    srcName,
							dest:   shortDest,
							detail: ui.SuccessStyle.Render("(removed)"),
						})
						modStats.unlinked++
					}
				}

			case resolver.StateConflict:
				rows = append(rows, linkRow{
					icon:   ui.WarningStyle.Render("⚠"),
					src:    srcName,
					dest:   shortDest,
					detail: ui.WarningStyle.Render("(conflict, skipping)"),
				})
				modStats.notLinked++

			case resolver.StatePending, resolver.StateMissing:
				rows = append(rows, linkRow{
					icon:   ui.InfoStyle.Render("ℹ"),
					src:    srcName,
					dest:   shortDest,
					detail: ui.DimStyle.Render("(not linked)"),
				})
				modStats.notLinked++

			case resolver.StateUnsafe:
				rows = append(rows, linkRow{
					icon:   ui.WarningStyle.Render("⚠"),
					src:    srcName,
					dest:   shortDest,
					detail: ui.WarningStyle.Render("(unsafe, skipping)"),
				})
				modStats.notLinked++
			}
		}

		// Print module tree
		fmt.Printf("  %s %s\n", ui.DimStyle.Render("📦"), ui.BoldStyle.Render(modName))
		for _, row := range rows {
			msg := fmt.Sprintf("    %s %s → %s", row.icon, row.src, row.dest)
			if row.detail != "" {
				msg += " " + row.detail
			}
			fmt.Println(msg)
		}

		// Module status line
		var statusParts []string
		if modStats.unlinked > 0 {
			label := "to remove"
			if !dryRun {
				label = "removed"
			}
			statusParts = append(statusParts, ui.ErrorStyle.Render(fmt.Sprintf("%d %s", modStats.unlinked, label)))
		}
		if modStats.notLinked > 0 {
			statusParts = append(statusParts, ui.DimStyle.Render(fmt.Sprintf("%d not linked", modStats.notLinked)))
		}
		if len(statusParts) > 0 {
			fmt.Printf("    %s\n", ui.DimStyle.Render(fmt.Sprintf("Status: %s", strings.Join(statusParts, " • "))))
		}
		fmt.Println()

		// Update global stats
		stats.unlinked += modStats.unlinked
		stats.notLinked += modStats.notLinked
	}

	// ── Commit or rollback ────────────────────────────────────────────
	if !dryRun {
		if err := tx.commit(); err != nil {
			ui.PrintError(fmt.Sprintf("Error during unlinking: %v", err))
			ui.PrintInfo("Rolling back changes...")
			ui.PrintSuccess("Rollback complete.")
			return fmt.Errorf("unlinking failed: %w", err)
		}
	}

	// ── Summary ───────────────────────────────────────────────────────
	ui.PrintDivider(0)
	summaryParts := []string{}
	if stats.unlinked > 0 {
		label := "to remove"
		if !dryRun {
			label = "removed"
		}
		summaryParts = append(summaryParts, ui.SuccessStyle.Render(fmt.Sprintf("%d ✔ %s", stats.unlinked, label)))
	}
	if stats.notLinked > 0 {
		summaryParts = append(summaryParts, ui.DimStyle.Render(fmt.Sprintf("%d not linked", stats.notLinked)))
	}
	fmt.Printf("%s %s\n", ui.BoldStyle.Render("Summary:"), strings.Join(summaryParts, " • "))

	if dryRun {
		fmt.Printf("\n%s\n", ui.DimStyle.Render("This was a dry run. No changes were made."))
	}

	return nil
}

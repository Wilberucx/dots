package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/transaction"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/yaml"
	"github.com/spf13/cobra"
)

func init() {
	linkCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runLink(cmd)
	}
}

// stateCount tracks link operation statistics per module.
type stateCount struct {
	linked    int
	conflicts int
	pending   int
	errors    int
}

// linkRow is a display row for one mapping.
type linkRow struct {
	icon    string
	src     string
	dest    string
	detail  string
	variant string
}

// doTx runs a transactional operation and records any error.
// Instead of panicking on error, we capture it and let the caller decide.
type linkTx struct {
	tx    *transaction.TransactionLog
	fail  error
}

func (l *linkTx) symlink(path, target string) {
	if l.fail != nil {
		return
	}
	if err := l.tx.Symlink(path, target); err != nil {
		l.fail = err
	}
}

func (l *linkTx) unlink(path string) {
	if l.fail != nil {
		return
	}
	if err := l.tx.Unlink(path); err != nil {
		l.fail = err
	}
}

func (l *linkTx) mkdir(path string) {
	if l.fail != nil {
		return
	}
	if err := l.tx.Mkdir(path); err != nil {
		l.fail = err
	}
}

func (l *linkTx) backup(path, backupPath string) {
	if l.fail != nil {
		return
	}
	l.tx.Backup(path, backupPath)
}

func (l *linkTx) commit() error {
	if l.fail != nil {
		l.tx.Rollback()
		return l.fail
	}
	l.tx.Commit()
	return nil
}

func runLink(cmd *cobra.Command) error {
	ui.PrintHeader("Linking Dotfiles")

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	modules := stringSliceFlag(cmd, "module")
	types := stringSliceFlag(cmd, "type")
	dryRun := boolFlag(cmd, "dry-run")
	force := boolFlag(cmd, "force")
	interactive := boolFlag(cmd, "interactive")
	variant := stringFlag(cmd, "variant")

	// ── Module selection ──────────────────────────────────────────────
	var selectedModules []string
	if len(modules) > 0 {
		selectedModules = modules
		ui.PrintInfo(fmt.Sprintf("Linking specified modules: %s", strings.Join(selectedModules, ", ")))
	} else if interactive {
		selectedModules = selectModulesInteractive(cfg, true)
		if len(selectedModules) == 0 {
			ui.PrintInfo("No modules selected.")
			return nil
		}
		ui.PrintInfo(fmt.Sprintf("Selected %d module(s)", len(selectedModules)))
		fmt.Println()
	} else {
		selectedModules = nil // Link all modules
	}

	// ── Variant validation ────────────────────────────────────────────
	if variant != "" && len(selectedModules) == 0 {
		ui.PrintError("When using --variant, you must specify the module name.")
		ui.PrintInfo("Example: dots link -m Nvim --variant notevim")
		return fmt.Errorf("--variant requires --module")
	}

	// Variant auto-swap: if variant is explicitly requested and a different variant is active, auto-force
	isVariantSwap := false
	if variant != "" && !force && len(selectedModules) > 0 {
		for _, modName := range selectedModules {
			active, _ := resolver.GetActiveVariant(cfg, modName)
			if active != "" && active != variant {
				force = true
				isVariantSwap = true
				ui.PrintInfo(fmt.Sprintf("Auto-swap: %s variant '%s' → '%s'", modName, active, variant))
				break
			}
		}
	}

	// Validate variant existence
	if variant != "" && len(selectedModules) > 0 {
		for _, modName := range selectedModules {
			vInfo, err := resolver.GetModuleVariantInfo(cfg, modName)
			if err != nil {
				return fmt.Errorf("checking variant info for %s: %w", modName, err)
			}
			if vInfo == nil || !vInfo.HasVariants {
				modDir := filepath.Join(cfg.RepoRoot, modName)
				entries, _ := os.ReadDir(modDir)
				var sources []string
				for _, e := range entries {
					if !e.IsDir() && e.Name() != "path.yaml" {
						sources = append(sources, e.Name())
					} else if e.IsDir() {
						sources = append(sources, e.Name()+"/")
					}
				}
				return fmt.Errorf("module '%s' has no variants (available: %s)", modName, strings.Join(sources, ", "))
			}
			found := false
			for _, v := range vInfo.Variants {
				if v == variant {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("variant '%s' not found in module '%s' (available: %s)", variant, modName, strings.Join(vInfo.Variants, ", "))
			}
		}
	}

	// Cascade warning
	if variant == "" && len(selectedModules) > 0 {
		for _, modName := range selectedModules {
			vInfo, _ := resolver.GetModuleVariantInfo(cfg, modName)
			if vInfo != nil && vInfo.HasVariants {
				ui.PrintWarning(fmt.Sprintf(
					"Module '%s' has multiple variants. Using default: '%s' (last in YAML). Use --variant to select a specific one.",
					modName, vInfo.DefaultVariant,
				))
			}
		}
	}

	// ── Resolve ───────────────────────────────────────────────────────
	allModules, err := resolver.ResolveModules(cfg, selectedModules, types, variant)
	if err != nil {
		return fmt.Errorf("resolving modules: %w", err)
	}
	if len(allModules) == 0 {
		ui.PrintWarning("No modules found.")
		return nil
	}

	// ── Process each module ───────────────────────────────────────────
	stats := &stateCount{}
	tx := &linkTx{tx: &transaction.TransactionLog{}}
	moduleNames := sortedModuleNames(allModules)

	// Determine effective variant for display
	getEffectiveVariant := func(moduleName string) string {
		if variant != "" {
			return variant
		}
		yamlPath := filepath.Join(cfg.RepoRoot, moduleName, "path.yaml")
		mappings, _ := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
		vInfo := yaml.DetectVariants(mappings)
		if vInfo.HasVariants {
			return vInfo.DefaultVariant
		}
		return ""
	}

	for _, modName := range moduleNames {
		statuses := allModules[modName]
		modStats := &stateCount{}
		var rows []linkRow

		// Effective variant for this module
		effVariant := getEffectiveVariant(modName)
		variantTag := ""
		if effVariant != "" {
			variantTag = fmt.Sprintf(" [%s]", effVariant)
		}

		for _, st := range statuses {
			if tx.fail != nil {
				break
			}

			srcName := filepath.Base(st.Source)
			shortDest := shortDisplayPath(st.Destination, cfg.HomeDir)

			switch st.State {
			case resolver.StateLinked:
				rows = append(rows, linkRow{
					icon:   ui.SuccessStyle.Render(ui.IconLinked),
					src:    srcName,
					dest:   shortDest,
					detail: "",
				})
				modStats.linked++

			case resolver.StateUnsafe:
				rows = append(rows, linkRow{
					icon:   ui.ErrorStyle.Render(ui.IconError),
					src:    srcName,
					dest:   shortDest,
					detail: ui.ErrorStyle.Render(fmt.Sprintf("(%s)", st.Detail)),
				})
				modStats.conflicts++

			case resolver.StateConflict:
				if force {
					if dryRun {
						if isVariantSwap {
							active, _ := resolver.GetActiveVariant(cfg, modName)
							rows = append(rows, linkRow{
								icon:   ui.InfoStyle.Render(ui.IconSwap),
								src:    srcName,
								dest:   shortDest,
								detail: ui.InfoStyle.Render(fmt.Sprintf("(swapped: %s → %s)", active, variant)),
							})
						} else {
							rows = append(rows, linkRow{
								icon:   ui.WarningStyle.Render(ui.IconConflict),
								src:    srcName,
								dest:   shortDest,
								detail: ui.WarningStyle.Render("(to be overwritten)"),
							})
						}
						modStats.pending++
					} else {
						if isVariantSwap {
							active, _ := resolver.GetActiveVariant(cfg, modName)
							rows = append(rows, linkRow{
								icon:   ui.InfoStyle.Render(ui.IconSwap),
								src:    srcName,
								dest:   shortDest,
								detail: ui.InfoStyle.Render(fmt.Sprintf("(swapped: %s → %s)", active, variant)),
							})
						} else {
							rows = append(rows, linkRow{
								icon:   ui.SuccessStyle.Render(ui.IconLinked),
								src:    srcName,
								dest:   shortDest,
								detail: ui.SuccessStyle.Render("(overwritten)"),
							})
						}
						modStats.linked++
						tx.unlink(st.Destination)
						tx.symlink(st.Destination, st.Source)
					}
				} else {
					rows = append(rows, linkRow{
						icon:   ui.ErrorStyle.Render(ui.IconConflict),
						src:    srcName,
						dest:   shortDest,
						detail: ui.ErrorStyle.Render(fmt.Sprintf("(conflict: %s)", st.Detail)),
					})
					modStats.conflicts++
				}

			case resolver.StatePending:
				if st.Detail == "backup needed" {
					backupPath := st.BackupPath
					if backupPath == "" {
						backupPath = st.Destination + ".orig"
					}
					if _, err := os.Stat(backupPath); err == nil {
						rows = append(rows, linkRow{
							icon:   ui.WarningStyle.Render(ui.IconConflict),
							src:    srcName,
							dest:   shortDest,
							detail: ui.WarningStyle.Render(fmt.Sprintf("(.orig exists at %s, run 'dots status --backups' to review)", shortDisplayPath(backupPath, cfg.HomeDir))),
						})
						modStats.pending++
					} else {
						if dryRun {
							rows = append(rows, linkRow{
								icon:   ui.WarningStyle.Render(ui.IconConflict),
								src:    srcName,
								dest:   shortDest,
								detail: ui.WarningStyle.Render("(.orig needed)"),
							})
							modStats.pending++
						} else {
							rows = append(rows, linkRow{
								icon:   ui.SuccessStyle.Render(ui.IconLinked),
								src:    srcName,
								dest:   shortDest,
								detail: ui.SuccessStyle.Render(fmt.Sprintf("(.orig saved and linked)%s", variantTag)),
							})
							modStats.linked++
							tx.backup(st.Destination, backupPath)
							tx.symlink(st.Destination, st.Source)
						}
					}
				} else {
					if dryRun {
						rows = append(rows, linkRow{
							icon:   ui.InfoStyle.Render(ui.IconPending),
							src:    srcName,
							dest:   shortDest,
							detail: ui.DimStyle.Render("(to be created)"),
						})
						modStats.pending++
					} else {
						rows = append(rows, linkRow{
							icon:   ui.SuccessStyle.Render(ui.IconLinked),
							src:    srcName,
							dest:   shortDest,
							detail: ui.SuccessStyle.Render(fmt.Sprintf("(created)%s", variantTag)),
						})
						modStats.linked++
						parentDir := filepath.Dir(st.Destination)
						if _, err := os.Stat(parentDir); os.IsNotExist(err) {
							tx.mkdir(parentDir)
						}
						tx.symlink(st.Destination, st.Source)
					}
				}
			}
		}

		// Print module tree
		fmt.Printf("  %s %s\n", ui.DimStyle.Render(ui.IconModule), ui.BoldStyle.Render(modName))
		for _, row := range rows {
			msg := fmt.Sprintf("    %s %s → %s", row.icon, row.src, row.dest)
			if row.detail != "" {
				msg += " " + row.detail
			}
			fmt.Println(msg)
		}

		// Module status line
		var statusParts []string
		if modStats.linked > 0 {
			statusParts = append(statusParts, ui.SuccessStyle.Render(fmt.Sprintf("%d linked", modStats.linked)))
		}
		if dryRun && modStats.pending > 0 {
			statusParts = append(statusParts, ui.WarningStyle.Render(fmt.Sprintf("%d to link", modStats.pending)))
		}
		if modStats.conflicts > 0 {
			statusParts = append(statusParts, ui.ErrorStyle.Render(fmt.Sprintf("%d conflicts", modStats.conflicts)))
		}
		if len(statusParts) > 0 {
			fmt.Printf("    %s\n", ui.DimStyle.Render(fmt.Sprintf("Status: %s", strings.Join(statusParts, " • "))))
		}
		fmt.Println()

		// Update global stats
		stats.linked += modStats.linked
		stats.conflicts += modStats.conflicts
		stats.pending += modStats.pending
	}

	// ── Commit or rollback ────────────────────────────────────────────
	if !dryRun {
		if err := tx.commit(); err != nil {
			ui.PrintError(fmt.Sprintf("Error during linking: %v", err))
			ui.PrintInfo("Rolling back changes...")
			// tx.commit() already called rollback on fail
			ui.PrintSuccess("Rollback complete.")
			return fmt.Errorf("linking failed: %w", err)
		}
	}

	// ── Summary ───────────────────────────────────────────────────────
	ui.PrintDivider(0)
	summaryParts := []string{}
	if stats.linked > 0 {
		summaryParts = append(summaryParts, ui.SuccessStyle.Render(fmt.Sprintf("%d ✔ linked", stats.linked)))
	}
	if stats.conflicts > 0 {
		summaryParts = append(summaryParts, ui.ErrorStyle.Render(fmt.Sprintf("%d ⚠ conflicts", stats.conflicts)))
	}
	if dryRun && stats.pending > 0 {
		summaryParts = append(summaryParts, ui.WarningStyle.Render(fmt.Sprintf("%d ℹ to link", stats.pending)))
	}
	fmt.Printf("%s %s\n", ui.BoldStyle.Render("Summary:"), strings.Join(summaryParts, " • "))

	if dryRun {
		fmt.Printf("\n%s\n", ui.DimStyle.Render("This was a dry run. No changes were made."))
	}

	return nil
}

// selectModulesInteractive runs a Bubbletea TUI for interactive module selection.
func selectModulesInteractive(cfg *config.DotsConfig, preselectAll bool) []string {
	modDirs, err := cfg.GetModuleDirs(nil, nil)
	if err != nil || len(modDirs) == 0 {
		return nil
	}

	names := make([]string, len(modDirs))
	for i, d := range modDirs {
		names[i] = d.Name
	}

	selected, err := ui.RunModuleSelector(names, preselectAll)
	if err != nil {
		return nil
	}
	return selected
}

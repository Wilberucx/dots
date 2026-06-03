package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/plan"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what dots would do without modifying the system",
		Long: `Show the plan of actions that 'dots link' would perform.

Displays all symlink operations (create, replace, backup, skip) grouped
by module. Use --module to filter and --format json for machine output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd)
		},
	}

	planCmd.Flags().StringSliceP("module", "m", nil, "Show plan only for specific modules (repeatable)")
	planCmd.Flags().StringSliceP("type", "t", nil, "Show plan only for modules of this type (repeatable)")
	planCmd.Flags().String("variant", "", "Show plan for a specific variant")
	planCmd.Flags().Bool("force", false, "Show plan with --force applied (conflicts become replacements)")
	planCmd.Flags().StringP("format", "f", "default", "Output format: default, json")

	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	modules := stringSliceFlag(cmd, "module")
	types := stringSliceFlag(cmd, "type")
	variant := stringFlag(cmd, "variant")
	format := stringFlag(cmd, "format")

	// ── Variant validation ────────────────────────────────────────────
	force := boolFlag(cmd, "force")

	if variant != "" && len(modules) == 0 {
		ui.PrintError("When using --variant, you must specify the module name.")
		ui.PrintInfo("Example: dots plan -m Nvim --variant notevim")
		return fmt.Errorf("--variant requires --module")
	}

	allModules, err := resolver.ResolveModules(cfg, modules, types, variant)
	if err != nil {
		return fmt.Errorf("resolving modules: %w", err)
	}

	if len(allModules) == 0 {
		ui.PrintWarning("No modules found.")
		return nil
	}

	// Variant auto-swap: detect if --variant implies swapping from an active variant
	variantSwapModules := buildVariantSwapMap(cfg, modules, variant, force)

	opts := plan.BuildOptions{
		Force:        force,
		VariantSwaps: variantSwapModules,
	}
	p := plan.BuildLinkPlan(allModules, opts)

	switch format {
	case "json":
		// JSON output must be pure JSON from first byte — no UI output before it
		return renderPlanJSON(p, cfg)
	default:
		// Human-readable format: print auto-swap info before the plan
		for _, modName := range sortedSwapModules(variantSwapModules) {
			active, _ := resolver.GetActiveVariant(cfg, modName)
			ui.PrintInfo(fmt.Sprintf("Auto-swap: %s variant '%s' → '%s'", modName, active, variant))
		}
		renderPlanDefault(p, cfg)
	}

	return nil
}

// renderPlanDefault renders the plan in human-readable tree format.
func renderPlanDefault(p *plan.Plan, cfg *config.DotsConfig) {
	ui.PrintHeader("Plan")

	byModule := p.ActionsByModule()

	for _, modName := range p.ModuleNames() {
		actions := byModule[modName]
		fmt.Printf("  %s %s\n", ui.DimStyle.Render(ui.IconModule), ui.BoldStyle.Render(modName))

		for _, a := range actions {
			shortDest := shortDisplayPath(a.Destination, cfg.HomeDir)
			srcName := filepath.Base(a.Source)

			var icon string
			var detailStyle func(string) string
			detailStr := a.Detail

			switch a.Kind {
			case plan.ActionCreateSymlink:
				icon = ui.InfoStyle.Render(ui.IconPending)
				detailStyle = func(s string) string { return ui.DimStyle.Render(s) }
			case plan.ActionBackupFile:
				icon = ui.WarningStyle.Render(ui.IconConflict)
				detailStyle = func(s string) string { return ui.WarningStyle.Render(s) }
			case plan.ActionReplaceSymlink:
				icon = ui.InfoStyle.Render(ui.IconSwap)
				detailStyle = func(s string) string { return ui.InfoStyle.Render(s) }
			case plan.ActionSkipLinked:
				icon = ui.SuccessStyle.Render(ui.IconLinked)
				detailStyle = func(s string) string { return ui.DimStyle.Render("(" + s + ")") }
			case plan.ActionSkipPending:
				icon = ui.DimStyle.Render("○")
				detailStyle = func(s string) string { return ui.DimStyle.Render("(" + s + ")") }
			case plan.ActionErrorConflict,
				plan.ActionErrorUnsafe:
				icon = ui.ErrorStyle.Render(ui.IconError)
				detailStyle = func(s string) string { return ui.ErrorStyle.Render("(" + s + ")") }
			default:
				icon = ui.DimStyle.Render("·")
				detailStyle = func(s string) string { return ui.DimStyle.Render("(" + s + ")") }
			}

			msg := fmt.Sprintf("    %s %s → %s", icon, srcName, shortDest)
			if detailStr != "" {
				msg += " " + detailStyle(detailStr)
			}
			fmt.Println(msg)
		}

		// Summary counts for this module
		counts := make(map[plan.ActionKind]int)
		for _, a := range actions {
			counts[a.Kind]++
		}
		var parts []string
		if n := counts[plan.ActionCreateSymlink]; n > 0 {
			parts = append(parts, ui.InfoStyle.Render(fmt.Sprintf("%d to create", n)))
		}
		if n := counts[plan.ActionBackupFile]; n > 0 {
			parts = append(parts, ui.WarningStyle.Render(fmt.Sprintf("%d to backup", n)))
		}
		if n := counts[plan.ActionReplaceSymlink]; n > 0 {
			parts = append(parts, ui.InfoStyle.Render(fmt.Sprintf("%d to replace", n)))
		}
		if n := counts[plan.ActionErrorConflict]; n > 0 {
			parts = append(parts, ui.ErrorStyle.Render(fmt.Sprintf("%d conflicts", n)))
		}
		if len(parts) > 0 {
			fmt.Printf("    %s\n", ui.DimStyle.Render(strings.Join(parts, " • ")))
		}
		fmt.Println()
	}

	// Global summary
	ui.PrintDivider(0)
	var summaryParts []string
	counts := p.CountByKind()
	if n := counts[plan.ActionCreateSymlink]; n > 0 {
		summaryParts = append(summaryParts, ui.InfoStyle.Render(fmt.Sprintf("%d to create", n)))
	}
	if n := counts[plan.ActionBackupFile]; n > 0 {
		summaryParts = append(summaryParts, ui.WarningStyle.Render(fmt.Sprintf("%d to backup", n)))
	}
	if n := counts[plan.ActionReplaceSymlink]; n > 0 {
		summaryParts = append(summaryParts, ui.InfoStyle.Render(fmt.Sprintf("%d to replace", n)))
	}
	if n := counts[plan.ActionSkipLinked]; n > 0 {
		summaryParts = append(summaryParts, ui.DimStyle.Render(fmt.Sprintf("%d already linked", n)))
	}
	if n := counts[plan.ActionErrorConflict]; n > 0 {
		summaryParts = append(summaryParts, ui.ErrorStyle.Render(fmt.Sprintf("%d conflicts", n)))
	}
	if len(summaryParts) > 0 {
		fmt.Printf("%s %s\n", ui.BoldStyle.Render("Summary:"), strings.Join(summaryParts, " • "))
	}

	fmt.Printf("\n%s\n", ui.DimStyle.Render("This is a plan. No changes were made. Run 'dots link' to apply."))
}

// renderPlanJSON renders the plan as JSON.
func renderPlanJSON(p *plan.Plan, cfg *config.DotsConfig) error {
	type planEntry struct {
		Module      string `json:"module"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
		Action      string `json:"action"`
		State       string `json:"state"`
		Detail      string `json:"detail,omitempty"`
	}

	entries := make([]planEntry, len(p.Actions))
	for i, a := range p.Actions {
		entries[i] = planEntry{
			Module:      a.Module,
			Source:      shortDisplayPath(a.Source, cfg.HomeDir),
			Destination: shortDisplayPath(a.Destination, cfg.HomeDir),
			Action:      string(a.Kind),
			State:       string(a.State),
			Detail:      a.Detail,
		}
	}

	output := map[string]interface{}{
		"plan":  entries,
		"count": len(entries),
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/plan"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/ui"
)

func init() {
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Show what dots would do without modifying the system",
		Long: `Show the plan of actions that 'dots link' would perform.

Displays all symlink operations (create, replace, backup, skip) grouped
by module. Use --module to filter and --format table, json, porcelain
for machine-parseable output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd)
		},
	}

	planCmd.Flags().StringSliceP("module", "m", nil, "Show plan only for specific modules (repeatable)")
	planCmd.Flags().StringSliceP("type", "t", nil, "Show plan only for modules of this type (repeatable)")
	planCmd.Flags().StringP("variant", "V", "", "Show plan for a specific variant")
	planCmd.Flags().Bool("force", false, "Show plan with --force applied (conflicts become replacements)")
	planCmd.Flags().StringP("format", "f", "default", "Output format: default, table, json, porcelain")
	planCmd.Flags().Bool("porcelain", false, "Machine-parseable tab-separated output (module\\taction\\tsource\\tdestination)")

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

	// --porcelain flag overrides --format
	porcelain, _ := cmd.Flags().GetBool("porcelain")
	if porcelain {
		format = "porcelain"
	}

	// Check config default for plan format
	format = resolvePlanFormat(format, cfg)

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
	variantSwapModules := buildVariantSwapMap(cfg, modules, nil, variant, force)

	opts := plan.BuildOptions{
		Force:        force,
		VariantSwaps: variantSwapModules,
	}
	p := plan.BuildLinkPlan(allModules, opts)

	switch format {
	case "table":
		// Show auto-swap info before the table
		for _, modName := range sortedSwapModules(variantSwapModules) {
			active, _ := resolver.GetActiveVariant(cfg, modName)
			ui.PrintInfo(fmt.Sprintf("Auto-swap: %s variant '%s' → '%s'", modName, active, variant))
		}
		renderPlanTable(p, cfg)
	case "json":
		// JSON output must be pure JSON from first byte — no UI output before it
		return renderPlanJSON(p, cfg)
	case "porcelain":
		renderPlanPorcelain(p, cfg)
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

// resolvePlanFormat resolves the output format for plan.
// If format is "default" (the flag default), it falls back to the config's
// output.plan, then to "default" if no config default is set.
func resolvePlanFormat(format string, cfg *config.DotsConfig) string {
	if format != "default" {
		return format
	}
	if cfg != nil && cfg.InitCfg != nil && cfg.InitCfg.Output != nil && cfg.InitCfg.Output.Plan != "" {
		return cfg.InitCfg.Output.Plan
	}
	return "default"
}

// renderPlanTable renders the plan in table format.
func renderPlanTable(p *plan.Plan, cfg *config.DotsConfig) {
	columns := []table.Column{
		{Title: "Module", Width: 16},
		{Title: "Action", Width: 14},
		{Title: "Source", Width: 20},
		{Title: "Destination", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
	)

	var rows []table.Row
	total := 0

	for _, modName := range p.ModuleNames() {
		actions := p.ActionsByModule()[modName]
		firstRow := true
		for _, a := range actions {
			modCell := ""
			if firstRow {
				modCell = modName
				firstRow = false
			}

			shortDest := shortDisplayPath(a.Destination, cfg.HomeDir)
			srcName := filepath.Base(a.Source)
			actionName := string(a.Kind)

			rows = append(rows, table.Row{
				modCell,
				actionName,
				srcName,
				shortDest,
			})
			total++
		}
	}

	if total == 0 {
		ui.PrintWarning("No actions in plan.")
		return
	}

	t.SetRows(rows)
	tableStyle := lipgloss.NewStyle().Padding(0, 1)
	fmt.Println(tableStyle.Render(ui.DimStyle.Render(t.View())))
	fmt.Printf("\n%s\n", ui.DimStyle.Render(fmt.Sprintf("Total: %d actions", total)))
}

// renderPlanPorcelain renders the plan as tab-separated lines.
func renderPlanPorcelain(p *plan.Plan, cfg *config.DotsConfig) {
	for _, modName := range p.ModuleNames() {
		actions := p.ActionsByModule()[modName]
		for _, a := range actions {
			shortDest := shortDisplayPath(a.Destination, cfg.HomeDir)
			srcName := filepath.Base(a.Source)
			// Tab-separated: module, action, source, destination
			fmt.Printf("%s\t%s\t%s\t%s\n",
				modName,
				string(a.Kind),
				srcName,
				shortDest,
			)
		}
	}
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

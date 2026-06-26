package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	statusCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd, args)
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	modules := mergeModuleArgs(stringSliceFlag(cmd, "module"), args)
	types := stringSliceFlag(cmd, "type")
	stateFilterFlags := stringSliceFlag(cmd, "state")
	format := stringFlag(cmd, "format")
	showBackups, _ := cmd.Flags().GetBool("backups")

	allModules, err := resolver.ResolveModules(cfg, modules, types, "")
	if err != nil {
		return fmt.Errorf("resolving modules: %w", err)
	}

	if len(allModules) == 0 {
		ui.PrintWarning("No modules found.")
		return nil
	}

	var stateSet map[string]bool
	if len(stateFilterFlags) > 0 {
		stateSet = make(map[string]bool)
		for _, s := range stateFilterFlags {
			switch s {
			case "unlinked":
				stateSet["pending"] = true
			case "broken":
				stateSet["conflict"] = true
			case "linked":
				stateSet["linked"] = true
			case "missing":
				stateSet["missing"] = true
			case "unsafe":
				stateSet["unsafe"] = true
			default:
				stateSet[s] = true
			}
		}
	}

	switch format {
	case "table":
		renderTable(allModules, stateSet, cfg, showBackups)
	case "json":
		return renderJSON(allModules, stateSet, cfg, showBackups)
	default:
		ui.PrintHeader("Dots Status")
		renderDefault(allModules, stateSet, cfg, showBackups)
	}

	return nil
}

// ─── Default (tree) output ──────────────────────────────────────────────────

type moduleCategory struct {
	name string
	info string
}

func renderDefault(
	allModules map[string][]resolver.LinkStatus,
	stateFilter map[string]bool,
	cfg *config.DotsConfig,
	showBackups bool,
) {
	var linked, broken, missingSrc, unlinked, notLinked []moduleCategory

	sortedNames := sortedModuleNames(allModules)

	for _, moduleName := range sortedNames {
		statuses := allModules[moduleName]

		if showBackups {
			hasBackup := false
			for _, st := range statuses {
				if st.BackupPath != "" {
					hasBackup = true
					break
				}
			}
			if !hasBackup {
				continue
			}
		}

		moduleLinked := 0
		moduleBroken := 0
		moduleMissing := 0
		modulePending := 0

		for _, st := range statuses {
			switch st.State {
			case resolver.StateLinked:
				moduleLinked++
			case resolver.StateConflict, resolver.StateUnsafe:
				moduleBroken++
			case resolver.StateMissing:
				moduleMissing++
			case resolver.StatePending:
				modulePending++
			}
		}

		var backupInfo string
		if showBackups {
			var backups []string
			for _, st := range statuses {
				if st.BackupPath != "" {
					backups = append(backups, filepath.Base(st.BackupPath))
				}
			}
			if len(backups) > 0 {
				backupInfo = strings.Join(backups, ", ")
			}
		}

		if moduleBroken > 0 {
			if stateFilter == nil || stateFilter["conflict"] || stateFilter["unsafe"] {
				var reasons []string
				conflicts := 0
				unsafeCount := 0
				for _, st := range statuses {
					if st.State == resolver.StateConflict {
						conflicts++
					}
					if st.State == resolver.StateUnsafe {
						unsafeCount++
					}
				}
				if conflicts > 0 {
					reasons = append(reasons, fmt.Sprintf("%d conflict(s)", conflicts))
				}
				if unsafeCount > 0 {
					reasons = append(reasons, fmt.Sprintf("%d unsafe path(s)", unsafeCount))
				}
				info := strings.Join(reasons, ", ")
				if backupInfo != "" {
					info = info + " ⚠ (" + backupInfo + ")"
				}
				broken = append(broken, moduleCategory{moduleName, info})
			}
		} else if moduleMissing > 0 {
			if stateFilter == nil || stateFilter["missing"] {
				info := fmt.Sprintf("%d missing source(s)", moduleMissing)
				if backupInfo != "" {
					info = info + " ⚠ (" + backupInfo + ")"
				}
				missingSrc = append(missingSrc, moduleCategory{moduleName, info})
			}
		} else if modulePending > 0 {
			if stateFilter == nil || stateFilter["pending"] {
				info := fmt.Sprintf("%d unlinked", modulePending)
				if backupInfo != "" {
					info = info + " ⚠ (" + backupInfo + ")"
				}
				unlinked = append(unlinked, moduleCategory{moduleName, info})
			}
		} else if moduleLinked > 0 {
			if stateFilter == nil || stateFilter["linked"] {
				info := fmt.Sprintf("%d linked", moduleLinked)
				if backupInfo != "" {
					info = info + " ⚠ (" + backupInfo + ")"
				}
				linked = append(linked, moduleCategory{moduleName, info})
			}
		} else {
			notLinked = append(notLinked, moduleCategory{moduleName, "no files to link"})
		}
	}

	displayCategoryWithModules("✔ Linked", len(linked), linked, allModules, cfg, ui.SuccessStyle)
	displayCategoryWithModules("ℹ Unlinked", len(unlinked), unlinked, allModules, cfg, ui.DimStyle)
	displayCategoryWithModules("✖ Broken", len(broken), broken, allModules, cfg, ui.ErrorStyle)
	displayCategoryWithModules("⚠ Missing Source", len(missingSrc), missingSrc, allModules, cfg, ui.WarningStyle)
	displayCategoryWithModules("• Empty", len(notLinked), notLinked, allModules, cfg, ui.DimStyle)

	ui.PrintDivider(0)

	var summaryParts []string
	if len(linked) > 0 {
		summaryParts = append(summaryParts, ui.SuccessStyle.Render(fmt.Sprintf("%d linked", len(linked))))
	}
	if len(unlinked) > 0 {
		summaryParts = append(summaryParts, ui.DimStyle.Render(fmt.Sprintf("%d unlinked", len(unlinked))))
	}
	if len(broken) > 0 {
		summaryParts = append(summaryParts, ui.ErrorStyle.Render(fmt.Sprintf("%d broken", len(broken))))
	}
	if len(missingSrc) > 0 {
		summaryParts = append(summaryParts, ui.WarningStyle.Render(fmt.Sprintf("%d missing", len(missingSrc))))
	}
	if len(notLinked) > 0 {
		summaryParts = append(summaryParts, ui.DimStyle.Render(fmt.Sprintf("%d empty", len(notLinked))))
	}

	fmt.Println(ui.BoldStyle.Render("Summary:") + " " + strings.Join(summaryParts, " • "))

	fmt.Println()
	ui.PrintInfo("Run 'dots doctor' for deep diagnostics (syntax check, broken links, YAML hints).")
}

func displayCategoryWithModules(
	title string,
	count int,
	items []moduleCategory,
	allModules map[string][]resolver.LinkStatus,
	cfg *config.DotsConfig,
	entryStyle lipgloss.Style,
) {
	if count == 0 {
		return
	}

	catStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("243"))
	if strings.HasPrefix(title, "✔") {
		catStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("76"))
	} else if strings.HasPrefix(title, "✖") {
		catStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	} else if strings.HasPrefix(title, "⚠") {
		catStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	}
	fmt.Println(catStyle.Render(fmt.Sprintf("%s (%d modules)", title, count)))

	for _, item := range items {
		var hasVariants bool
		var variantNames []string
		var activeVariant string

		vInfo, err := resolver.GetModuleVariantInfo(cfg, item.name)
		if err == nil && vInfo != nil && vInfo.HasVariants {
			hasVariants = true
			variantNames = vInfo.Variants
			active, _ := resolver.GetActiveVariant(cfg, item.name)
			activeVariant = active
		}

		if hasVariants {
			fmt.Printf("  %s\n", entryStyle.Render(item.name))
			for _, v := range variantNames {
				if v == activeVariant {
					fmt.Printf("    %s %s %s\n", entryStyle.Render("●"), entryStyle.Render(v), ui.DimStyle.Render("← active"))
				} else {
					fmt.Printf("    %s %s\n", ui.DimStyle.Render("○"), entryStyle.Render(v))
				}
			}
		} else {
			fmt.Printf("  %s %s\n", entryStyle.Render(item.name), ui.DimStyle.Render("("+item.info+")"))
		}
	}
	fmt.Println()
}

// ─── Table output ───────────────────────────────────────────────────────────

func stateSymbol(state resolver.LinkState) string {
	switch state {
	case resolver.StateLinked:
		return "✔ linked"
	case resolver.StateConflict:
		return "✖ conflict"
	case resolver.StateUnsafe:
		return "⚠ unsafe"
	case resolver.StatePending:
		return "○ unlinked"
	case resolver.StateMissing:
		return "… missing"
	default:
		return string(state)
	}
}

func renderTable(
	allModules map[string][]resolver.LinkStatus,
	stateFilter map[string]bool,
	cfg *config.DotsConfig,
	showBackups bool,
) {
	columns := []table.Column{
		{Title: "Module", Width: 20},
		{Title: "Source", Width: 24},
		{Title: "Destination", Width: 36},
		{Title: "State", Width: 12},
		{Title: "Backup", Width: 16},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
	)

	var rows []table.Row
	total := 0

	sortedNames := sortedModuleNames(allModules)

	for _, moduleName := range sortedNames {
		statuses := allModules[moduleName]
		firstRow := true

		for _, st := range statuses {
			if stateFilter != nil && !stateFilter[string(st.State)] {
				continue
			}

			if showBackups && st.BackupPath == "" {
				continue
			}

			modCell := ""
			if firstRow {
				modCell = moduleName
				firstRow = false
			}

			backupCell := ""
			if st.BackupPath != "" {
				backupCell = "⚠ " + filepath.Base(st.BackupPath)
			}

			srcCell := st.ConfigSource
			if srcCell == "" {
				srcCell = filepath.Base(st.Source)
			}

			destCell := st.ConfigDest
			if destCell == "" {
				destCell = shortDisplayPath(st.Destination, cfg.HomeDir)
			}

			rows = append(rows, table.Row{
				modCell,
				srcCell,
				destCell,
				stateSymbol(st.State),
				backupCell,
			})
			total++
		}
	}

	if total == 0 {
		ui.PrintWarning("No dotfiles match the given filters.")
		return
	}

	t.SetRows(rows)
	tableStyle := lipgloss.NewStyle().Padding(0, 1)
	fmt.Println(tableStyle.Render(ui.DimStyle.Render(t.View())))
	fmt.Printf("\n%s\n", ui.DimStyle.Render(fmt.Sprintf("Total: %d files", total)))
}

// ─── JSON output ────────────────────────────────────────────────────────────

func renderJSON(
	allModules map[string][]resolver.LinkStatus,
	stateFilter map[string]bool,
	cfg *config.DotsConfig,
	showBackups bool,
) error {
	type fileEntry struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
		State       string `json:"state"`
		BackupPath  string `json:"backup_path,omitempty"`
	}

	type moduleData struct {
		Files []fileEntry `json:"files"`
	}

	summary := map[string]int{
		"linked":   0,
		"unlinked": 0,
		"broken":   0,
		"missing":  0,
		"unsafe":   0,
	}

	modulesData := make(map[string]moduleData)
	homeDir := cfg.HomeDir

	jsonStateLabels := map[resolver.LinkState]string{
		resolver.StateLinked:   "linked",
		resolver.StatePending:  "unlinked",
		resolver.StateConflict: "broken",
		resolver.StateMissing:  "missing",
		resolver.StateUnsafe:   "unsafe",
	}

	sortedNames := sortedModuleNames(allModules)

	for _, moduleName := range sortedNames {
		statuses := allModules[moduleName]
		var files []fileEntry

		for _, st := range statuses {
			if stateFilter != nil && !stateFilter[string(st.State)] {
				continue
			}
			if showBackups && st.BackupPath == "" {
				continue
			}

			label := jsonStateLabels[st.State]
			if label == "" {
				label = string(st.State)
			}

			fe := fileEntry{
				Source:      filepath.Base(st.Source),
				Destination: shortDisplayPath(st.Destination, homeDir),
				State:       label,
			}
			if st.BackupPath != "" {
				fe.BackupPath = st.BackupPath
			}

			files = append(files, fe)
			summary[label]++
		}

		if len(files) > 0 {
			modulesData[moduleName] = moduleData{Files: files}
		}
	}

	output := map[string]interface{}{
		"modules": modulesData,
		"summary": summary,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func shortDisplayPath(path, homeDir string) string {
	if strings.HasPrefix(path, homeDir) {
		return "~" + strings.TrimPrefix(path, homeDir)
	}
	return path
}

func sortedModuleNames(allModules map[string][]resolver.LinkStatus) []string {
	names := make([]string, 0, len(allModules))
	for name := range allModules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

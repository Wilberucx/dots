package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cantoarch/dots/internal/config"
	"github.com/cantoarch/dots/internal/resolver"
	"github.com/cantoarch/dots/internal/ui"
	"github.com/cantoarch/dots/internal/yaml"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	// Override the skeleton RunE with the real implementation
	statusCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd)
	}
}

// stateLabels maps internal state to display labels.
var stateLabels = map[resolver.LinkState]string{
	resolver.StateLinked:   "linked",
	resolver.StatePending:  "unlinked",
	resolver.StateConflict: "broken",
	resolver.StateMissing:  "missing",
	resolver.StateUnsafe:   "unsafe",
}

func runStatus(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	modules := stringSliceFlag(cmd, "module")
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

	// Build state filter set with user-facing name mapping
	var stateSet map[string]bool
	if len(stateFilterFlags) > 0 {
		stateSet = make(map[string]bool)
		for _, s := range stateFilterFlags {
			// Map user-facing state names to internal ones
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

		// Filter by --backups flag
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

	// Display results
	displayCategoryWithModules("✔ Linked", len(linked), linked, allModules, cfg)
	displayCategoryWithModules("ℹ Unlinked", len(unlinked), unlinked, allModules, cfg)
	displayCategoryWithModules("✖ Broken", len(broken), broken, allModules, cfg)
	displayCategoryWithModules("⚠ Missing Source", len(missingSrc), missingSrc, allModules, cfg)
	displayCategoryWithModules("• Empty", len(notLinked), notLinked, allModules, cfg)

	// Summary
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
}

func displayCategoryWithModules(
	title string,
	count int,
	items []moduleCategory,
	allModules map[string][]resolver.LinkStatus,
	cfg *config.DotsConfig,
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
		yamlPath := filepath.Join(cfg.RepoRoot, item.name, "path.yaml")
		mappings, _ := yaml.ParsePathYAML(yamlPath, cfg.CurrentOS)
		variantInfo := yaml.DetectVariants(mappings)

		if variantInfo.HasVariants {
			active, _ := resolver.GetActiveVariant(cfg, item.name)
			fmt.Printf("  %s\n", ui.DimStyle.Render(item.name))
			for _, v := range variantInfo.Variants {
				if v == active {
					fmt.Printf("    %s %s %s\n", ui.SuccessStyle.Render("●"), ui.BoldStyle.Render(v), ui.DimStyle.Render("← active"))
				} else {
					fmt.Printf("    %s %s\n", ui.DimStyle.Render("○"), ui.DimStyle.Render(v))
				}
			}
		} else {
			fmt.Printf("  %s %s\n", ui.DimStyle.Render(item.name), ui.DimStyle.Render("("+item.info+")"))
		}
	}
	fmt.Println()
}

// ─── Table output ───────────────────────────────────────────────────────────

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
		{Title: "State", Width: 10},
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

			label := stateLabels[st.State]
			if label == "" {
				label = string(st.State)
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

			rows = append(rows, table.Row{
				modCell,
				filepath.Base(st.Source),
				shortDisplayPath(st.Destination, cfg.HomeDir),
				label,
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

// shortDisplayPath replaces homeDir with ~ for display.
func shortDisplayPath(path, homeDir string) string {
	if strings.HasPrefix(path, homeDir) {
		return "~" + strings.TrimPrefix(path, homeDir)
	}
	return path
}

// sortedModuleNames returns sorted keys from the allModules map.
func sortedModuleNames(allModules map[string][]resolver.LinkStatus) []string {
	names := make([]string, 0, len(allModules))
	for name := range allModules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

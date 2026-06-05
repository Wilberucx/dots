package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Wilberucx/dots/internal/checker"
	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Deep health check for your dotfiles setup",
		Long: `Run comprehensive diagnostics on your dotfiles repository.

Checks for:
  - Invalid syntax in dots.lua and path.yaml files
  - Missing source files referenced in configs
  - Broken or dangling symlinks
  - Unsafe paths (destinations outside $HOME)
  - YAML legacy modules that could be migrated to Lua
  - Module discovery issues
  - Active variant consistency`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd)
		},
	}

	doctorCmd.Flags().Bool("hints", true, "Show migration hints (YAML → Lua)")
	doctorCmd.Flags().Bool("fix", false, "Attempt to auto-fix detected issues (coming soon)")

	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	showHints, _ := cmd.Flags().GetBool("hints")
	_, _ = cmd.Flags().GetBool("fix") // reserved for future use

	ui.PrintHeader("Doctor — Dotfiles Diagnostics")

	exitCode := 0
	totalErrCount := 0

	// ── 1. Repository detection ───────────────────────────────────────
	checkRepoDetection(cfg, &exitCode)
	if exitCode > 0 {
		totalErrCount++
	}

	// ── 2. Syntax check ──────────────────────────────────────────────
	result := checker.RunSyntaxCheck(cfg, checker.CheckOptions{NoHints: !showHints})
	checker.CheckBrokenLinks(cfg, result)

	if len(result.Issues) > 0 {
		fmt.Printf("\n%s\n", ui.BoldStyle.Render("Issues Found:"))
		checker.PrintResult(result)

		errCount := 0
		warnCount := 0
		hintCount := 0
		for _, issue := range result.Issues {
			switch issue.Severity {
			case checker.SeverityError:
				errCount++
			case checker.SeverityWarning:
				warnCount++
			case checker.SeverityHint:
				hintCount++
			}
		}

		// Warnings and hints alone should NOT cause non-zero exit
		if errCount > 0 {
			exitCode = 1
			totalErrCount += errCount
		}

		fmt.Println()
		printDoctorVerdict(errCount, warnCount, hintCount)
	} else {
		fmt.Printf("\n%s\n\n", ui.SuccessStyle.Render("✔ No issues found."))
		printDoctorVerdict(0, 0, 0)
	}

	// ── 3. Module discovery check ─────────────────────────────────────
	prevExit := exitCode
	checkModuleDiscovery(cfg, &exitCode)
	if exitCode > prevExit {
		totalErrCount++
	}

	// ── 4. YAML legacy summary ────────────────────────────────────────
	if showHints {
		prevExit = exitCode
		checkYAMLLegacy(cfg, &exitCode)
		if exitCode > prevExit {
			totalErrCount++
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("doctor found %d error(s) — check the diagnostics above", totalErrCount)
	}

	return nil
}

// checkRepoDetection verifies the repository is properly detected.
func checkRepoDetection(cfg *config.DotsConfig, exitCode *int) {
	fmt.Printf("%s %s\n", ui.BoldStyle.Render("Repository:"), ui.SuccessStyle.Render(cfg.RepoRoot))

	markers := []struct {
		path   string
		name   string
		active bool
	}{
		{filepath.Join(cfg.RepoRoot, "init.lua"), "init.lua", false},
		{filepath.Join(cfg.RepoRoot, config.MarkerDir, config.MarkerConfig), ".dots/config.yaml", false},
		{filepath.Join(cfg.RepoRoot, config.LegacyMarker), "dots.toml", false},
	}

	for i := range markers {
		if _, err := os.Stat(markers[i].path); err == nil {
			markers[i].active = true
		}
	}

	activeCount := 0
	for _, m := range markers {
		if m.active {
			activeCount++
			if m.name == "init.lua" {
				fmt.Printf("  %s %s %s\n", ui.SuccessStyle.Render("✔"), m.name, ui.DimStyle.Render("(primary)"))
			} else {
				fmt.Printf("  %s %s %s\n", ui.WarningStyle.Render("⚠"), m.name, ui.DimStyle.Render("(legacy)"))
			}
		}
	}

	if activeCount == 0 {
		fmt.Printf("  %s\n", ui.ErrorStyle.Render("✘ No marker file found!"))
		*exitCode = 1
	}

	if activeCount > 1 {
		fmt.Printf("  %s\n", ui.WarningStyle.Render("⚠ Multiple markers found — init.lua takes priority"))
	}

	fmt.Printf("  %s %s\n", ui.DimStyle.Render("OS:"), cfg.CurrentOS)
	if cfg.IsLuaRepo {
		fmt.Printf("  %s %s\n", ui.DimStyle.Render("Format:"), ui.SuccessStyle.Render("Lua (recommended)"))
	} else {
		fmt.Printf("  %s %s\n", ui.DimStyle.Render("Format:"), ui.WarningStyle.Render("YAML (legacy)"))
	}

	fmt.Println()
}

// checkModuleDiscovery verifies that modules are properly detected.
func checkModuleDiscovery(cfg *config.DotsConfig, exitCode *int) {
	fmt.Printf("%s\n", ui.BoldStyle.Render("Modules:"))

	modDirs, err := cfg.GetModuleDirs(nil, nil)
	if err != nil {
		fmt.Printf("  %s\n", ui.ErrorStyle.Render(fmt.Sprintf("✘ Error: %v", err)))
		*exitCode = 1
		return
	}

	if len(modDirs) == 0 {
		fmt.Printf("  %s\n", ui.WarningStyle.Render("⚠ No modules found"))
		return
	}

	luaCount := 0
	yamlCount := 0
	for _, d := range modDirs {
		if d.Type != 0 { // non-zero = Lua module
			luaCount++
		} else {
			yamlCount++
		}
	}

	fmt.Printf("  %s %d modules found (%d Lua, %d YAML)\n", ui.SuccessStyle.Render("✔"), len(modDirs), luaCount, yamlCount)

	if yamlCount > 0 {
		fmt.Printf("  %s %d module(s) using legacy YAML format — run 'dots migrate' to convert\n",
			ui.WarningStyle.Render("⚠"), yamlCount)
	}

	for _, d := range modDirs {
		luaPath := filepath.Join(d.Path, "dots.lua")
		yamlPath := filepath.Join(d.Path, "path.yaml")
		hasLua := false
		hasYAML := false
		if _, err := os.Stat(luaPath); err == nil {
			hasLua = true
		}
		if _, err := os.Stat(yamlPath); err == nil {
			hasYAML = true
		}
		if hasLua && hasYAML {
			fmt.Printf("  %s Module '%s' has both dots.lua and path.yaml — using dots.lua\n",
				ui.WarningStyle.Render("⚠"), d.Name)
		}
	}

	for _, d := range modDirs {
		vInfo, err := resolver.GetModuleVariantInfo(cfg, d.Name)
		if err != nil {
			fmt.Printf("  %s Module '%s': variant detection error: %v\n",
				ui.WarningStyle.Render("⚠"), d.Name, err)
			continue
		}
		if vInfo != nil && vInfo.HasVariants {
			active, _ := resolver.GetActiveVariant(cfg, d.Name)
			if active == "" {
				fmt.Printf("  %s Module '%s' has variants %v but none is active\n",
					ui.DimStyle.Render("ℹ"), d.Name, vInfo.Variants)
			}
		}
	}

	fmt.Println()
}

// checkYAMLLegacy shows YAML legacy module information.
func checkYAMLLegacy(cfg *config.DotsConfig, exitCode *int) {
	modDirs, err := cfg.GetModuleDirs(nil, nil)
	if err != nil {
		return
	}

	yamlCount := 0
	for _, d := range modDirs {
		if d.Type == 0 {
			yamlCount++
		}
	}

	if yamlCount > 0 {
		fmt.Printf("%s\n", ui.BoldStyle.Render("Legacy YAML Modules:"))
		fmt.Printf("  %s %d module(s) still using path.yaml\n",
			ui.WarningStyle.Render("⚠"), yamlCount)
		fmt.Printf("  %s %s\n",
			ui.DimStyle.Render("→"),
			"Migrate to Lua: https://github.com/Wilberucx/dots/docs/lua-syntax.md")
		for _, d := range modDirs {
			if d.Type == 0 { // YAML module
				fmt.Printf("    • %s\n", d.Name)
			}
		}
		fmt.Println()
	}

	if _, err := os.Stat(filepath.Join(cfg.RepoRoot, config.LegacyMarker)); err == nil {
		fmt.Printf("  %s Legacy marker '%s' found — consider removing after migration\n",
			ui.WarningStyle.Render("⚠"), config.LegacyMarker)
		fmt.Println()
	}
}

// printDoctorVerdict prints the final health verdict.
func printDoctorVerdict(errCount, warnCount, hintCount int) {
	var verdict string
	var style lipgloss.Style

	switch {
	case errCount > 0:
		verdict = fmt.Sprintf("Unhealthy — %d error(s), %d warning(s), %d hint(s)", errCount, warnCount, hintCount)
		style = ui.ErrorStyle
	case warnCount > 0:
		verdict = fmt.Sprintf("Needs Attention — %d warning(s), %d hint(s)", warnCount, hintCount)
		style = ui.WarningStyle
	case hintCount > 0:
		verdict = fmt.Sprintf("Good — %d hint(s) available", hintCount)
		style = ui.InfoStyle
	default:
		verdict = "Healthy — no issues found"
		style = ui.SuccessStyle
	}

	fmt.Printf("%s %s\n\n", ui.BoldStyle.Render("Health:"), style.Render(verdict))
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/writer"
	"github.com/spf13/cobra"
)

const markerContent = `# .dots/config.yaml — marker for the dots CLI
# This file identifies this directory as a dotfiles repository managed by dots.
# See: https://github.com/Wilberucx/dots

[dots]
version = "1"
`

func init() {
	initCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInit()
	}
}

func runInit() error {
	ui.PrintHeader("Init")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	markerDir := filepath.Join(cwd, config.MarkerDir)
	markerPath := filepath.Join(markerDir, config.MarkerConfig)
	legacyPath := filepath.Join(cwd, config.LegacyMarker)

	// Check if already initialized
	if config.IsDotfilesRepo(cwd) {
		if _, err := os.Stat(markerPath); err == nil {
			ui.PrintWarning(fmt.Sprintf("Dotfiles already initialized in %s", cwd))
			return nil
		}
		// Legacy marker found — migrate
		ui.PrintWarning(fmt.Sprintf("Legacy marker '%s' found — migrating to new format...", config.LegacyMarker))
		return migrateFromLegacy(cwd, markerDir, markerPath, legacyPath)
	}

	// Create .dots directory and config.yaml
	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return fmt.Errorf("creating marker directory: %w", err)
	}

	if err := writer.WriteConfigYAML(markerPath, markerContent); err != nil {
		return fmt.Errorf("creating marker file: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Successfully initialized dotfiles repository in %s", cwd))
	ui.PrintInfo(fmt.Sprintf("Created '%s/%s'.", config.MarkerDir, config.MarkerConfig))

	// DOTS_REPO prompt
	_, shellConfig := writer.DetectShell()
	exportLine := fmt.Sprintf("export DOTS_REPO=\"%s\"", cwd)

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("DOTS_REPO") + ui.DimStyle.Render(" tells dots where to find your dotfiles."))
	ui.PrintInfo(fmt.Sprintf("Without it, you must be inside your dotfiles directory or use --path."))
	fmt.Println()

	// Try to create directory if shell config doesn't exist
	shellConfigDir := filepath.Dir(shellConfig)
	if _, err := os.Stat(shellConfigDir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(shellConfigDir, 0755); mkErr != nil {
			ui.PrintWarning(fmt.Sprintf("Could not create directory %s: %v", shellConfigDir, mkErr))
		}
	}

	addRepo := ui.RunConfirm(
		fmt.Sprintf("Add DOTS_REPO to your %s?", filepath.Base(shellConfig)),
		true,
	)

	if addRepo {
		appendLine := fmt.Sprintf("\n# dots: dotfiles repository\n%s\n", exportLine)
		if err := writer.AppendToFile(shellConfig, appendLine); err != nil {
			return fmt.Errorf("writing to %s: %w", shellConfig, err)
		}
		ui.PrintSuccess(fmt.Sprintf("Added to %s", shellConfig))
		ui.PrintInfo(fmt.Sprintf("Run 'source %s' or restart your terminal.", shellConfig))
	} else {
		ui.PrintInfo("You can add it manually:")
		fmt.Printf("  %s\n", ui.SuccessStyle.Render(exportLine))
		fmt.Println()
		ui.PrintInfo(fmt.Sprintf("Or use 'dots --path %s <command>' instead.", cwd))
	}

	return nil
}

func migrateFromLegacy(cwd, markerDir, markerPath, legacyPath string) error {
	legacyContent, err := os.ReadFile(legacyPath)
	if err != nil {
		return fmt.Errorf("reading legacy %s: %w", config.LegacyMarker, err)
	}

	if err := os.MkdirAll(markerDir, 0755); err != nil {
		return fmt.Errorf("creating marker directory: %w", err)
	}

	content := fmt.Sprintf("# %s/%s — marker for the dots CLI\n# Migrated from legacy %s\n\n%s",
		config.MarkerDir, config.MarkerConfig, config.LegacyMarker, string(legacyContent))

	if err := writer.WriteConfigYAML(markerPath, content); err != nil {
		return fmt.Errorf("writing marker file: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Migrated %s to %s/%s", config.LegacyMarker, config.MarkerDir, config.MarkerConfig))
	ui.PrintInfo(fmt.Sprintf("You can safely delete %s if desired.", config.LegacyMarker))
	return nil
}

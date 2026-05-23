package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Wilberucx/dots/internal/system"
	"github.com/Wilberucx/dots/internal/transaction"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/writer"
	"github.com/spf13/cobra"
)

func init() {
	adoptCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runAdopt(cmd, args)
	}
}

func runAdopt(cmd *cobra.Command, args []string) error {
	ui.PrintHeader("Adopting Configuration")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Verify the source exists
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// Safety check — outside HOME
	if !system.IsSafePath(absPath) {
		ui.PrintWarning(fmt.Sprintf("%s is outside HOME.", absPath))
		if !ui.RunConfirm("Proceed anyway?", false) {
			ui.PrintInfo("Adoption cancelled.")
			return nil
		}
	}

	name := stringFlag(cmd, "name")
	if name == "" {
		// Use InquirerPy-like prompt — for now, use the file/dir base name
		name = promptModuleName(absPath)
	}
	dryRun := boolFlag(cmd, "dry-run")

	moduleDir := filepath.Join(cfg.RepoRoot, name)
	yamlPath := filepath.Join(moduleDir, "path.yaml")
	destination := writer.DestinationStr(absPath, cfg.HomeDir)
	data := writer.LoadModuleData(yamlPath)

	sourceName := filepath.Base(absPath)
	targetFile := filepath.Join(moduleDir, sourceName)

	// Check if module dir already exists
	moduleExists := false
	if stat, err := os.Stat(moduleDir); err == nil && stat.IsDir() {
		moduleExists = true
	}

	tx := &transaction.TransactionLog{}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Case 1: Module exists AND destination already declared → offer variant
	if moduleExists && writer.IsDestinationDeclared(data, destination) {
		ui.PrintInfo(fmt.Sprintf("Module '%s' already declares %s as a destination.", name, destination))

		if !ui.RunConfirm(fmt.Sprintf("Create a new variant in '%s' for this file?", name), true) {
			ui.PrintInfo("Adoption cancelled.")
			return nil
		}

		variantName := promptVariantName(sourceName)
		variantDir := filepath.Join(moduleDir, variantName)
		variantTarget := filepath.Join(variantDir, sourceName)
		entry := map[string]interface{}{
			"source":      fmt.Sprintf("%s/%s", variantName, sourceName),
			"destination": destination,
		}

		return doAdopt(absPath, variantTarget, yamlPath, entry, tx, dryRun, "Updated", func() {
			if !dryRun {
				ui.PrintInfo(fmt.Sprintf("Run 'dots link -m %s --variant %s'", name, variantName))
			}
		})
	}

	// Case 2: Normal adopt
	entry := map[string]interface{}{
		"source":      sourceName,
		"destination": destination,
	}
	return doAdopt(absPath, targetFile, yamlPath, entry, tx, dryRun, "Created", func() {
		if !dryRun {
			ui.PrintInfo(fmt.Sprintf("Run 'dots link -m %s'", name))
		}
	})
}

func doAdopt(absPath, targetFile, yamlPath string, entry map[string]interface{}, tx *transaction.TransactionLog, dryRun bool, label string, onSuccess func()) error {
	if dryRun {
		ui.PrintInfo(fmt.Sprintf("[DRY] Would move %s → %s", absPath, targetFile))
		ui.PrintInfo(fmt.Sprintf("[DRY] Would write entry to %s: %v", yamlPath, entry))
		return nil
	}

	// Check if target already exists
	if _, err := os.Stat(targetFile); err == nil {
		return fmt.Errorf("%s already exists in repo", targetFile)
	}

	// Move the file into the repo
	if err := tx.Move(absPath, targetFile); err != nil {
		return fmt.Errorf("moving file into repo: %w", err)
	}

	// Append entry to path.yaml
	if err := writer.AppendFileEntry(yamlPath, entry); err != nil {
		return fmt.Errorf("writing to path.yaml: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Moved %s → %s/", filepath.Base(absPath), filepath.Dir(targetFile)))
	ui.PrintSuccess(fmt.Sprintf("%s %s", label, yamlPath))
	tx.Commit()
	onSuccess()
	return nil
}

// promptModuleName asks the user for a module name, with a default based on the path.
func promptModuleName(path string) string {
	base := filepath.Base(path)
	// Capitalize first letter as default
	if len(base) > 0 {
		base = string(base[0]-32) + base[1:]
	}
	return ui.RunPrompt("Module name:", base)
}

// promptVariantName asks the user for a variant name, with a default based on the source file stem.
func promptVariantName(sourceName string) string {
	stem := sourceName
	if ext := filepath.Ext(stem); ext != "" {
		stem = stem[:len(stem)-len(ext)]
	}
	return ui.RunPrompt("Variant name (will be used as subfolder):", stem)
}

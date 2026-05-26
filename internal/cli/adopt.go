package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	// ── Detect existing symlink pointing into the repo ────────────
	if fi, lstatErr := os.Lstat(absPath); lstatErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		if linkTarget, readErr := os.Readlink(absPath); readErr == nil {
			if !filepath.IsAbs(linkTarget) {
				linkTarget = filepath.Join(filepath.Dir(absPath), linkTarget)
			}
			linkTarget, _ = filepath.EvalSymlinks(linkTarget)

			if strings.HasPrefix(linkTarget, cfg.RepoRoot) {
				// Already managed by dots — detect module and add config entry
				relPath, _ := filepath.Rel(cfg.RepoRoot, linkTarget)
				parts := strings.SplitN(relPath, string(filepath.Separator), 2)
				detectedModule := ""
				if len(parts) > 0 {
					detectedModule = parts[0]
				}

				existingName := stringFlag(cmd, "name")
				if existingName == "" && detectedModule != "" {
					existingName = detectedModule
				} else if existingName == "" {
					existingName = promptModuleName(absPath)
				}

				ui.PrintInfo(fmt.Sprintf(
					"'%s' is already a symlink to the repo:\n  → %s (module: %s)",
					shortDisplayPath(absPath, cfg.HomeDir),
					shortDisplayPath(linkTarget, cfg.HomeDir),
					detectedModule,
				))

				if !ui.RunConfirm(fmt.Sprintf("Add file entry to '%s' config?", existingName), true) {
					ui.PrintInfo("Adoption cancelled.")
					return nil
				}

				dryRun := boolFlag(cmd, "dry-run")
				moduleDir := filepath.Join(cfg.RepoRoot, existingName)
				luaPath := filepath.Join(moduleDir, "dots.lua")
				yamlPath := filepath.Join(moduleDir, "path.yaml")
				dest := writer.DestinationStr(absPath, cfg.HomeDir)
				sourceName := filepath.Base(absPath)

				if _, statErr := os.Stat(luaPath); statErr == nil {
					return adoptExistingSymlink(luaPath, moduleDir, sourceName, dest, existingName, dryRun)
				} else if _, statErr := os.Stat(yamlPath); statErr == nil {
					return adoptExistingSymlinkYAML(yamlPath, moduleDir, sourceName, dest, existingName, dryRun)
				} else {
					return fmt.Errorf("module '%s' has no config file (dots.lua or path.yaml)", existingName)
				}
			}
		}
	}

	name := stringFlag(cmd, "name")
	if name == "" {
		name = promptModuleName(absPath)
	}
	dryRun := boolFlag(cmd, "dry-run")

	moduleDir := filepath.Join(cfg.RepoRoot, name)

	// Detect module type: Lua (dots.lua) or YAML (path.yaml)
	luaConfigPath := filepath.Join(moduleDir, "dots.lua")
	yamlPath := filepath.Join(moduleDir, "path.yaml")
	isLua := false
	if _, err := os.Stat(luaConfigPath); err == nil {
		isLua = true
	}

	destination := writer.DestinationStr(absPath, cfg.HomeDir)
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

	if isLua {
		return adoptLuaModule(absPath, targetFile, luaConfigPath, moduleDir, name, sourceName, destination, tx, dryRun)
	}

	// ── YAML adoption (existing behavior) ──
	data := writer.LoadModuleData(yamlPath)

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

	// Case 2: Normal adopt (YAML)
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

// adoptLuaModule adopts a file into a Lua-based module.
func adoptLuaModule(absPath, targetFile, luaConfigPath, moduleDir, name, sourceName, destination string, tx *transaction.TransactionLog, dryRun bool) error {
	if dryRun {
		ui.PrintInfo(fmt.Sprintf("[DRY] Would move %s → %s", absPath, targetFile))
		ui.PrintInfo(fmt.Sprintf("[DRY] Would add file(%q, %q) to %s", sourceName, destination, luaConfigPath))
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

	// Append file entry to dots.lua
	if err := appendLuaFileEntry(luaConfigPath, sourceName, destination); err != nil {
		return fmt.Errorf("writing to dots.lua: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Moved %s → %s/", filepath.Base(absPath), filepath.Dir(targetFile)))
	ui.PrintSuccess(fmt.Sprintf("Added file entry to %s", luaConfigPath))
	tx.Commit()
	ui.PrintInfo(fmt.Sprintf("Run 'dots link -m %s'", name))
	return nil
}

// adoptExistingSymlink handles adoption of a file already symlinked into a Lua module.
// Skips the file move since the file is already in the repo.
func adoptExistingSymlink(luaConfigPath, moduleDir, sourceName, destination, moduleName string, dryRun bool) error {
	if dryRun {
		ui.PrintInfo(fmt.Sprintf("[DRY] Would add file(%q, %q) to %s", sourceName, destination, luaConfigPath))
		return nil
	}

	// Verify the source file actually exists in the module directory
	sourcePath := filepath.Join(moduleDir, sourceName)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		ui.PrintWarning(fmt.Sprintf("Source file %s not found in module directory", sourceName))
		ui.PrintInfo(fmt.Sprintf("Expected at: %s", sourcePath))
		if !ui.RunConfirm("Add entry anyway?", false) {
			ui.PrintInfo("Adoption cancelled.")
			return nil
		}
	}

	if err := appendLuaFileEntry(luaConfigPath, sourceName, destination); err != nil {
		return fmt.Errorf("writing to dots.lua: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Added file entry to %s", luaConfigPath))
	ui.PrintSuccess(fmt.Sprintf("  file(%q, %q)", sourceName, destination))
	ui.PrintInfo(fmt.Sprintf("Run 'dots link -m %s' to ensure the symlink is correct.", moduleName))
	return nil
}

// adoptExistingSymlinkYAML handles adoption of a file already symlinked into a YAML module.
// Skips the file move since the file is already in the repo.
func adoptExistingSymlinkYAML(yamlPath, moduleDir, sourceName, destination, moduleName string, dryRun bool) error {
	if dryRun {
		ui.PrintInfo(fmt.Sprintf("[DRY] Would write entry to %s: source=%s, destination=%s", yamlPath, sourceName, destination))
		return nil
	}

	entry := map[string]interface{}{
		"source":      sourceName,
		"destination": destination,
	}

	if err := writer.AppendFileEntry(yamlPath, entry); err != nil {
		return fmt.Errorf("writing to path.yaml: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Added entry to %s", yamlPath))
	ui.PrintInfo(fmt.Sprintf("Run 'dots link -m %s' to ensure the symlink is correct.", moduleName))
	return nil
}

// appendLuaFileEntry adds a file() entry to an existing dots.lua file.
func appendLuaFileEntry(luaPath, source, destination string) error {
	// Read existing content
	data, err := os.ReadFile(luaPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Insert before the closing "}" of the return statement
	entry := fmt.Sprintf("    file(%q, %q),\n", source, destination)

	// Find the last "}" that closes the return table
	idx := lastIndexOutsideString(content, "}")
	if idx < 0 {
		return fmt.Errorf("cannot parse dots.lua: no closing brace found")
	}

	// Check if we need to create a files section first
	hasFiles := false
	if idxFiles := indexOutsideString(content, "files = {"); idxFiles >= 0 {
		hasFiles = true
	}

	if !hasFiles {
		// Need to create files section
		filesSection := fmt.Sprintf("\n  files = {\n    file(%q, %q),\n  },\n", source, destination)
		newContent := content[:idx] + filesSection + content[idx:]
		return os.WriteFile(luaPath, []byte(newContent), 0644)
	}

	newContent := content[:idx] + "  " + entry + content[idx:]
	return os.WriteFile(luaPath, []byte(newContent), 0644)
}

// indexOutsideString finds the first occurrence of substr in s, ignoring occurrences inside strings.
func indexOutsideString(s, substr string) int {
	inString := false
	strChar := byte(0)
	for i := 0; i < len(s); i++ {
		if inString {
			if s[i] == strChar && (i == 0 || s[i-1] != '\\') {
				inString = false
			}
			continue
		}
		if s[i] == '"' || s[i] == '\'' {
			inString = true
			strChar = s[i]
			continue
		}
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// lastIndexOutsideString finds the last occurrence of substr in s, ignoring occurrences inside strings.
func lastIndexOutsideString(s, substr string) int {
	inString := false
	strChar := byte(0)
	lastIdx := -1
	for i := 0; i < len(s); i++ {
		if inString {
			if s[i] == strChar && (i == 0 || s[i-1] != '\\') {
				inString = false
			}
			continue
		}
		if s[i] == '"' || s[i] == '\'' {
			inString = true
			strChar = s[i]
			continue
		}
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			lastIdx = i
		}
	}
	return lastIdx
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
		runes := []rune(base)
		runes[0] = unicode.ToUpper(runes[0])
		base = string(runes)
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

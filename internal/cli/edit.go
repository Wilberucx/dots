package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Wilberucx/dots/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	editCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runEdit(cmd, args)
	}
}

func runEdit(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	module := args[0]
	modulePath := filepath.Join(cfg.RepoRoot, module)

	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		return fmt.Errorf("module '%s' not found in %s", module, cfg.RepoRoot)
	}

	target := modulePath
	if boolFlag(cmd, "config") {
		yamlPath := filepath.Join(modulePath, "path.yaml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return fmt.Errorf("module '%s' does not have a path.yaml file", module)
		}
		target = yamlPath
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	ui.PrintInfo(fmt.Sprintf("Opening %s with %s...", target, editor))

	editCmd := exec.Command(editor, target)
	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("could not open editor: %w", err)
	}

	return nil
}

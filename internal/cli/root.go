package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cantoarch/dots/internal/config"
	"github.com/cantoarch/dots/internal/ui"
	"github.com/spf13/cobra"
)

// rootCmd is the base command for dots.
var rootCmd = &cobra.Command{
	Use:   "dots",
	Short: "dots — dotfile manager",
	Long: `dots — Declarative, symlink-based dotfile manager for Linux, macOS, and Windows.

Manages your dotfiles through symlinks, with support for variants,
dependencies, backups, and cross-platform configuration.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Check for updates in background goroutine
		go checkForUpdates()
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Println(Version)
			return nil
		}
		return cmd.Help()
	},
}

// repoPath is the --path flag value, set in PersistentPreRunE.
var repoPath string

func init() {
	rootCmd.PersistentFlags().StringVarP(&repoPath, "path", "p", "", "Path to dotfiles repository (overrides auto-detection)")

	// Register subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(migrateCmd)

	// backup is a group with subcommands
	rootCmd.AddCommand(backupCmd)
}

// loadConfig loads DotsConfig, respecting --path flag.
func loadConfig() (*config.DotsConfig, error) {
	if repoPath != "" {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			return nil, err
		}
		if !config.IsDotfilesRepo(absPath) {
			return nil, fmt.Errorf("not a dotfiles repository: %s", absPath)
		}
		os.Setenv("DOTS_REPO", absPath)
	}
	return config.Load()
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrorStyle.Render("Error: "+err.Error()))
		os.Exit(1)
	}
	notifyIfNeeded()
}

// ─── Helper for repeatable string flags ──────────────────────────────────────

// stringSliceFlag returns a comma-separated string slice from a cobra flag.
func stringSliceFlag(cmd *cobra.Command, name string) []string {
	val, _ := cmd.Flags().GetStringSlice(name)
	return val
}

// stringFlag returns a string value from a cobra flag.
func stringFlag(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

// boolFlag returns a bool value from a cobra flag.
func boolFlag(cmd *cobra.Command, name string) bool {
	val, _ := cmd.Flags().GetBool(name)
	return val
}

// ─── Subcommands ────────────────────────────────────────────────────────────

// initCmd represents the `dots init` command.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a dotfiles repository",
	Long:  "Initialize a new dotfiles repository by creating .dots/config.yaml marker file.",
}

// linkCmd represents the `dots link` command.
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Create symlinks for dotfiles modules",
	Long: `Create symlinks for dotfiles modules.

By default, links all modules. Use --interactive to select specific modules.`,
}

func init() {
	linkCmd.Flags().StringSliceP("module", "m", nil, "Link only specific modules (repeatable)")
	linkCmd.Flags().StringSliceP("type", "t", nil, "Link only modules of this type (repeatable)")
	linkCmd.Flags().Bool("dry-run", false, "Show what would happen")
	linkCmd.Flags().Bool("force", false, "Overwrite existing symlinks")
	linkCmd.Flags().BoolP("interactive", "i", false, "Interactively select modules to link")
	linkCmd.Flags().String("variant", "", "Specific variant to use")
}

// unlinkCmd represents the `dots unlink` command.
var unlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Remove symlinks for dotfiles modules",
	Long:  "Remove symlinks for dotfiles modules.",
}

func init() {
	unlinkCmd.Flags().StringSliceP("module", "m", nil, "Unlink only specific modules (repeatable)")
	unlinkCmd.Flags().Bool("dry-run", false, "Show what would happen")
	unlinkCmd.Flags().BoolP("interactive", "i", false, "Interactively select modules to unlink")
}

// statusCmd represents the `dots status` command.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show symlink status grouped by state",
	Long:  "Show status of all dotfiles modules grouped by state.",
}

func init() {
	statusCmd.Flags().StringSliceP("module", "m", nil, "Show status only for specific modules (repeatable)")
	statusCmd.Flags().StringSliceP("type", "t", nil, "Show status only for modules of this type (repeatable)")
	statusCmd.Flags().StringSliceP("state", "s", nil, "Filter by state: linked, unlinked, broken, missing, unsafe (repeatable)")
	statusCmd.Flags().StringP("format", "f", "default", "Output format: default, table, json")
	statusCmd.Flags().Bool("backups", false, "Show only mappings with .orig backup files")
}

// listCmd represents the `dots list` command.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List modules or backups",
	Long:  "List modules or backups with optional filters.",
	Aliases: []string{"ls"},
}

func init() {
	listCmd.Flags().Bool("linked", false, "Show linked modules")
	listCmd.Flags().Bool("unlinked", false, "Show unlinked modules")
	listCmd.Flags().Bool("broken", false, "Show broken modules")
	listCmd.Flags().Bool("variant", false, "Show variants")
	listCmd.Flags().Bool("backups", false, "Show backup (.orig) files")
}

// adoptCmd represents the `dots adopt` command.
var adoptCmd = &cobra.Command{
	Use:   "adopt [path]",
	Short: "Import a config file into the dotfiles repo",
	Long:  "Import a config file into the dotfiles repo and register it in path.yaml.",
	Args:  cobra.ExactArgs(1),
}

func init() {
	adoptCmd.Flags().StringP("name", "n", "", "Module name (e.g. Zsh)")
	adoptCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
}

// editCmd represents the `dots edit` command.
var editCmd = &cobra.Command{
	Use:   "edit [module]",
	Short: "Open a module folder or its path.yaml in $EDITOR",
	Long:  "Open a module folder or its path.yaml in your $EDITOR.",
	Args:  cobra.ExactArgs(1),
}

func init() {
	editCmd.Flags().BoolP("config", "c", false, "Edit the path.yaml configuration file")
}

// installCmd represents the `dots install` command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install dependencies declared in path.yaml files",
	Long:  "Install dependencies declared in path.yaml files across all modules.",
}

func init() {
	installCmd.Flags().Bool("dry-run", false, "Show commands without executing")
	installCmd.Flags().StringSliceP("module", "m", nil, "Install deps only for specific modules (repeatable)")
	installCmd.Flags().StringSliceP("type", "t", nil, "Install deps only for modules of this type (repeatable)")
}

// migrateCmd represents the `dots migrate` command.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate path.yaml files from v2 to v3 format",
	Long:  "Migrate all path.yaml files in the repository from v2 to v3 format.",
}

func init() {
	migrateCmd.Flags().Bool("dry-run", false, "Preview changes without modifying files")
	migrateCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
}

// ─── backup group (dots backup {run,list,diff}) ──────────────────────────────

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup dotfiles repository",
	Long:  "Backup dotfiles with git commit and optional push.",
}

var backupRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a backup",
	Long:  "Backup dotfiles with git commit and optional push.",
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent backups",
	Long:  "List the most recent backups from git history.",
}

var backupDiffCmd = &cobra.Command{
	Use:   "diff [ref]",
	Short: "Show diff since last backup",
	Long:  "Show what changed since the last backup or a specific reference.",
	Args:  cobra.MaximumNArgs(1),
}

func init() {
	backupRunCmd.Flags().Bool("no-push", false, "Skip push to remote after commit")
	backupRunCmd.Flags().Bool("no-sync", false, "Skip remote sync check, push directly")
	backupRunCmd.Flags().StringP("message", "m", "", "Commit message (default: timestamp)")

	backupListCmd.Flags().IntP("limit", "n", 10, "Number of backups to show")

	backupDiffCmd.Flags().String("ref", "HEAD~1", "Commit or ref to compare against HEAD")

	backupCmd.AddCommand(backupRunCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupDiffCmd)
}

// ─── Version flag ────────────────────────────────────────────────────────────

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(Version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Flags().BoolP("version", "v", false, "Show version and exit")
}

// Version is set at build time via -ldflags.
var Version = "0.8.1-dev (go-migration)"

// checkForUpdates and notifyIfNeeded are implemented in updates.go

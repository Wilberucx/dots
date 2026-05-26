package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts for bash, zsh, fish, and powershell.
// Hidden from general help — this is for shell integration, not human consumption.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for dots.

To load completions in your current shell session:

    Bash:       source <(dots completion bash)
    Zsh:        source <(dots completion zsh)
    Fish:       dots completion fish | source
    PowerShell: dots completion powershell | Out-String | Invoke-Expression

To persist completions, add the line above to your shell's configuration file
(e.g., ~/.bashrc, ~/.zshrc, ~/.config/fish/config.fish).`,

	DisableFlagsInUseLine: true,
	Hidden:                true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s\n\nSupported shells: bash, zsh, fish, powershell", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

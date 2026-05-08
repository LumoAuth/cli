package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:                   "completion <bash|zsh|fish|powershell>",
	Short:                 "Generate shell completion scripts",
	Long:                  completionLongHelp,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

const completionLongHelp = `Generate shell completion scripts for the lumo CLI.

To load completions:

  Bash:
    $ source <(lumo completion bash)
    # Persist:
    $ lumo completion bash | sudo tee /etc/bash_completion.d/lumo > /dev/null

  Zsh:
    # If shell completion isn't already enabled in your zsh, run:
    $ echo "autoload -U compinit; compinit" >> ~/.zshrc
    # Then:
    $ lumo completion zsh > "${fpath[1]}/_lumo"
    $ exec zsh

  fish:
    $ lumo completion fish | source
    # Persist:
    $ lumo completion fish > ~/.config/fish/completions/lumo.fish

  PowerShell:
    PS> lumo completion powershell | Out-String | Invoke-Expression
    # Persist by adding the line above to your $PROFILE.
`

func init() {
	rootCmd.AddCommand(completionCmd)
}

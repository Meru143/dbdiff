package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate the autocompletion script for the specified shell",
	Long: `To load completions:

Bash:

  $ source <(dbdiff completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ dbdiff completion bash > /etc/bash_completion.d/dbdiff
  # macOS:
  $ dbdiff completion bash > $(brew --prefix)/etc/bash_completion.d/dbdiff

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ dbdiff completion zsh > "${fpath[1]}/_dbdiff"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ dbdiff completion fish | source

  # To load completions for each session, execute once:
  $ dbdiff completion fish > ~/.config/fish/completions/dbdiff.fish

PowerShell:

  PS> dbdiff completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> dbdiff completion powershell > dbdiff.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}

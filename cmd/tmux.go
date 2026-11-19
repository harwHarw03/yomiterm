package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tmuxCmd = &cobra.Command{
	Use:   "tmux-conf",
	Short: "Print ready-to-use tmux.conf keybindings for popup dictionary lookups",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(`# YomiTerm tmux integration (paste into ~/.tmux.conf)
# Prefix + y: Look up clipboard text (WSL2 / Wayland / X11 / macOS / tmux buffer) in a floating popup
bind-key y display-popup -E -w 70% -h 60% -T " YomiTerm Lookup " \
  "yomiterm lookup --clipboard | less -R"

# Prefix + Y: Prompt for a word/phrase and look it up in a floating popup
bind-key Y command-prompt -p "YomiTerm:" \
  "display-popup -E -w 70% -h 60% -T ' YomiTerm: %% ' 'yomiterm lookup \"%%\" | less -R'"
`)
	},
}

func init() {
	rootCmd.AddCommand(tmuxCmd)
}

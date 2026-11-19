package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harw/yomiterm/internal/lookup"
	"github.com/harw/yomiterm/internal/tui"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read <file.md>",
	Short: "Open an interactive CJK-aware Markdown reader with popup dictionary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawBytes, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", args[0], err)
		}

		cfg, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()

		engine, err := lookup.NewEngine(database)
		if err != nil {
			return err
		}

		model, err := tui.NewModel(filepath.Base(args[0]), string(rawBytes), engine, cfg)
		if err != nil {
			return err
		}

		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(readCmd)
}

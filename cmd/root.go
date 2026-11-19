package cmd

import (
	"fmt"
	"os"

	"github.com/harw/yomiterm/internal/config"
	"github.com/harw/yomiterm/internal/db"
	"github.com/spf13/cobra"
)

var (
	flagDBPath     string
	flagConfigPath string
)

var rootCmd = &cobra.Command{
	Use:   "yomiterm",
	Short: "Offline-first CJK dictionary lookup & interactive Markdown reader",
	Long:  "YomiTerm combines a Yomitan-compatible SQLite lookup engine with a CJK-aware TUI Markdown reader and fast CLI for tmux popups.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfigPath, "config", "", "Path to config.toml (default XDG config dir)")
	rootCmd.PersistentFlags().StringVar(&flagDBPath, "db", "", "Path to SQLite database (default XDG data dir)")
}

func loadConfigAndDB() (*config.Config, *db.DB, error) {
	cfg, err := config.Load(flagConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	if flagDBPath != "" {
		cfg.DatabasePath = flagDBPath
	}
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	return cfg, database, nil
}

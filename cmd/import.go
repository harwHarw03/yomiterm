package cmd

import (
	"fmt"
	"time"

	"github.com/harw/yomiterm/internal/importer"
	"github.com/spf13/cobra"
)

var (
	importPriority int
	importReplace  bool
)

var importCmd = &cobra.Command{
	Use:   "import <path.zip|dir>",
	Short: "Import a Yomitan dictionary (.zip archive or extracted directory) into SQLite",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()

		start := time.Now()
		res, err := importer.Import(database, args[0], importer.Options{
			Priority: importPriority,
			Replace:  importReplace,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Imported %q (rev: %s) in %s\n", res.Title, res.Revision, time.Since(start).Round(time.Millisecond))
		fmt.Printf("  Tags: %d | Terms: %d | Kanji: %d | TermMeta: %d | KanjiMeta: %d\n",
			res.Tags, res.Terms, res.Kanji, res.TermMeta, res.KanjiMeta)
		return nil
	},
}

func init() {
	importCmd.Flags().IntVar(&importPriority, "priority", 0, "Display priority (lower = higher; 0 = auto)")
	importCmd.Flags().BoolVar(&importReplace, "replace", false, "Overwrite if a dictionary with the same title already exists")
	rootCmd.AddCommand(importCmd)
}

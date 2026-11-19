package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var dictCmd = &cobra.Command{
	Use:   "dict",
	Short: "Manage imported dictionaries (list, enable, disable, priority, remove)",
}

var dictListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all imported dictionaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()

		dicts, err := database.ListDictionaries()
		if err != nil {
			return err
		}
		if len(dicts) == 0 {
			fmt.Println("No dictionaries imported yet. Run: yomiterm import <dict.zip|dir>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tENABLED\tPRIO\tTERMS\tKANJI\tTITLE\tREVISION")
		for _, d := range dicts {
			en := "yes"
			if !d.Enabled {
				en = "no"
			}
			fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%d\t%s\t%s\n",
				d.ID, en, d.Priority, d.TermCount, d.KanjiCount, d.Title, d.Revision)
		}
		return w.Flush()
	},
}

var dictEnableCmd = &cobra.Command{
	Use:   "enable <id|title>",
	Short: "Enable a dictionary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()
		return database.SetDictionaryEnabled(args[0], true)
	},
}

var dictDisableCmd = &cobra.Command{
	Use:   "disable <id|title>",
	Short: "Disable a dictionary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()
		return database.SetDictionaryEnabled(args[0], false)
	},
}

var dictPriorityCmd = &cobra.Command{
	Use:   "priority <id|title> <priority>",
	Short: "Set dictionary display priority (lower = higher priority)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		prio, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid priority %q: %w", args[1], err)
		}
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()
		return database.SetDictionaryPriority(args[0], prio)
	},
}

var dictRemoveCmd = &cobra.Command{
	Use:   "remove <id|title>",
	Short: "Delete an imported dictionary",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, database, err := loadConfigAndDB()
		if err != nil {
			return err
		}
		defer database.Close()
		return database.DeleteDictionary(args[0])
	},
}

func init() {
	dictCmd.AddCommand(dictListCmd, dictEnableCmd, dictDisableCmd, dictPriorityCmd, dictRemoveCmd)
	rootCmd.AddCommand(dictCmd)
}

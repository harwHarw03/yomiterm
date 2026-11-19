package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harw/yomiterm/internal/clipboard"
	"github.com/harw/yomiterm/internal/lookup"
	"github.com/spf13/cobra"
)

var (
	lookupJSON      bool
	lookupPlain     bool
	lookupClipboard bool
	lookupKanjiOnly bool
	lookupExamples  bool
)

var lookupCmd = &cobra.Command{
	Use:   "lookup [text]",
	Short: "Look up a word or phrase in local dictionaries",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var text string
		if lookupClipboard {
			var err error
			text, err = clipboard.ReadText()
			if err != nil {
				return err
			}
		} else if len(args) == 1 {
			text = args[0]
		} else {
			return fmt.Errorf("provide <text> or pass --clipboard")
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

		res, err := engine.Lookup(text)
		if err != nil {
			return err
		}

		if lookupJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(res)
		}

		showEx := lookupExamples || cfg.ShowExamples
		printLookupResult(res, lookupPlain, lookupKanjiOnly, showEx)
		return nil
	},
}

func printLookupResult(res *lookup.LookupResult, plain, kanjiOnly, showEx bool) {
	if len(res.Terms) == 0 && len(res.Kanji) == 0 {
		fmt.Printf("No entries found for %q\n", res.Query)
		return
	}

	headStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true)
	readStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	dictStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	exStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)

	if plain {
		headStyle = lipgloss.NewStyle()
		readStyle = lipgloss.NewStyle()
		tagStyle = lipgloss.NewStyle()
		dictStyle = lipgloss.NewStyle()
		exStyle = lipgloss.NewStyle()
	}

	if !kanjiOnly && len(res.Terms) > 0 {
		for idx, t := range res.Terms {
			if idx > 0 {
				fmt.Println(dictStyle.Render("────────────────────────────────────────"))
			}
			line := headStyle.Render(t.Expression)
			if t.Reading != "" && t.Reading != t.Expression {
				line += " " + readStyle.Render("["+t.Reading+"]")
			}
			if t.Reason != "" {
				line += " " + tagStyle.Render("("+t.Reason+")")
			}
			line += " " + dictStyle.Render("· "+t.DictTitle)
			fmt.Println(line)

			for sIdx, s := range t.Senses {
				if s.DefinitionTags != "" {
					fmt.Println("  " + tagStyle.Render("["+s.DefinitionTags+"]"))
				}
				for dIdx, def := range s.Definitions {
					prefix := fmt.Sprintf("  %d. ", sIdx+1)
					if dIdx > 0 {
						prefix = "     • "
					}
					fmt.Println(prefix + strings.ReplaceAll(def, "\n", "\n     "))
				}
				if showEx {
					for _, ex := range s.Examples {
						fmt.Println("     " + exStyle.Render("» "+ex))
					}
				}
			}
		}
	}

	if len(res.Kanji) > 0 && (kanjiOnly || len(res.Terms) == 0) {
		for idx, k := range res.Kanji {
			if idx > 0 {
				fmt.Println()
			}
			fmt.Printf("%s  %s\n", headStyle.Render(k.Character), dictStyle.Render(k.DictTitle))
			if k.Onyomi != "" {
				fmt.Println("  On:  " + readStyle.Render(k.Onyomi))
			}
			if k.Kunyomi != "" {
				fmt.Println("  Kun: " + readStyle.Render(k.Kunyomi))
			}
			if len(k.Meanings) > 0 {
				fmt.Println("  Meaning: " + strings.Join(k.Meanings, "; "))
			}
		}
	}
}

func init() {
	lookupCmd.Flags().BoolVar(&lookupJSON, "json", false, "Output results as JSON")
	lookupCmd.Flags().BoolVar(&lookupPlain, "plain", false, "Disable ANSI colors")
	lookupCmd.Flags().BoolVarP(&lookupClipboard, "clipboard", "c", false, "Read query text from clipboard (WSL/Wayland/X11/macOS/tmux)")
	lookupCmd.Flags().BoolVar(&lookupKanjiOnly, "kanji", false, "Show only Kanji breakdown")
	lookupCmd.Flags().BoolVar(&lookupExamples, "examples", false, "Include example sentences")
	rootCmd.AddCommand(lookupCmd)
}

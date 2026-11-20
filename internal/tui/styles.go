package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	StatusBar     lipgloss.Style
	StatusBadge   lipgloss.Style
	HelpBar       lipgloss.Style
	PopupBox         lipgloss.Style
	PopupBoxInactive lipgloss.Style
	PopupHeader      lipgloss.Style
	DepthBadge       lipgloss.Style
	TabActive     lipgloss.Style
	TabInactive   lipgloss.Style
	TermHeadword  lipgloss.Style
	TermReading   lipgloss.Style
	TermDictBadge lipgloss.Style
	TermTag       lipgloss.Style
	SenseNum      lipgloss.Style
	ExampleText   lipgloss.Style
	MutedText     lipgloss.Style
	KanjiChar     lipgloss.Style
	SearchPrompt  lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1),
		StatusBadge: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true).
			Padding(0, 1),
		HelpBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1),
		PopupBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1),
		PopupBoxInactive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Background(lipgloss.Color("234")).
			Foreground(lipgloss.Color("248")).
			Padding(0, 1),
		PopupHeader: lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")),
		DepthBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("222")).
			Bold(true),
		TabActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Bold(true).
			Padding(0, 1),
		TabInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1),
		TermHeadword: lipgloss.NewStyle().
			Foreground(lipgloss.Color("222")).
			Bold(true),
		TermReading: lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")),
		TermDictBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),
		TermTag: lipgloss.NewStyle().
			Foreground(lipgloss.Color("179")),
		SenseNum: lipgloss.NewStyle().
			Foreground(lipgloss.Color("109")).
			Bold(true),
		ExampleText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),
		MutedText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")),
		KanjiChar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("215")).
			Bold(true),
		SearchPrompt: lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("229")).
			Padding(0, 1),
	}
}

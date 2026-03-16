package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	ActiveBorder   lipgloss.Style
	InactiveBorder lipgloss.Style
	SelectedItem   lipgloss.Style
	NormalItem     lipgloss.Style
	StatusBar      lipgloss.Style
	Title          lipgloss.Style
	StatusMode     lipgloss.Style
	StatusHints    lipgloss.Style
	Timestamp      lipgloss.Style
	SessionName    lipgloss.Style
	SessionPreview lipgloss.Style
	RunningIcon    lipgloss.Style
	StoppedIcon    lipgloss.Style
	DialogBorder   lipgloss.Style
	DialogTitle    lipgloss.Style
	AccentBar      lipgloss.Style
}

func DefaultStyles() Styles {
	t := DarkTheme
	return Styles{
		ActiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Primary)),
		InactiveBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.InactiveBorder)),
		SelectedItem: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Secondary)).
			Foreground(lipgloss.Color(t.Text)),
		NormalItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextSecondary)),
		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Surface)).
			Foreground(lipgloss.Color(t.TextMuted)).
			Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Primary)),
		StatusMode: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Primary)).
			Background(lipgloss.Color(t.Surface)).
			Padding(0, 1),
		StatusHints: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextDim)).
			Background(lipgloss.Color(t.Surface)).
			Padding(0, 1),
		Timestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextDim)),
		SessionName: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Text)),
		SessionPreview: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextMuted)),
		RunningIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Success)),
		StoppedIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.TextDim)),
		DialogBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Primary)).
			Padding(1, 2),
		DialogTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(t.Text)).
			MarginBottom(1),
		AccentBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Primary)),
	}
}

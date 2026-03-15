package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D9FF")).
			Padding(0, 1)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			Padding(1, 2).
			Width(40)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginBottom(1)

	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Width(12)

	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500"))

	PendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Padding(1, 0)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00D9FF"))

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3C3C3C"))
)

func GetStatusStyle(connected bool) lipgloss.Style {
	if connected {
		return SuccessStyle
	}
	return ErrorStyle
}

func GetBarColor(percent float64) lipgloss.Color {
	switch {
	case percent < 50:
		return lipgloss.Color("#00FF00")
	case percent < 80:
		return lipgloss.Color("#FFA500")
	default:
		return lipgloss.Color("#FF0000")
	}
}

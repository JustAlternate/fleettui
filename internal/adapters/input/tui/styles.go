package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorSuccess  = lipgloss.Color("#3BCEAC")
	ColorWarning  = lipgloss.Color("#FFBE0B")
	ColorCritical = lipgloss.Color("#FF6B6B")
	ColorPrimary  = lipgloss.Color("#E8E8E8")
	ColorMuted    = lipgloss.Color("#888888")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess).
			MarginRight(1)

	HeaderStyle = lipgloss.NewStyle().
			Padding(1, 1)

	StatsStyle = lipgloss.NewStyle()

	StatsHealthyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSuccess)

	StatsLabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			Padding(1, 2).
			Width(40)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(12)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	CriticalStyle = lipgloss.NewStyle().
			Foreground(ColorCritical)

	PendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Padding(1, 0)

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#2A2A2A"))

	// Table view styles

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#3C3C3C"))

	TableRowStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Background(lipgloss.Color("#2A2A2A")).
				Bold(true)

	// Search bar styles

	SearchBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)

	SearchPrefixStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	SearchTextStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	SearchPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	SearchFilterInfoStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Panel bar styles

	PanelBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 1)

	PanelBarNameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSuccess)

	PanelBarIPStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	PanelFrameStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3C3C3C")).
			Padding(0, 1)

	PanelHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

// Column width constants — shared by header and row renderers for exact alignment.
const (
	ColWidthName    = 16
	ColWidthIP      = 16
	ColWidthStatus  = 12
	ColWidthCPU     = 8
	ColWidthRAM     = 8
	ColWidthNetIn   = 10
	ColWidthNetOut  = 10
	ColWidthUptime  = 10
	ColWidthSystemd = 14
)

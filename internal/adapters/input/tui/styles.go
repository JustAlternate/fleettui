package tui

import (
	"encoding/hex"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/justalternate/fleetui/internal/domain"
)

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

func GetStatusStyle(node *domain.Node) lipgloss.Style {
	if node == nil {
		return PendingStyle
	}
	if node.IsPending() {
		return PendingStyle
	}
	if node.IsAvailable() {
		return SuccessStyle
	}
	return CriticalStyle
}

func GetGradientColor(percent float64) lipgloss.Color {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	if percent <= 40 {
		return ColorSuccess
	}

	if percent <= 70 {
		t := (percent - 40) / 30
		return interpolateColor(ColorSuccess, ColorWarning, t)
	}

	t := (percent - 70) / 30
	return interpolateColor(ColorWarning, ColorCritical, t)
}

func interpolateColor(c1, c2 lipgloss.Color, t float64) lipgloss.Color {
	r1, g1, b1 := hexToRGB(string(c1))
	r2, g2, b2 := hexToRGB(string(c2))

	r := uint8(float64(r1) + (float64(r2)-float64(r1))*t)
	g := uint8(float64(g1) + (float64(g2)-float64(g1))*t)
	b := uint8(float64(b1) + (float64(b2)-float64(b1))*t)

	return lipgloss.Color(rgbToHex(r, g, b))
}

func hexToRGB(hexColor string) (uint8, uint8, uint8) {
	if len(hexColor) != 7 || hexColor[0] != '#' {
		return 0, 0, 0
	}

	bytes, err := hex.DecodeString(hexColor[1:])
	if err != nil || len(bytes) != 3 {
		return 0, 0, 0
	}

	return bytes[0], bytes[1], bytes[2]
}

func rgbToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func GetAnimatedDot(frame int, node *domain.Node) string {
	if node == nil {
		return "●"
	}

	if node.IsPending() {
		dots := []string{"◐", "◓", "◑", "◒"}
		return dots[frame%4]
	}

	if !node.IsAvailable() {
		return "○"
	}

	return "●"
}

func GetCardStyle(node *domain.Node) lipgloss.Style {
	borderColor := lipgloss.Color("#3C3C3C")
	if node != nil && !node.IsAvailable() && !node.IsPending() {
		borderColor = lipgloss.Color("#5C3C3C")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(40)
}

package tui

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justalternate/fleettui/internal/domain"
)

// ---------------------------------------------------------------------------
// Status & animation helpers
// ---------------------------------------------------------------------------

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

func GetCardStyle(node *domain.Node, selected bool) lipgloss.Style {
	borderColor := lipgloss.Color("#3C3C3C")
	if selected {
		borderColor = ColorSuccess
	} else if node != nil && !node.IsAvailable() && !node.IsPending() {
		borderColor = lipgloss.Color("#5C3C3C")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(40)
}

// ---------------------------------------------------------------------------
// Color helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d interface{}) string {
	if d == nil {
		return "N/A"
	}

	duration, ok := d.(interface{ Hours() float64 })
	if !ok {
		return "N/A"
	}

	hours := int(duration.Hours())
	days := hours / 24
	hours = hours % 24

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}

func formatNetworkRate(rateMBps float64) string {
	abs := rateMBps
	if abs < 0 {
		abs = -abs
	}
	bps := abs * 1024 * 1024

	switch {
	case bps < 1024:
		return fmt.Sprintf("%.0f B/s", bps)
	case bps < 1024*1024:
		return fmt.Sprintf("%.0f KB/s", bps/1024)
	case bps < 1024*1024*1024:
		return fmt.Sprintf("%.0f MB/s", bps/(1024*1024))
	default:
		return fmt.Sprintf("%.0f GB/s", bps/(1024*1024*1024))
	}
}

func getNetworkColor(rateMBps float64) lipgloss.Color {
	abs := rateMBps
	if abs < 0 {
		abs = -abs
	}
	bps := abs * 1024 * 1024

	switch {
	case bps < 1024:
		return ColorSuccess
	case bps < 1024*1024:
		t := bps / (1024 * 1024)
		return interpolateColor(ColorSuccess, ColorWarning, t)
	case bps < 1024*1024*1024:
		t := bps / (1024 * 1024 * 1024)
		return interpolateColor(ColorWarning, ColorCritical, t)
	default:
		return ColorCritical
	}
}

func renderProgressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	var filledBar string
	for i := 0; i < filled; i++ {
		segmentPercent := float64(i+1) / float64(width) * 100
		color := GetGradientColor(segmentPercent)
		filledBar += lipgloss.NewStyle().Foreground(color).Render("█")
	}
	emptyBar := ProgressBarEmptyStyle.Render(strings.Repeat("░", empty))

	return filledBar + emptyBar
}

// ---------------------------------------------------------------------------
// SSH helpers
// ---------------------------------------------------------------------------

// parseHostPort splits an address like "host:2201" into host and port.
// If there is no port suffix, port returns "".
func parseHostPort(addr string) (host, port string) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return addr, ""
	}
	// Check if what follows the colon is all digits (a port number).
	p := addr[idx+1:]
	for _, c := range p {
		if c < '0' || c > '9' {
			return addr, ""
		}
	}
	return addr[:idx], p
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"fleettui/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderView() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var sections []string

	title := TitleStyle.Render("FleetTUI")
	stats := StatsStyle.Render(m.renderStats())
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, stats)
	sections = append(sections, HeaderStyle.Render(header))

	sections = append(sections, "\n")
	sections = append(sections, m.viewport.View())

	help := HelpStyle.Render("[q] Quit • [r] Refresh • [j/k] Scroll")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderCardsContent() string {
	if len(m.nodes) == 0 {
		return "\n  No hosts configured.\n  Add hosts to ~/.config/fleettui/hosts.yaml\n"
	}

	var cards []string
	for _, node := range m.nodes {
		card := m.renderNodeCard(node)
		cards = append(cards, card)
	}

	columns := m.getColumns()

	var rows []string
	for i := 0; i < len(cards); i += columns {
		end := i + columns
		if end > len(cards) {
			end = len(cards)
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m *Model) renderStats() string {
	total := len(m.nodes)
	healthy := 0
	offline := 0
	for _, node := range m.nodes {
		if node.IsAvailable() {
			healthy++
		} else if !node.IsPending() {
			offline++
		}
	}

	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render("│")

	healthyStr := fmt.Sprintf("%d/%d healthy", healthy, total)
	if healthy == total {
		healthyStr = fmt.Sprintf("%d healthy", total)
	}

	healthyStyled := StatsHealthyStyle.Render(healthyStr)
	intervalStr := fmt.Sprintf("%ds", m.config.RefreshRate/time.Second)
	intervalStyled := StatsLabelStyle.Render(intervalStr)

	if offline > 0 {
		offlineStyled := CriticalStyle.Render(fmt.Sprintf("%d offline", offline))
		return fmt.Sprintf(" %s %s %s %s %s ", separator, healthyStyled, separator, offlineStyled, intervalStyled)
	}

	return fmt.Sprintf(" %s %s %s ", separator, healthyStyled, intervalStyled)
}

func (m *Model) renderCards() string {
	return m.renderCardsContent()
}

func (m *Model) renderNodeCard(node *domain.Node) string {
	var lines []string

	statusDot := GetAnimatedDot(m.animationFrame, node)
	statusStyle := GetStatusStyle(node)

	title := CardTitleStyle.Render(fmt.Sprintf("%s %s", statusStyle.Render(statusDot), node.Name))
	lines = append(lines, title)

	if m.config.IsMetricEnabled(domain.MetricOS) && node.OSInfo != "" {
		lines = append(lines, m.renderRow("OS:", truncateString(node.OSInfo, 28)))
	}

	lines = append(lines, m.renderRow("IP:", node.IP))

	if node.IsPending() {
		lines = append(lines, m.renderRow("Status:", PendingStyle.Render("Connecting...")))
	} else if !node.IsAvailable() {
		if node.Error != "" {
			lines = append(lines, m.renderRow("Status:", CriticalStyle.Render("Error")))
			lines = append(lines, m.renderRow("Error:", truncateString(node.Error, 25)))
		} else {
			lines = append(lines, m.renderRow("Status:", CriticalStyle.Render("Offline")))
		}
	} else {
		lines = append(lines, m.renderRow("Status:", SuccessStyle.Render("Online")))

		if m.config.IsMetricEnabled(domain.MetricCPU) {
			lines = append(lines, m.renderProgressRow("CPU:", node.Metrics.CPU.UsagePercent))
		}

		if m.config.IsMetricEnabled(domain.MetricRAM) {
			lines = append(lines, m.renderProgressRow("RAM:", node.Metrics.RAM.UsagePercent))
		}

		if m.config.IsMetricEnabled(domain.MetricNetwork) {
			netIn := fmt.Sprintf("↓ %.2f MB/s", node.Metrics.Network.InRateMBps)
			netOut := fmt.Sprintf("↑ %.2f MB/s", node.Metrics.Network.OutRateMBps)
			lines = append(lines, m.renderRow("Network:", fmt.Sprintf("%s  %s", netIn, netOut)))
		}

		if m.config.IsMetricEnabled(domain.MetricUptime) {
			lines = append(lines, m.renderRow("Uptime:", formatDuration(node.Metrics.Uptime)))
		}

		if m.config.IsMetricEnabled(domain.MetricSystemd) {
			if node.HasFailedUnits() {
				systemdStr := fmt.Sprintf("%d failed", node.Metrics.Systemd.FailedCount)
				lines = append(lines, m.renderRow("Systemd:", WarningStyle.Render(systemdStr+" ⚠")))
			} else {
				systemdStr := fmt.Sprintf("OK (%d units)", node.Metrics.Systemd.TotalCount)
				lines = append(lines, m.renderRow("Systemd:", SuccessStyle.Render(systemdStr)))
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	cardStyle := GetCardStyle(node)
	return cardStyle.Render(content)
}

func (m *Model) renderRow(label, value string) string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		LabelStyle.Render(label),
		ValueStyle.Render(value),
	)
}

func (m *Model) renderProgressRow(label string, percent float64) string {
	bar := renderProgressBar(percent, 15)
	value := fmt.Sprintf("%.1f%% %s", percent, bar)
	return m.renderRow(label, value)
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

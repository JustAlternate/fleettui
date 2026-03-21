package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/justalternate/fleetui/internal/domain"
)

func (m *Model) renderCardsContent() string {
	displayed := m.getDisplayedNodes()

	if len(displayed) == 0 {
		if m.searchText != "" {
			return "\n  No matches for \"" + m.searchText + "\"\n"
		}
		return "\n  No hosts configured.\n  Add hosts to ~/.config/fleettui/hosts.yaml\n"
	}

	var cards []string
	for i, node := range displayed {
		card := m.renderNodeCard(node, i == m.cursor)
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

func (m *Model) renderNodeCard(node *domain.Node, selected bool) string {
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
		lines = append(lines, m.renderRow("Status:", PendingStyle.Render("Pending...")))
	} else if !node.IsAvailable() {
		if node.Error != "" {
			lines = append(lines, m.renderRow("Status:", CriticalStyle.Render("Error")))
			errorStyle := lipgloss.NewStyle().Foreground(ColorCritical).Width(24)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
				LabelStyle.Render("Error:"),
				errorStyle.Render(node.Error)))
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
			netIn := fmt.Sprintf("↓ %s", formatNetworkRate(node.Metrics.Network.InRateMBps))
			netOut := fmt.Sprintf("↑ %s", formatNetworkRate(node.Metrics.Network.OutRateMBps))
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
	cardStyle := GetCardStyle(node, selected)
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

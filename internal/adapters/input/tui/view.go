package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justalternate/fleetui/internal/domain"
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

	switch m.viewMode {
	case ViewTable:
		sections = append(sections, m.renderTableHeader())
		sections = append(sections, m.tableViewport.View())
	default:
		sections = append(sections, m.viewport.View())
	}

	help := HelpStyle.Render(m.renderHelp())
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderHelp() string {
	base := "[q] Quit • [r] Refresh • [tab] Switch view"
	switch m.viewMode {
	case ViewTable:
		return base + " • [j/k] Navigate • [g/G] Top/Bottom"
	default:
		return base + " • [j/k] Scroll • [g/G] Top/Bottom"
	}
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

	return fmt.Sprintf("%s %s ", separator, healthyStyled)
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

// ---------------------------------------------------------------------------
// Table view
// ---------------------------------------------------------------------------

// tableCell renders a plain fixed-width cell with muted foreground.
// Used only for the pinned header row (no background concern there).
func tableCell(s string, width int) string {
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(truncateString(s, width))
}

// renderCell composes a cell of exactly `width` visible characters.
// The value string may already contain ANSI escape codes (pre-styled).
// Any trailing padding spaces are emitted with the row background so there
// are no "gaps" in the highlight band.
func renderCell(styledValue string, width int, bg lipgloss.Style) string {
	visible := lipgloss.Width(styledValue)
	if visible >= width {
		return styledValue
	}
	pad := bg.Render(strings.Repeat(" ", width-visible))
	return styledValue + pad
}

// renderTableHeader returns the pinned column-header row for the table view.
// It is rendered outside the viewport so it stays fixed while rows scroll.
func (m *Model) renderTableHeader() string {
	cols := []string{
		tableCell("NAME", ColWidthName),
		tableCell("IP", ColWidthIP),
		tableCell("STATUS", ColWidthStatus),
	}

	if m.config.IsMetricEnabled(domain.MetricCPU) {
		cols = append(cols, tableCell("CPU%", ColWidthCPU))
	}
	if m.config.IsMetricEnabled(domain.MetricRAM) {
		cols = append(cols, tableCell("RAM%", ColWidthRAM))
	}
	if m.config.IsMetricEnabled(domain.MetricNetwork) {
		cols = append(cols, tableCell("NET↓ MB/s", ColWidthNetIn))
		cols = append(cols, tableCell("NET↑ MB/s", ColWidthNetOut))
	}
	if m.config.IsMetricEnabled(domain.MetricUptime) {
		cols = append(cols, tableCell("UPTIME", ColWidthUptime))
	}
	if m.config.IsMetricEnabled(domain.MetricSystemd) {
		cols = append(cols, tableCell("SYSTEMD UNITS", ColWidthSystemd))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	return TableHeaderStyle.Width(m.width - 2).Render(row)
}

// renderTableContent builds the full scrollable body for the table view
// and returns it as a string to be set on the tableViewport.
func (m *Model) renderTableContent() string {
	if len(m.nodes) == 0 {
		return "\n  No hosts configured.\n  Add hosts to ~/.config/fleettui/hosts.yaml\n"
	}

	var rows []string
	for i, node := range m.nodes {
		rows = append(rows, m.renderTableRow(node, i, i == m.cursor))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderTableRow renders a single node as a table row.
//
// Every cell is built with renderCell which explicitly paints padding spaces
// with the row background style, ensuring no uncoloured gaps appear between
// columns on the selected row.
func (m *Model) renderTableRow(node *domain.Node, _ int, selected bool) string {
	rowStyle := TableRowStyle
	if selected {
		rowStyle = TableRowSelectedStyle
	}

	// bgStyle is a background-only style used to fill inter-column padding so
	// that the highlight is unbroken across the full row width.
	bgStyle := lipgloss.NewStyle().Background(rowStyle.GetBackground())

	// colFg returns a foreground+background style for a metric value cell.
	colFg := func(c lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(c).Background(rowStyle.GetBackground())
	}

	// -- Fixed cells (always shown) ------------------------------------------
	nameFg := colFg(ColorPrimary)
	if selected {
		nameFg = nameFg.Bold(true)
	}

	dot := GetAnimatedDot(m.animationFrame, node)
	statusFg := lipgloss.NewStyle().
		Foreground(GetStatusStyle(node).GetForeground()).
		Background(rowStyle.GetBackground())
	var statusStr string
	switch {
	case node.IsPending():
		statusStr = statusFg.Render(dot + " Pending")
	case !node.IsAvailable():
		statusStr = statusFg.Render(dot + " Offline")
	default:
		statusStr = statusFg.Render(dot + " Online")
	}

	line := renderCell(nameFg.Render(truncateString(node.Name, ColWidthName)), ColWidthName, bgStyle) +
		renderCell(colFg(ColorMuted).Render(truncateString(node.IP, ColWidthIP)), ColWidthIP, bgStyle) +
		renderCell(statusStr, ColWidthStatus, bgStyle)

	// -- Metric cells (configurable) -----------------------------------------
	dash := colFg(ColorMuted).Render("—")
	available := node.IsAvailable()

	if m.config.IsMetricEnabled(domain.MetricCPU) {
		if available {
			pct := node.Metrics.CPU.UsagePercent
			val := colFg(GetGradientColor(pct)).Render(fmt.Sprintf("%.1f%%", pct))
			line += renderCell(val, ColWidthCPU, bgStyle)
		} else {
			line += renderCell(dash, ColWidthCPU, bgStyle)
		}
	}

	if m.config.IsMetricEnabled(domain.MetricRAM) {
		if available {
			pct := node.Metrics.RAM.UsagePercent
			val := colFg(GetGradientColor(pct)).Render(fmt.Sprintf("%.1f%%", pct))
			line += renderCell(val, ColWidthRAM, bgStyle)
		} else {
			line += renderCell(dash, ColWidthRAM, bgStyle)
		}
	}

	if m.config.IsMetricEnabled(domain.MetricNetwork) {
		if available {
			inVal := colFg(ColorAccent).Render(fmt.Sprintf("%.2f", node.Metrics.Network.InRateMBps))
			outVal := colFg(ColorAccent).Render(fmt.Sprintf("%.2f", node.Metrics.Network.OutRateMBps))
			line += renderCell(inVal, ColWidthNetIn, bgStyle)
			line += renderCell(outVal, ColWidthNetOut, bgStyle)
		} else {
			line += renderCell(dash, ColWidthNetIn, bgStyle)
			line += renderCell(dash, ColWidthNetOut, bgStyle)
		}
	}

	if m.config.IsMetricEnabled(domain.MetricUptime) {
		if available {
			val := colFg(ColorPrimary).Render(formatDuration(node.Metrics.Uptime))
			line += renderCell(val, ColWidthUptime, bgStyle)
		} else {
			line += renderCell(dash, ColWidthUptime, bgStyle)
		}
	}

	if m.config.IsMetricEnabled(domain.MetricSystemd) {
		if available {
			var val string
			if node.HasFailedUnits() {
				val = colFg(ColorCritical).Render(fmt.Sprintf("Failed (%d) ⚠", node.Metrics.Systemd.FailedCount))
			} else {
				val = colFg(ColorSuccess).Render(fmt.Sprintf("OK (%d)", node.Metrics.Systemd.TotalCount))
			}
			line += renderCell(val, ColWidthSystemd, bgStyle)
		} else {
			line += renderCell(dash, ColWidthSystemd, bgStyle)
		}
	}

	// The outer Render fills any remaining terminal width with the background.
	return rowStyle.Width(m.width - 2).Render(line)
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justalternate/fleetui/internal/domain"
)

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
		cols = append(cols, tableCell("NET↓", ColWidthNetIn))
		cols = append(cols, tableCell("NET↑", ColWidthNetOut))
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
	displayed := m.getDisplayedNodes()

	if len(displayed) == 0 {
		if m.searchText != "" {
			return "\n  No matches for \"" + m.searchText + "\"\n"
		}
		return "\n  No hosts configured.\n  Add hosts to ~/.config/fleettui/hosts.yaml\n"
	}

	var rows []string
	for i, node := range displayed {
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
			inVal := colFg(getNetworkColor(node.Metrics.Network.InRateMBps)).Render(formatNetworkRate(node.Metrics.Network.InRateMBps))
			outVal := colFg(getNetworkColor(node.Metrics.Network.OutRateMBps)).Render(formatNetworkRate(node.Metrics.Network.OutRateMBps))
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

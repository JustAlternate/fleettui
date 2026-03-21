package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderView() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// SSH panel mode — show thin bar + terminal.
	if m.panelMode {
		return m.renderPanelView()
	}

	// Normal mode.
	var sections []string

	title := TitleStyle.Render("FleetTUI")
	stats := StatsStyle.Render(m.renderStats())
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, stats)
	sections = append(sections, HeaderStyle.Render(header))

	// Search bar — visible when in search mode or when a filter is active.
	if m.searchMode || m.searchText != "" {
		sections = append(sections, m.renderSearchBar())
		// Extra spacing below search bar in table view.
		if m.viewMode == ViewTable {
			sections = append(sections, "")
		}
	}

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

func (m *Model) renderPanelView() string {
	bar := m.renderPanelBar()
	term := m.termViewport.View()
	help := PanelHelpStyle.Render("[ctrl+q] Close panel")

	panelContent := lipgloss.JoinVertical(lipgloss.Left, bar, term, help)
	framed := PanelFrameStyle.Render(panelContent)

	wrapper := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Align(lipgloss.Center, lipgloss.Center)

	return wrapper.Render(framed)
}

func (m *Model) renderPanelBar() string {
	if m.panelNode == nil {
		return ""
	}

	var status string
	var statusColor lipgloss.Color

	switch {
	case m.sshConnecting:
		status = "Connecting..."
		statusColor = ColorWarning
	case m.sshError != "":
		status = m.sshError
		statusColor = ColorCritical
	default:
		status = "Connected"
		statusColor = ColorSuccess
	}

	name := PanelBarNameStyle.Render(m.panelNode.Name)
	ip := PanelBarIPStyle.Render(m.panelNode.IP)
	statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)

	content := lipgloss.JoinHorizontal(lipgloss.Top, name, " ", ip, " ", statusStyled)
	barWidth := m.termViewport.Width
	if barWidth <= 0 {
		barWidth = m.width - 8
	}
	if barWidth < 10 {
		barWidth = 10
	}
	return PanelBarStyle.Width(barWidth).Render(content)
}

func (m *Model) renderHelp() string {
	base := "[q] Quit • [r] Refresh • [s] SSH • [tab] Switch view • [/] Search"
	switch m.viewMode {
	case ViewTable:
		return base + " • [j/k] Navigate • [g/G] Top/Bottom"
	default:
		return base + " • [j/k] Up/Down • [h/l] Left/Right • [g/G] Top/Bottom"
	}
}

func (m *Model) renderStats() string {
	total := len(m.nodes)
	healthy := 0
	for _, node := range m.nodes {
		if node.IsAvailable() {
			healthy++
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

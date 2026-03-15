package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type RefreshMsg struct{}

type NodesUpdatedMsg struct{}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.cancel()
		return m, tea.Quit
	case "r":
		go func() {
			m.collector.CollectAll(m.ctx)
		}()
		return m, nil
	}
	return m, nil
}

func (m *Model) handleWindowSize(width, height int) (tea.Model, tea.Cmd) {
	m.width = width
	m.height = height
	return m, nil
}

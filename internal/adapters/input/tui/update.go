package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type animationTickMsg time.Time

func tickCmd(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func animationTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		case "r":
			now := time.Now().UnixMilli()
			if now-m.lastRefresh >= 1000 {
				m.lastRefresh = now
				go m.collector.CollectAll(m.ctx)
			}
			return m, nil
		case "up", "k":
			m.viewport.LineUp(1)
			return m, nil
		case "down", "j":
			m.viewport.LineDown(1)
			return m, nil
		}

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.viewport.LineUp(3)
			return m, nil
		case tea.MouseWheelDown:
			m.viewport.LineDown(3)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 6
		m.updateViewportContent()

	case tickMsg:
		m.nodes = m.collector.GetNodes()
		m.updateViewportContent()
		return m, tickCmd(m.config.RefreshRate)

	case animationTickMsg:
		m.animationFrame = (m.animationFrame + 1) % 4
		m.updateViewportContent()
		return m, animationTickCmd()
	}

	return m, nil
}

func (m *Model) updateViewportContent() {
	content := m.renderCardsContent()
	m.viewport.SetContent(content)
}

func (m *Model) getColumns() int {
	columns := 3
	if m.width < 130 {
		columns = 2
	}
	if m.width < 90 {
		columns = 1
	}
	return columns
}

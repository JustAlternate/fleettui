package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// tickMsg is sent when it's time to refresh the display
type tickMsg time.Time

// tickCmd creates a command that sends a tickMsg after the specified duration
func tickCmd(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles all messages and updates the model state
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		case "r":
			go m.collector.CollectAll(m.ctx)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.nodes = m.collector.GetNodes()
		return m, tickCmd(m.config.RefreshRate)
	}

	return m, nil
}

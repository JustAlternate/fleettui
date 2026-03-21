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
	// When in search mode, route most keys to the text input.
	if m.searchMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			key := msg.String()
			switch key {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.recalcViewportHeights()
				m.updateViewportContent()
				m.updateTableContent()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.recalcViewportHeights()
				m.updateViewportContent()
				m.updateTableContent()
				return m, nil
			}

			// Forward to text input.
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchText = m.searchInput.Value()
			m.applyFilter()
			m.updateViewportContent()
			m.updateTableContent()
			return m, cmd
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Accumulate vim count prefix (digits, but not "0" alone as first digit
		// since bare "0" has no motion meaning here — we simply skip it).
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			m.countPrefix = m.countPrefix*10 + int(key[0]-'0')
			return m, nil
		}
		// Allow "0" only as a subsequent digit in a multi-digit count.
		if key == "0" && m.countPrefix > 0 {
			m.countPrefix = m.countPrefix * 10
			return m, nil
		}

		// Consume the accumulated count (default 1 when none was typed).
		count := m.countPrefix
		if count == 0 {
			count = 1
		}
		m.countPrefix = 0

		switch key {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit

		case "esc":
			// Clear filter if one is active.
			if m.searchText != "" {
				m.searchText = ""
				m.filteredNodes = nil
				m.searchInput.SetValue("")
				m.cursor = 0
				m.recalcViewportHeights()
				m.updateViewportContent()
				m.updateTableContent()
			}
			return m, nil

		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			m.recalcViewportHeights()
			return m, nil

		case "tab":
			// Cycle through available views.
			m.viewMode = (m.viewMode + 1) % 2 // ViewCards(0) <-> ViewTable(1)
			if m.searchMode || m.searchText != "" {
				m.recalcViewportHeights()
			}
			return m, nil

		case "r":
			now := time.Now().UnixMilli()
			if now-m.lastRefresh >= 1000 {
				m.lastRefresh = now
				go m.collector.CollectAll(m.ctx)
			}
			return m, nil

		case "up", "k":
			switch m.viewMode {
			case ViewTable:
				m.cursor -= count
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updateTableContent()
				m.ensureCursorVisible()
			default:
				m.viewport.ScrollUp(count)
			}
			return m, nil

		case "down", "j":
			displayed := m.getDisplayedNodes()
			switch m.viewMode {
			case ViewTable:
				m.cursor += count
				if m.cursor > len(displayed)-1 {
					m.cursor = len(displayed) - 1
				}
				m.updateTableContent()
				m.ensureCursorVisible()
			default:
				m.viewport.ScrollDown(count)
			}
			return m, nil

		case "g":
			switch m.viewMode {
			case ViewTable:
				m.cursor = 0
				m.updateTableContent()
				m.tableViewport.SetYOffset(0)
			default:
				m.viewport.SetYOffset(0)
			}
			return m, nil

		case "G":
			displayed := m.getDisplayedNodes()
			switch m.viewMode {
			case ViewTable:
				if len(displayed) > 0 {
					m.cursor = len(displayed) - 1
					m.updateTableContent()
					m.ensureCursorVisible()
				}
			default:
				m.viewport.GotoBottom()
			}
			return m, nil
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				switch m.viewMode {
				case ViewTable:
					m.tableViewport.ScrollUp(3)
				default:
					m.viewport.ScrollUp(3)
				}
				return m, nil
			case tea.MouseButtonWheelDown:
				switch m.viewMode {
				case ViewTable:
					m.tableViewport.ScrollDown(3)
				default:
					m.viewport.ScrollDown(3)
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcViewportHeights()
		m.updateViewportContent()
		m.updateTableContent()

	case tickMsg:
		m.nodes = m.collector.GetNodes()
		// Re-apply filter after node refresh.
		if m.searchText != "" {
			m.applyFilter()
		}
		// Clamp cursor in case the node list shrank.
		displayed := m.getDisplayedNodes()
		if m.cursor >= len(displayed) && len(displayed) > 0 {
			m.cursor = len(displayed) - 1
		}
		m.updateViewportContent()
		m.updateTableContent()
		return m, tickCmd(m.config.RefreshRate)

	case animationTickMsg:
		m.animationFrame = (m.animationFrame + 1) % 4
		m.updateViewportContent()
		m.updateTableContent()
		return m, animationTickCmd()
	}

	return m, nil
}

func (m *Model) updateViewportContent() {
	content := m.renderCardsContent()
	m.viewport.SetContent(content)
}

func (m *Model) updateTableContent() {
	content := m.renderTableContent()
	m.tableViewport.SetContent(content)
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

// ensureCursorVisible scrolls the tableViewport the minimum amount needed so
// that the cursor row is always visible. Each row is exactly 1 terminal line.
func (m *Model) ensureCursorVisible() {
	top := m.tableViewport.YOffset
	bottom := top + m.tableViewport.Height - 1

	if m.cursor < top {
		// Cursor went above the visible window — scroll up to it.
		m.tableViewport.SetYOffset(m.cursor)
	} else if m.cursor > bottom {
		// Cursor went below the visible window — scroll down to it.
		m.tableViewport.SetYOffset(m.cursor - m.tableViewport.Height + 1)
	}
}

// recalcViewportHeights recomputes viewport dimensions based on terminal size
// and whether the search bar is currently visible.
func (m *Model) recalcViewportHeights() {
	contentHeight := m.height - 7
	tableContentHeight := contentHeight - 1
	if m.searchMode || m.searchText != "" {
		contentHeight--
		tableContentHeight--
		// Extra spacer line below search bar in table view.
		if m.viewMode == ViewTable {
			contentHeight--
			tableContentHeight--
		}
	}

	m.viewport.Width = m.width
	m.viewport.Height = contentHeight

	m.tableViewport.Width = m.width
	m.tableViewport.Height = tableContentHeight
}

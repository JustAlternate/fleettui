package tui

// cardHeight is the estimated vertical space a single card occupies in the
// viewport (border + padding + content lines).  Used to compute which grid
// row a cursor index falls on and whether the viewport needs scrolling.
const cardHeight = 10

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

// ensureCardCursorVisible scrolls the card viewport so the selected card is
// visible.  Each grid row is cardHeight lines tall.
func (m *Model) ensureCardCursorVisible() {
	columns := m.getColumns()
	if columns == 0 {
		return
	}
	rowIndex := m.cursor / columns
	cardsTop := rowIndex * cardHeight
	cardsBottom := cardsTop + cardHeight - 1

	vpTop := m.viewport.YOffset
	vpBottom := vpTop + m.viewport.Height - 1

	if cardsTop < vpTop {
		m.viewport.SetYOffset(cardsTop)
	} else if cardsBottom > vpBottom {
		m.viewport.SetYOffset(cardsBottom - m.viewport.Height + 1)
	}
}

// clampCursor ensures cursor stays within the displayed node list.
func (m *Model) clampCursor() {
	displayed := m.getDisplayedNodes()
	if len(displayed) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(displayed) {
		m.cursor = len(displayed) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) getColumns() int {
	columns := 4
	if m.width < 168 {
		columns = 3
	}
	if m.width < 126 {
		columns = 2
	}
	if m.width < 84 {
		columns = 1
	}
	return columns
}

func (m *Model) updateViewportContent() {
	content := m.renderCardsContent()
	m.viewport.SetContent(content)
}

func (m *Model) updateTableContent() {
	content := m.renderTableContent()
	m.tableViewport.SetContent(content)
}

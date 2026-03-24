package tui

import (
	"fmt"
	"image/color"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/justalternate/fleettui/internal/domain"
)

type tickMsg time.Time
type animationTickMsg time.Time
type panelRefreshMsg time.Time
type sshConnectedMsg struct {
	seq     int
	session *sshSession
}
type sshConnectFailedMsg struct {
	seq int
	err error
}
type sshSessionEndedMsg struct{ seq int }
type termChunk struct {
	seq    int
	data   []byte
	closed bool
}
type termOutputMsg struct{ chunk termChunk }
type logsConnectedMsg struct {
	seq     int
	session *logsStreamSession
}
type logsConnectFailedMsg struct {
	seq int
	err error
}
type logsChunk struct {
	seq    int
	data   []byte
	closed bool
}
type logsOutputMsg struct{ chunk logsChunk }

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

func panelRefreshCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return panelRefreshMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── SSH panel active: route all input to the terminal ──────────
	if m.panelMode {
		return m.updatePanel(msg)
	}

	// ── Search mode: route keys to text input ──────────────────────
	if m.searchMode {
		return m.updateSearch(msg)
	}

	// ── Normal mode ────────────────────────────────────────────────
	return m.updateNormal(msg)
}

// ── SSH panel mode ─────────────────────────────────────────────────

func (m *Model) updatePanel(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.panelKind == PanelLogs {
		return m.updateLogsPanel(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ctrl+Q closes the panel manually.
		if msg.Type == tea.KeyCtrlQ {
			m.closePanel()
			m.updateViewportContent()
			m.updateTableContent()
			return m, nil
		}
		if m.ssh == nil || m.emulator == nil {
			return m, nil
		}
		seq := keyMsgToBytes(msg)
		if len(seq) > 0 {
			_, _ = m.emulator.InputPipe().Write(seq)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w, h := m.panelDimensions()
		m.termViewport.Width = w
		m.termViewport.Height = h
		if m.emulator != nil {
			m.emulator.Resize(w, h)
		}
		if m.ssh != nil {
			_ = sendWindowChange(m.ssh.session, w, h)
		}
		m.updateTermContent()
		return m, nil

	case panelRefreshMsg:
		if !m.panelMode {
			return m, nil
		}
		m.updateTermContent()
		return m, panelRefreshCmd()

	case termOutputMsg:
		if msg.chunk.seq != m.panelSeq {
			return m, nil
		}
		if msg.chunk.closed {
			m.closePanel()
			m.updateViewportContent()
			m.updateTableContent()
			return m, nil
		}
		if len(msg.chunk.data) > 0 && m.emulator != nil {
			_, _ = m.emulator.Write(msg.chunk.data)
		}
		m.updateTermContent()
		return m, m.waitTermOutput(msg.chunk.seq)

	case sshConnectedMsg:
		if msg.seq != m.panelSeq {
			closeSSH(msg.session)
			return m, nil
		}
		m.ssh = msg.session
		m.sshConnecting = false
		m.panelStatus = "Connected"
		m.panelStatusFg = ColorSuccess
		m.termChan = make(chan termChunk, 64)
		m.termStop = make(chan struct{})
		w, h := m.panelDimensions()
		m.termViewport.Width = w
		m.termViewport.Height = h
		if m.emulator != nil {
			m.emulator.Focus()
		}
		m.updateTermContent()
		go m.readSSHOutput(msg.seq, msg.session, m.termChan, m.termStop)
		go m.readTerminalInput(msg.session, m.emulator, m.termStop)
		return m, m.waitTermOutput(msg.seq)

	case sshConnectFailedMsg:
		if msg.seq != m.panelSeq {
			return m, nil
		}
		m.sshConnecting = false
		m.sshError = fmt.Sprintf("Connection failed: %v", msg.err)
		m.panelStatus = m.sshError
		m.panelStatusFg = ColorCritical
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return sshSessionEndedMsg{seq: msg.seq}
		})

	case sshSessionEndedMsg:
		if msg.seq != m.panelSeq {
			return m, nil
		}
		m.closePanel()
		m.updateViewportContent()
		m.updateTableContent()
		return m, nil
	}
	return m, nil
}

// ── Search mode ────────────────────────────────────────────────────

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
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

// ── Normal mode ────────────────────────────────────────────────────

func (m *Model) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcViewportHeights()
		m.clampCursor()
		m.updateViewportContent()
		m.updateTableContent()

	case tickMsg:
		m.nodes = m.collector.GetNodes()
		if m.searchText != "" {
			m.applyFilter()
		}
		m.clampCursor()
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

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Accumulate vim count prefix.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		m.countPrefix = m.countPrefix*10 + int(key[0]-'0')
		return m, nil
	}
	if key == "0" && m.countPrefix > 0 {
		m.countPrefix = m.countPrefix * 10
		return m, nil
	}

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
		m.viewMode = (m.viewMode + 1) % 2
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

	case "S":
		displayed := m.getDisplayedNodes()
		if len(displayed) == 0 || m.cursor >= len(displayed) {
			return m, nil
		}
		return m, m.openPanel(displayed[m.cursor])

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
			m.cursor -= count * m.getColumns()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateViewportContent()
			m.ensureCardCursorVisible()
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
			m.cursor += count * m.getColumns()
			if m.cursor > len(displayed)-1 {
				m.cursor = len(displayed) - 1
			}
			m.updateViewportContent()
			m.ensureCardCursorVisible()
		}
		return m, nil

	case "left", "h":
		if m.viewMode == ViewCards {
			m.cursor -= count
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.updateViewportContent()
			m.ensureCardCursorVisible()
		}
		return m, nil

	case "right", "l":
		if m.viewMode == ViewCards {
			displayed := m.getDisplayedNodes()
			m.cursor += count
			if m.cursor > len(displayed)-1 {
				m.cursor = len(displayed) - 1
			}
			m.updateViewportContent()
			m.ensureCardCursorVisible()
		}
		return m, nil

	case "L":
		displayed := m.getDisplayedNodes()
		if len(displayed) == 0 || m.cursor >= len(displayed) {
			return m, nil
		}
		return m, m.openLogsPanel(displayed[m.cursor])

	case "g":
		switch m.viewMode {
		case ViewTable:
			m.cursor = 0
			m.updateTableContent()
			m.tableViewport.SetYOffset(0)
		default:
			m.cursor = 0
			m.updateViewportContent()
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
			if len(displayed) > 0 {
				m.cursor = len(displayed) - 1
				m.updateViewportContent()
				m.ensureCardCursorVisible()
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
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
	return m, nil
}

// ── SSH panel lifecycle ────────────────────────────────────────────

func (m *Model) openPanel(node *domain.Node) tea.Cmd {
	m.panelSeq++
	m.panelMode = true
	m.panelKind = PanelSSH
	m.panelNode = node
	m.panelTitle = "SSH"
	m.panelHelp = "[ctrl+q] Close panel"
	m.panelStatus = "Connecting..."
	m.panelStatusFg = ColorWarning
	m.sshConnecting = true
	m.sshError = ""
	m.ssh = nil
	m.termChan = nil
	m.termStop = nil

	w, h := m.panelDimensions()
	m.termViewport.Width = w
	m.termViewport.Height = h
	m.emulator = vt.NewSafeEmulator(w, h)
	cursorColor := color.RGBA{R: 0x3B, G: 0xCE, B: 0xAC, A: 0xFF}
	m.emulator.SetDefaultCursorColor(cursorColor)
	m.emulator.SetCursorColor(cursorColor)
	m.emulator.Focus()
	m.updateTermContent()

	seq := m.panelSeq
	connectCmd := func() tea.Msg {
		sess, err := connectSSH(node, w, h)
		if err != nil {
			return sshConnectFailedMsg{seq: seq, err: err}
		}
		return sshConnectedMsg{seq: seq, session: sess}
	}

	return tea.Batch(connectCmd, panelRefreshCmd())
}

func (m *Model) openLogsPanel(node *domain.Node) tea.Cmd {
	m.panelSeq++
	m.panelMode = true
	m.panelKind = PanelLogs
	m.panelNode = node
	m.panelTitle = "Logs"
	m.panelHelp = "[ctrl+q] Close panel • [f] Follow • [w] Wrap • [c] Clear • [/] Filter • [g/G] Top/Bottom • [V] Select • [y] Yank"
	m.panelStatus = "Connecting..."
	m.panelStatusFg = ColorWarning
	m.sshConnecting = false
	m.sshError = ""
	m.ssh = nil
	m.termChan = nil
	m.termStop = nil
	m.logsSession = nil
	m.logsChan = nil
	m.logsStop = nil
	m.logsFollow = true
	m.logsFilterMode = false
	m.logsFilter = ""
	m.logsFilterInput.SetValue("")
	m.logsFilterInput.Blur()
	m.logsRawLines = nil
	m.logsViewLines = nil
	m.logsTail = ""
	m.logsMatchIndex = -1
	m.logsCursor = -1
	m.logsWrap = false
	m.logsCursorRow0 = -1
	m.logsCursorRow1 = -1
	m.logsSelectMode = false
	m.logsSelectAnchor = -1

	w, h := m.panelDimensions()
	m.termViewport.Width = w
	m.termViewport.Height = h
	m.termViewport.SetContent("Connecting to logs stream...")
	m.termViewport.SetYOffset(0)

	if m.emulator != nil {
		_ = m.emulator.Close()
	}
	m.emulator = nil

	seq := m.panelSeq
	connectCmd := func() tea.Msg {
		sess, err := connectLogsStream(node, "journalctl -f -n 200 --no-pager -o short-iso")
		if err != nil {
			return logsConnectFailedMsg{seq: seq, err: err}
		}
		return logsConnectedMsg{seq: seq, session: sess}
	}

	return tea.Batch(connectCmd, panelRefreshCmd())
}

func (m *Model) updateLogsPanel(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.logsFilterMode {
			if msg.Type == tea.KeyEsc {
				m.logsFilterMode = false
				m.logsFilterInput.Blur()
				m.logsFilterInput.SetValue("")
				m.logsFilter = ""
				m.applyLogsFilter()
				m.ensureLogsCursorInRange()
				m.updateLogsViewport()
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				m.logsFilterMode = false
				m.logsFilterInput.Blur()
				m.applyLogsFilter()
				m.ensureLogsCursorInRange()
				m.updateLogsViewport()
				return m, nil
			}

			var cmd tea.Cmd
			m.logsFilterInput, cmd = m.logsFilterInput.Update(msg)
			m.logsFilter = m.logsFilterInput.Value()
			m.applyLogsFilter()
			m.ensureLogsCursorInRange()
			m.updateLogsViewport()
			return m, cmd
		}

		if msg.Type == tea.KeyCtrlQ {
			m.closePanel()
			m.updateViewportContent()
			m.updateTableContent()
			return m, nil
		}
		if msg.Type == tea.KeyEsc && m.logsSelectMode {
			m.logsSelectMode = false
			m.logsSelectAnchor = -1
			if m.logsFollow {
				m.panelStatus = "Streaming"
				m.panelStatusFg = ColorSuccess
			} else {
				m.panelStatus = "Paused"
				m.panelStatusFg = ColorWarning
			}
			m.updateLogsViewport()
			return m, nil
		}
		if msg.Type == tea.KeyEsc && m.logsFilter != "" {
			m.logsFilter = ""
			m.logsFilterInput.SetValue("")
			m.applyLogsFilter()
			m.ensureLogsCursorInRange()
			m.updateLogsViewport()
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			m.pauseLogsFollow()
			m.moveLogsCursor(1)
			return m, nil
		case "k", "up":
			m.pauseLogsFollow()
			m.moveLogsCursor(-1)
			return m, nil
		case "g":
			if len(m.logsViewLines) == 0 {
				return m, nil
			}
			m.pauseLogsFollow()
			m.logsCursor = 0
			m.termViewport.SetYOffset(0)
			m.updateLogsViewport()
			return m, nil
		case "G":
			if len(m.logsViewLines) == 0 {
				return m, nil
			}
			m.pauseLogsFollow()
			m.logsCursor = len(m.logsViewLines) - 1
			m.ensureLogsCursorVisible()
			m.updateLogsViewport()
			return m, nil
		case "V":
			if len(m.logsViewLines) == 0 {
				return m, nil
			}
			if !m.logsSelectMode {
				m.logsSelectMode = true
				m.logsSelectAnchor = m.logsCursor
				if m.logsSelectAnchor < 0 {
					m.logsSelectAnchor = 0
				}
				m.panelStatus = "Paused • Selecting"
				m.panelStatusFg = ColorWarning
			} else {
				m.logsSelectMode = false
				m.logsSelectAnchor = -1
				if m.logsFollow {
					m.panelStatus = "Streaming"
					m.panelStatusFg = ColorSuccess
				} else {
					m.panelStatus = "Paused"
					m.panelStatusFg = ColorWarning
				}
			}
			m.updateLogsViewport()
			return m, nil
		case "y":
			if len(m.logsViewLines) == 0 {
				return m, nil
			}
			var content string
			if m.logsSelectMode {
				start, end := m.logsSelectionBounds()
				if start < 0 || end < start || end >= len(m.logsViewLines) {
					return m, nil
				}
				content = strings.Join(m.logsViewLines[start:end+1], "\n")
			} else {
				if m.logsCursor < 0 || m.logsCursor >= len(m.logsViewLines) {
					return m, nil
				}
				content = m.logsViewLines[m.logsCursor]
			}
			if err := copyTextToClipboard(content); err != nil {
				m.panelStatus = "Yank failed"
				m.panelStatusFg = ColorCritical
				return m, nil
			}
			if m.logsSelectMode {
				start, end := m.logsSelectionBounds()
				m.panelStatus = fmt.Sprintf("Paused • Yanked %d lines", end-start+1)
				m.panelStatusFg = ColorSuccess
				m.logsSelectMode = false
				m.logsSelectAnchor = -1
				return m, nil
			}
			if m.logsFollow {
				m.panelStatus = "Streaming • Yanked"
			} else {
				m.panelStatus = "Paused • Yanked"
			}
			m.panelStatusFg = ColorSuccess
			return m, nil
		case "f":
			m.logsFollow = !m.logsFollow
			if m.logsFollow {
				m.termViewport.GotoBottom()
				m.panelStatus = "Streaming"
				m.panelStatusFg = ColorSuccess
			} else {
				m.pauseLogsFollow()
			}
			return m, nil
		case "w":
			m.logsWrap = !m.logsWrap
			m.ensureLogsCursorInRange()
			m.updateLogsViewport()
			return m, nil
		case "c":
			m.logsRawLines = nil
			m.logsViewLines = nil
			m.logsTail = ""
			m.logsMatchIndex = -1
			m.logsCursor = -1
			m.logsCursorRow0 = -1
			m.logsCursorRow1 = -1
			m.logsSelectMode = false
			m.logsSelectAnchor = -1
			m.updateLogsViewport()
			return m, nil
		case "/":
			m.logsFilterMode = true
			m.logsFilterInput.SetValue(m.logsFilter)
			m.logsFilterInput.Focus()
			return m, nil
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.termViewport.ScrollUp(3)
				m.pauseLogsFollow()
				return m, nil
			case tea.MouseButtonWheelDown:
				m.termViewport.ScrollDown(3)
				m.pauseLogsFollow()
				return m, nil
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w, h := m.panelDimensions()
		m.termViewport.Width = w
		m.termViewport.Height = h
		m.updateLogsViewport()
		return m, nil

	case logsOutputMsg:
		if msg.chunk.seq != m.panelSeq {
			return m, nil
		}
		if msg.chunk.closed {
			if m.panelStatus != "Paused" {
				m.panelStatus = "Disconnected"
				m.panelStatusFg = ColorWarning
			}
			return m, nil
		}
		if len(msg.chunk.data) > 0 {
			m.appendLogsChunk(msg.chunk.data)
			m.applyLogsFilter()
			m.ensureLogsCursorInRange()
			m.updateLogsViewport()
		}
		return m, m.waitLogsOutput(msg.chunk.seq)

	case logsConnectedMsg:
		if msg.seq != m.panelSeq {
			closeLogsStream(msg.session)
			return m, nil
		}
		m.logsSession = msg.session
		m.logsChan = make(chan logsChunk, 128)
		m.logsStop = make(chan struct{})
		m.panelStatus = "Streaming"
		m.panelStatusFg = ColorSuccess
		m.ensureLogsCursorInRange()
		go m.readLogsOutput(msg.seq, msg.session, m.logsChan, m.logsStop)
		return m, m.waitLogsOutput(msg.seq)

	case logsConnectFailedMsg:
		if msg.seq != m.panelSeq {
			return m, nil
		}
		m.panelStatus = fmt.Sprintf("Connection failed: %v", msg.err)
		m.panelStatusFg = ColorCritical
		m.termViewport.SetContent(m.panelStatus)
		return m, nil

	case panelRefreshMsg:
		if !m.panelMode || m.panelKind != PanelLogs {
			return m, nil
		}
		m.updateLogsViewport()
		return m, panelRefreshCmd()
	}

	return m, nil
}

func (m *Model) closePanel() {
	if m.termStop != nil {
		close(m.termStop)
		m.termStop = nil
	}
	if m.logsStop != nil {
		close(m.logsStop)
		m.logsStop = nil
	}
	closeSSH(m.ssh)
	closeLogsStream(m.logsSession)
	if m.emulator != nil {
		m.emulator.Close()
	}
	m.termChan = nil
	m.logsChan = nil
	m.panelMode = false
	m.panelKind = PanelNone
	m.panelNode = nil
	m.panelTitle = ""
	m.panelStatus = ""
	m.panelStatusFg = ""
	m.panelHelp = ""
	m.ssh = nil
	m.logsSession = nil
	m.emulator = nil
	m.sshConnecting = false
	m.sshError = ""
	m.logsFollow = true
	m.logsFilterMode = false
	m.logsFilter = ""
	m.logsFilterInput.SetValue("")
	m.logsFilterInput.Blur()
	m.logsRawLines = nil
	m.logsViewLines = nil
	m.logsTail = ""
	m.logsMatchIndex = -1
	m.logsCursor = -1
	m.logsWrap = false
	m.logsCursorRow0 = -1
	m.logsCursorRow1 = -1
	m.logsSelectMode = false
	m.logsSelectAnchor = -1
}

func (m *Model) panelDimensions() (width, height int) {
	innerWidth := m.width - 10
	if innerWidth < 20 {
		innerWidth = m.width - 2
	}
	if innerWidth < 10 {
		innerWidth = 10
	}

	barHeight := lipgloss.Height(PanelBarStyle.Width(innerWidth).Render("x"))
	if barHeight < 1 {
		barHeight = 1
	}

	helpHeight := lipgloss.Height(PanelHelpStyle.Render("[ctrl+q] Close panel"))
	if helpHeight < 1 {
		helpHeight = 1
	}

	searchHeight := 0
	if m.panelKind == PanelLogs && (m.logsFilterMode || m.logsFilter != "") {
		searchHeight = lipgloss.Height(m.renderLogsSearchBar())
	}

	h := m.height - 8 - barHeight - helpHeight - searchHeight
	if m.height < 16 {
		h = m.height - 4 - barHeight - helpHeight - searchHeight
	}
	if h < 5 {
		h = 5
	}
	w := innerWidth
	return w, h
}

func (m *Model) updateTermContent() {
	if m.emulator == nil {
		return
	}
	content := m.renderTermWithCursor()
	m.termViewport.SetContent(content)
	if m.emulator.IsAltScreen() {
		m.termViewport.SetYOffset(0)
		return
	}
	m.termViewport.GotoBottom()
}

func (m *Model) renderTermWithCursor() string {
	if m.emulator == nil {
		return ""
	}

	w := m.emulator.Width()
	h := m.emulator.Height()
	if w <= 0 || h <= 0 {
		return m.emulator.Render()
	}

	buf := uv.NewBuffer(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := m.emulator.CellAt(x, y)
			if cell == nil {
				continue
			}
			buf.SetCell(x, y, cell.Clone())
		}
	}

	cpos := m.emulator.CursorPosition()
	if cpos.X < 0 || cpos.Y < 0 || cpos.X >= w || cpos.Y >= h {
		return buf.Render()
	}

	cursorCell := buf.CellAt(cpos.X, cpos.Y)
	if cursorCell == nil {
		cursorCell = uv.NewCell(m.emulator.WidthMethod(), " ")
	}
	cursorCell = cursorCell.Clone()
	cursorCell.Style.Attrs |= uv.AttrReverse
	buf.SetCell(cpos.X, cpos.Y, cursorCell)

	return buf.Render()
}

// ── Key → bytes conversion ─────────────────────────────────────────

// keyMsgToBytes converts a tea.KeyMsg to the raw bytes that should be sent
// to an SSH session.
func keyMsgToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyEnter:
		return []byte("\r")
	case tea.KeyBackspace:
		return []byte{0x7f}
	case tea.KeyTab:
		return []byte("\t")
	case tea.KeyEsc:
		return []byte{0x1b}
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyCtrlA:
		return []byte{0x01}
	case tea.KeyCtrlB:
		return []byte{0x02}
	case tea.KeyCtrlC:
		return []byte{0x03}
	case tea.KeyCtrlD:
		return []byte{0x04}
	case tea.KeyCtrlE:
		return []byte{0x05}
	case tea.KeyCtrlF:
		return []byte{0x06}
	case tea.KeyCtrlG:
		return []byte{0x07}
	case tea.KeyCtrlH:
		return []byte{0x08}
	case tea.KeyCtrlJ:
		return []byte{0x0a}
	case tea.KeyCtrlK:
		return []byte{0x0b}
	case tea.KeyCtrlL:
		return []byte{0x0c}
	case tea.KeyCtrlN:
		return []byte{0x0e}
	case tea.KeyCtrlO:
		return []byte{0x0f}
	case tea.KeyCtrlP:
		return []byte{0x10}
	case tea.KeyCtrlR:
		return []byte{0x12}
	case tea.KeyCtrlT:
		return []byte{0x14}
	case tea.KeyCtrlU:
		return []byte{0x15}
	case tea.KeyCtrlV:
		return []byte{0x16}
	case tea.KeyCtrlW:
		return []byte{0x17}
	case tea.KeyCtrlX:
		return []byte{0x18}
	case tea.KeyCtrlY:
		return []byte{0x19}
	case tea.KeyCtrlZ:
		return []byte{0x1a}
	case tea.KeyCtrlUnderscore:
		return []byte{0x1f}
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	}
	return nil
}

// readSSHOutput is a goroutine that reads from SSH stdout and feeds it to the
// emulator. It signals via termChan whenever new data arrives.
func (m *Model) readSSHOutput(seq int, session *sshSession, out chan<- termChunk, stop <-chan struct{}) {
	if session == nil || out == nil {
		return
	}

	go func() {
		_ = session.session.Wait()
		_ = session.stdoutR.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, err := session.stdoutR.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case out <- termChunk{seq: seq, data: data}:
			case <-stop:
				close(out)
				return
			}
		}
		if err != nil {
			break
		}

		select {
		case <-stop:
			close(out)
			return
		default:
		}
	}
	select {
	case out <- termChunk{seq: seq, closed: true}:
	default:
	}
	close(out)
}

func (m *Model) readTerminalInput(session *sshSession, emulator *vt.SafeEmulator, stop <-chan struct{}) {
	if session == nil || emulator == nil {
		return
	}

	buf := make([]byte, 1024)
	for {
		n, err := emulator.Read(buf)
		if n > 0 {
			if _, werr := session.stdinW.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
		select {
		case <-stop:
			return
		default:
		}
	}
}

func (m *Model) readLogsOutput(seq int, session *logsStreamSession, out chan<- logsChunk, stop <-chan struct{}) {
	if session == nil || out == nil {
		return
	}

	go func() {
		_ = session.session.Wait()
		_ = session.stdoutR.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, err := session.stdoutR.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			select {
			case out <- logsChunk{seq: seq, data: data}:
			case <-stop:
				close(out)
				return
			}
		}
		if err != nil {
			break
		}

		select {
		case <-stop:
			close(out)
			return
		default:
		}
	}
	select {
	case out <- logsChunk{seq: seq, closed: true}:
	default:
	}
	close(out)
}

func (m *Model) waitLogsOutput(seq int) tea.Cmd {
	if m.logsChan == nil {
		return nil
	}
	ch := m.logsChan
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return logsOutputMsg{chunk: logsChunk{seq: seq, closed: true}}
		}
		return logsOutputMsg{chunk: chunk}
	}
}

func (m *Model) appendLogsChunk(data []byte) {
	if len(data) == 0 {
		return
	}
	combined := m.logsTail + string(data)
	parts := strings.Split(combined, "\n")
	m.logsTail = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		m.logsRawLines = append(m.logsRawLines, compactJournalTimestamp(line))
	}

	if m.logsTail != "" && len(parts) == 1 {
		return
	}

	if len(m.logsRawLines) > m.logsMaxLines {
		over := len(m.logsRawLines) - m.logsMaxLines
		m.logsRawLines = m.logsRawLines[over:]
	}
}

func (m *Model) applyLogsFilter() {
	m.logsViewLines = m.logsViewLines[:0]
	needle := strings.ToLower(strings.TrimSpace(m.logsFilter))
	if needle == "" {
		m.logsViewLines = append(m.logsViewLines, m.logsRawLines...)
		m.logsMatchIndex = -1
		return
	}

	negate := false
	if strings.HasPrefix(needle, "!") {
		negate = true
		needle = strings.TrimPrefix(needle, "!")
		needle = strings.TrimSpace(needle)
	}
	if needle == "" {
		m.logsViewLines = append(m.logsViewLines, m.logsRawLines...)
		m.logsMatchIndex = -1
		return
	}

	for _, line := range m.logsRawLines {
		lowerLine := strings.ToLower(line)
		matched := strings.Contains(lowerLine, needle)
		if negate {
			matched = !matched
		}
		if matched {
			m.logsViewLines = append(m.logsViewLines, line)
		}
	}
	if len(m.logsViewLines) == 0 {
		m.logsMatchIndex = -1
	} else if m.logsMatchIndex >= len(m.logsViewLines) {
		m.logsMatchIndex = 0
	}
}

func (m *Model) updateLogsViewport() {
	if m.panelKind != PanelLogs {
		return
	}
	lines := m.logsViewLines
	if len(lines) == 0 {
		if m.logsFilter != "" {
			m.termViewport.SetContent("No log lines match current filter")
		} else {
			m.termViewport.SetContent("Waiting for logs...")
		}
	} else {
		m.ensureLogsCursorInRange()
		rendered := make([]string, 0, len(lines))
		lineWidth := m.termViewport.Width
		if lineWidth < 1 {
			lineWidth = 1
		}
		normalLineStyle := lipgloss.NewStyle()
		if m.logsWrap {
			normalLineStyle = normalLineStyle.Width(lineWidth)
		}
		cursorLineStyle := LogsCursorStyle
		if m.logsWrap {
			cursorLineStyle = cursorLineStyle.Width(lineWidth)
		}
		selectedLineStyle := LogsSelectedStyle
		if m.logsWrap {
			selectedLineStyle = selectedLineStyle.Width(lineWidth)
		}
		selStart, selEnd := -1, -1
		if m.logsSelectMode {
			selStart, selEnd = m.logsSelectionBounds()
		}
		for i, line := range lines {
			if m.logsSelectMode && i >= selStart && i <= selEnd {
				if i == m.logsCursor {
					rendered = append(rendered, cursorLineStyle.Render(line))
				} else {
					rendered = append(rendered, selectedLineStyle.Render(line))
				}
				continue
			}
			if i == m.logsCursor {
				rendered = append(rendered, cursorLineStyle.Render(line))
				continue
			}
			rendered = append(rendered, normalLineStyle.Render(line))
		}
		m.termViewport.SetContent(strings.Join(rendered, "\n"))
	}

	if m.logsFollow {
		m.recomputeLogsCursorRows()
		m.termViewport.GotoBottom()
		if len(lines) > 0 {
			m.logsCursor = len(lines) - 1
			m.recomputeLogsCursorRows()
			if m.logsCursorRow1 >= 0 && m.termViewport.Height > 0 {
				target := m.logsCursorRow1 - m.termViewport.Height + 1
				if target < 0 {
					target = 0
				}
				m.termViewport.SetYOffset(target)
			}
		}
		return
	}

	m.ensureLogsCursorVisible()
}

func (m *Model) pauseLogsFollow() {
	m.logsFollow = false
	m.panelStatus = "Paused"
	m.panelStatusFg = ColorWarning
}

func (m *Model) ensureLogsCursorInRange() {
	if len(m.logsViewLines) == 0 {
		m.logsCursor = -1
		m.logsCursorRow0 = -1
		m.logsCursorRow1 = -1
		m.logsSelectMode = false
		m.logsSelectAnchor = -1
		return
	}
	if m.logsCursor < 0 {
		m.logsCursor = len(m.logsViewLines) - 1
	}
	if m.logsCursor >= len(m.logsViewLines) {
		m.logsCursor = len(m.logsViewLines) - 1
	}
	if m.logsSelectMode {
		if m.logsSelectAnchor < 0 {
			m.logsSelectAnchor = 0
		}
		if m.logsSelectAnchor >= len(m.logsViewLines) {
			m.logsSelectAnchor = len(m.logsViewLines) - 1
		}
	}
}

func (m *Model) moveLogsCursor(delta int) {
	if len(m.logsViewLines) == 0 {
		return
	}
	m.ensureLogsCursorInRange()
	m.logsCursor += delta
	if m.logsCursor < 0 {
		m.logsCursor = 0
	}
	if m.logsCursor >= len(m.logsViewLines) {
		m.logsCursor = len(m.logsViewLines) - 1
	}
	m.ensureLogsCursorVisible()
	m.updateLogsViewport()
}

func (m *Model) ensureLogsCursorVisible() {
	if m.logsCursor < 0 {
		return
	}
	m.recomputeLogsCursorRows()
	if m.logsCursorRow0 < 0 || m.logsCursorRow1 < 0 {
		return
	}

	top := m.termViewport.YOffset
	bottom := top + m.termViewport.Height - 1
	if m.logsCursorRow0 < top {
		m.termViewport.SetYOffset(m.logsCursorRow0)
		return
	}
	if m.logsCursorRow1 > bottom {
		m.termViewport.SetYOffset(m.logsCursorRow1 - m.termViewport.Height + 1)
	}
}

func (m *Model) recomputeLogsCursorRows() {
	if m.logsCursor < 0 || m.logsCursor >= len(m.logsViewLines) {
		m.logsCursorRow0 = -1
		m.logsCursorRow1 = -1
		return
	}

	width := m.termViewport.Width
	if width < 1 {
		width = 1
	}

	start := 0
	for i := 0; i < m.logsCursor; i++ {
		start += wrappedLineHeight(m.logsViewLines[i], width, m.logsWrap)
	}

	height := wrappedLineHeight(m.logsViewLines[m.logsCursor], width, m.logsWrap)
	m.logsCursorRow0 = start
	m.logsCursorRow1 = start + height - 1
}

func wrappedLineHeight(line string, width int, wrap bool) int {
	if !wrap {
		return 1
	}
	if width < 1 {
		return 1
	}
	w := lipgloss.Width(line)
	if w <= 0 {
		return 1
	}
	h := w / width
	if w%width != 0 {
		h++
	}
	if h < 1 {
		return 1
	}
	return h
}

func compactJournalTimestamp(line string) string {
	if line == "" {
		return line
	}
	firstSpace := strings.IndexByte(line, ' ')
	if firstSpace <= 0 {
		return line
	}
	token := line[:firstSpace]
	if len(token) < 19 || token[10] != 'T' {
		return line
	}
	// Convert ISO timestamp to standard date-time with year.
	return token[:10] + " " + token[11:19] + line[firstSpace:]
}

func copyTextToClipboard(text string) error {
	cmds := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"pbcopy"},
	}

	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			continue
		}
		if err := cmd.Start(); err != nil {
			_ = stdin.Close()
			continue
		}
		_, _ = stdin.Write([]byte(text))
		_ = stdin.Close()
		if err := cmd.Wait(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("clipboard command not available")
}

func (m *Model) logsSelectionBounds() (int, int) {
	if !m.logsSelectMode || len(m.logsViewLines) == 0 {
		return -1, -1
	}
	start := m.logsSelectAnchor
	end := m.logsCursor
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if end >= len(m.logsViewLines) {
		end = len(m.logsViewLines) - 1
	}
	if start >= len(m.logsViewLines) {
		start = len(m.logsViewLines) - 1
	}
	return start, end
}

// ── Channel-reading tea.Cmds ───────────────────────────────────────

func (m *Model) waitTermOutput(seq int) tea.Cmd {
	if m.termChan == nil {
		return nil
	}
	ch := m.termChan
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return sshSessionEndedMsg{seq: seq}
		}
		return termOutputMsg{chunk: chunk}
	}
}

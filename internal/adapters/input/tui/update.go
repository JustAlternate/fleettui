package tui

import (
	"fmt"
	"image/color"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/justalternate/fleetui/internal/domain"
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

	case "s":
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
	m.panelNode = node
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

func (m *Model) closePanel() {
	if m.termStop != nil {
		close(m.termStop)
		m.termStop = nil
	}
	closeSSH(m.ssh)
	if m.emulator != nil {
		m.emulator.Close()
	}
	m.termChan = nil
	m.panelMode = false
	m.panelNode = nil
	m.ssh = nil
	m.emulator = nil
	m.sshConnecting = false
	m.sshError = ""
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

	h := m.height - 8 - barHeight - helpHeight
	if m.height < 16 {
		h = m.height - 4 - barHeight - helpHeight
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

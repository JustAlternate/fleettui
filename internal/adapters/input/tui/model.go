package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/vt"
	"github.com/justalternate/fleettui/internal/domain"
	"github.com/justalternate/fleettui/internal/service"
)

// ViewMode represents which view is currently active in the TUI.
type ViewMode int

const (
	// ViewCards is the default card-grid view.
	ViewCards ViewMode = iota
	// ViewTable is the dense list/table view.
	ViewTable
)

type PanelKind int

const (
	PanelNone PanelKind = iota
	PanelSSH
	PanelLogs
)

type Model struct {
	nodes          []*domain.Node
	filteredNodes  []*domain.Node // subset of nodes matching current search
	config         *domain.Config
	collector      *service.MetricsCollector
	width          int
	height         int
	ctx            context.Context
	cancel         context.CancelFunc
	animationFrame int
	lastRefresh    int64
	viewport       viewport.Model
	tableViewport  viewport.Model // separate viewport so each view remembers its scroll position
	viewMode       ViewMode
	cursor         int  // selected row index in table view
	countPrefix    int  // vim-style numeric motion multiplier (e.g. 9 in "9j")
	searchMode     bool // whether the search bar is focused
	searchText     string
	searchInput    textinput.Model

	// SSH panel state
	panelMode     bool             // true when the SSH panel is active
	panelKind     PanelKind        // active panel kind
	panelNode     *domain.Node     // node being SSH'd into
	panelTitle    string           // panel title (e.g. SSH, Logs)
	panelStatus   string           // top-bar status text
	panelStatusFg lipgloss.Color   // top-bar status color
	panelHelp     string           // bottom help text for panel mode
	ssh           *sshSession      // SSH connection (nil until connected)
	emulator      *vt.SafeEmulator // terminal emulator
	termViewport  viewport.Model   // viewport for terminal output
	sshConnecting bool             // true while SSH handshake is in progress
	sshError      string           // connection error to display
	termChan      chan termChunk   // channel carrying terminal output chunks
	termStop      chan struct{}    // closes to stop output reader goroutine
	panelSeq      int              // monotonically increasing panel session id

	// Logs panel state
	logsSession      *logsStreamSession // active logs stream session
	logsChan         chan logsChunk     // channel carrying logs output chunks
	logsStop         chan struct{}      // closes to stop logs reader goroutine
	logsFollow       bool               // auto-scroll to bottom on new lines
	logsFilterMode   bool               // true while logs filter input is focused
	logsFilter       string             // active logs filter
	logsFilterInput  textinput.Model
	logsRawLines     []string // full logs history buffer
	logsViewLines    []string // filtered logs lines displayed in viewport
	logsTail         string   // partial line carried between chunks
	logsMaxLines     int
	logsMatchIndex   int
	logsCursor       int
	logsWrap         bool
	logsCursorRow0   int
	logsCursorRow1   int
	logsSelectMode   bool
	logsSelectAnchor int
}

// NewModel creates a new TUI model
func NewModel(nodes []*domain.Node, config *domain.Config, collector *service.MetricsCollector) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	vp := viewport.New(100, 20)
	vp.SetContent("Loading...")

	tvp := viewport.New(100, 20)
	tvp.SetContent("Loading...")

	si := textinput.New()
	si.Prompt = "/> "
	si.Placeholder = "Search name, status, IP..."
	si.PromptStyle = SearchPrefixStyle
	si.TextStyle = SearchTextStyle
	si.PlaceholderStyle = SearchPlaceholderStyle
	si.CharLimit = 64

	lsi := textinput.New()
	lsi.Prompt = "/> "
	lsi.Placeholder = "Search logs..."
	lsi.PromptStyle = SearchPrefixStyle
	lsi.TextStyle = SearchTextStyle
	lsi.PlaceholderStyle = SearchPlaceholderStyle
	lsi.CharLimit = 64

	tv := viewport.New(100, 20)
	tv.SetContent("")

	return &Model{
		nodes:            nodes,
		config:           config,
		collector:        collector,
		ctx:              ctx,
		cancel:           cancel,
		viewport:         vp,
		tableViewport:    tvp,
		searchInput:      si,
		logsFilterInput:  lsi,
		termViewport:     tv,
		logsFollow:       true,
		logsMaxLines:     5000,
		logsMatchIndex:   -1,
		logsCursor:       -1,
		logsWrap:         false,
		logsCursorRow0:   -1,
		logsCursorRow1:   -1,
		logsSelectMode:   false,
		logsSelectAnchor: -1,
	}
}

// Init initializes the TUI and starts the collector
func (m *Model) Init() tea.Cmd {
	go m.collector.Start(m.ctx)
	return tea.Batch(
		tickCmd(m.config.RefreshRate),
		animationTickCmd(),
	)
}

// View renders the TUI
func (m *Model) View() string {
	return m.renderView()
}

package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justalternate/fleetui/internal/domain"
	"github.com/justalternate/fleetui/internal/service"
)

// ViewMode represents which view is currently active in the TUI.
type ViewMode int

const (
	// ViewCards is the default card-grid view.
	ViewCards ViewMode = iota
	// ViewTable is the dense list/table view.
	ViewTable
	// ViewDetail is reserved for a future per-node detail panel.
	// ViewDetail
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

	return &Model{
		nodes:         nodes,
		config:        config,
		collector:     collector,
		ctx:           ctx,
		cancel:        cancel,
		viewport:      vp,
		tableViewport: tvp,
		searchInput:   si,
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

// getDisplayedNodes returns the filtered node list when a search is active,
// otherwise the full node list.
func (m *Model) getDisplayedNodes() []*domain.Node {
	if m.searchText != "" {
		return m.filteredNodes
	}
	return m.nodes
}

// applyFilter recomputes filteredNodes based on the current search text.
// The query is matched case-insensitively against node name, status, and IP.
// A leading "!" negates the match.
func (m *Model) applyFilter() {
	query := strings.TrimSpace(m.searchText)
	if query == "" {
		m.filteredNodes = nil
		return
	}

	negate := false
	if strings.HasPrefix(query, "!") {
		negate = true
		query = strings.TrimPrefix(query, "!")
		query = strings.TrimSpace(query)
	}
	lowerQuery := strings.ToLower(query)

	var result []*domain.Node
	for _, node := range m.nodes {
		haystack := strings.ToLower(node.Name + " " + nodeStatusString(node) + " " + node.IP)
		matched := strings.Contains(haystack, lowerQuery)
		if negate {
			matched = !matched
		}
		if matched {
			result = append(result, node)
		}
	}

	m.filteredNodes = result

	// Clamp cursor to filtered list size.
	displayed := m.getDisplayedNodes()
	if m.cursor >= len(displayed) && len(displayed) > 0 {
		m.cursor = len(displayed) - 1
	}
}

// nodeStatusString returns a lowercase status label for a node.
func nodeStatusString(node *domain.Node) string {
	switch {
	case node.IsPending():
		return "pending"
	case !node.IsAvailable():
		if node.Error != "" {
			return "error"
		}
		return "offline"
	default:
		return "online"
	}
}

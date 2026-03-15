package tui

import (
	"context"

	"fleettui/internal/domain"
	"fleettui/internal/service"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	nodes          []*domain.Node
	config         *domain.Config
	collector      *service.MetricsCollector
	width          int
	height         int
	ctx            context.Context
	cancel         context.CancelFunc
	animationFrame int
	lastRefresh    int64
	viewport       viewport.Model
	ready          bool
}

// NewModel creates a new TUI model
func NewModel(nodes []*domain.Node, config *domain.Config, collector *service.MetricsCollector) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	vp := viewport.New(100, 20)
	vp.SetContent("Loading...")
	return &Model{
		nodes:     nodes,
		config:    config,
		collector: collector,
		ctx:       ctx,
		cancel:    cancel,
		viewport:  vp,
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

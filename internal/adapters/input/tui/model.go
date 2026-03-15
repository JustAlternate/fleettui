package tui

import (
	"context"

	"fleettui/internal/domain"
	"fleettui/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the TUI application state
type Model struct {
	nodes     []*domain.Node
	config    *domain.Config
	collector *service.MetricsCollector
	width     int
	height    int
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewModel creates a new TUI model
func NewModel(nodes []*domain.Node, config *domain.Config, collector *service.MetricsCollector) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		nodes:     nodes,
		config:    config,
		collector: collector,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Init initializes the TUI and starts the collector
func (m *Model) Init() tea.Cmd {
	go m.collector.Start(m.ctx)
	return tickCmd(m.config.RefreshRate)
}

// View renders the TUI
func (m *Model) View() string {
	return m.renderView()
}

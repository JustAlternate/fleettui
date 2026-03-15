package tui

import (
	"context"
	"time"

	"fleettui/internal/domain"
	"fleettui/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type Model struct {
	nodes     []*domain.Node
	config    *domain.Config
	collector *service.MetricsCollector
	width     int
	height    int
	ctx       context.Context
	cancel    context.CancelFunc
}

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

func (m *Model) Init() tea.Cmd {
	go m.collector.Start(m.ctx)
	return tea.Batch(
		tickCmd(m.config.RefreshRate),
	)
}

func tickCmd(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return tickMsg(t)
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

func (m *Model) View() string {
	return m.renderView()
}

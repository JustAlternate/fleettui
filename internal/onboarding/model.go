package onboarding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fleettui/internal/adapters/output/config"
	"fleettui/internal/domain"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type step int

const (
	stepWelcome step = iota
	stepAddNode
	stepNodeName
	stepNodeIP
	stepNodeUser
	stepAddMoreNodes
	stepSelectMetrics
	stepComplete
)

type Model struct {
	step      step
	configDir string
	nodes     []domain.HostConfig
	metrics   []domain.MetricType

	// Form inputs
	nameInput textinput.Model
	ipInput   textinput.Model
	userInput textinput.Model

	// Current node being added
	currentNode domain.HostConfig

	// Metrics selection
	metricSelected map[domain.MetricType]bool
	cursor         int

	// Error message
	errMsg string

	// Completion
	complete bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D9FF")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginBottom(1)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			MarginTop(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)

func NewModel() *Model {
	m := &Model{
		step:           stepWelcome,
		configDir:      filepath.Join(os.Getenv("HOME"), ".config", "fleettui"),
		nodes:          []domain.HostConfig{},
		metrics:        []domain.MetricType{},
		metricSelected: make(map[domain.MetricType]bool),
		cursor:         0,
	}

	// Initialize all metrics as selected by default
	allMetrics := []domain.MetricType{
		domain.MetricCPU,
		domain.MetricRAM,
		domain.MetricNetwork,
		domain.MetricConnectivity,
		domain.MetricUptime,
		domain.MetricSystemd,
		domain.MetricOS,
	}
	for _, metric := range allMetrics {
		m.metricSelected[metric] = true
	}

	// Setup text inputs
	m.nameInput = textinput.New()
	m.nameInput.Placeholder = "e.g., web-server-01"
	m.nameInput.Focus()

	m.ipInput = textinput.New()
	m.ipInput.Placeholder = "e.g., 192.168.1.10"

	m.userInput = textinput.New()
	m.userInput.Placeholder = "e.g., root (press Enter for root)"

	return m
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "tab":
			return m.handleTab()

		case "up":
			if m.step == stepSelectMetrics && m.cursor > 0 {
				m.cursor--
			}

		case "down":
			if m.step == stepSelectMetrics {
				allMetrics := m.getAllMetrics()
				if m.cursor < len(allMetrics)-1 {
					m.cursor++
				}
			}

		case " ":
			if m.step == stepSelectMetrics {
				allMetrics := m.getAllMetrics()
				if m.cursor < len(allMetrics) {
					metric := allMetrics[m.cursor]
					m.metricSelected[metric] = !m.metricSelected[metric]
				}
			}

		case "y", "Y":
			if m.step == stepAddMoreNodes {
				m.step = stepNodeName
				m.nameInput.Focus()
			}

		case "n", "N":
			if m.step == stepAddMoreNodes {
				m.step = stepSelectMetrics
			}
		}
	}

	// Update text inputs
	switch m.step {
	case stepNodeName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepNodeIP:
		m.ipInput, cmd = m.ipInput.Update(msg)
	case stepNodeUser:
		m.userInput, cmd = m.userInput.Update(msg)
	}

	return m, cmd
}

func (m *Model) View() string {
	switch m.step {
	case stepWelcome:
		return m.welcomeView()
	case stepAddNode:
		return m.addNodeView()
	case stepNodeName:
		return m.nodeNameView()
	case stepNodeIP:
		return m.nodeIPView()
	case stepNodeUser:
		return m.nodeUserView()
	case stepAddMoreNodes:
		return m.addMoreNodesView()
	case stepSelectMetrics:
		return m.selectMetricsView()
	case stepComplete:
		return m.completeView()
	default:
		return "Unknown step"
	}
}

func (m *Model) welcomeView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Welcome to FleetTUI "),
		"",
		"FleetTUI is a beautiful terminal UI for monitoring your server fleet.",
		"",
		subtitleStyle.Render("Let's set up your configuration."),
		"",
		promptStyle.Render("Press Enter to continue..."),
		helpStyle.Render("[ctrl+c] Quit"),
	)
}

func (m *Model) addNodeView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Add Your First Node "),
		"",
		"A node is a server you want to monitor.",
		"You'll need:",
		"  • A name (e.g., 'web-server-01')",
		"  • An IP address or hostname",
		"  • SSH username (defaults to root)",
		"",
		promptStyle.Render("Press Enter to add a node..."),
		helpStyle.Render("[ctrl+c] Quit"),
	)
}

func (m *Model) nodeNameView() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Node Name "),
		"",
		"What would you like to call this node?",
		"",
		m.nameInput.View(),
	)

	if m.errMsg != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, errorStyle.Render(m.errMsg))
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, helpStyle.Render("[Enter] Continue • [ctrl+c] Quit"))

	return content
}

func (m *Model) nodeIPView() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Node IP Address "),
		"",
		fmt.Sprintf("What's the IP address or hostname for '%s'?", m.currentNode.Name),
		"",
		m.ipInput.View(),
	)

	if m.errMsg != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, errorStyle.Render(m.errMsg))
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content, helpStyle.Render("[Enter] Continue • [ctrl+c] Quit"))

	return content
}

func (m *Model) nodeUserView() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" SSH User "),
		"",
		fmt.Sprintf("What user should connect to '%s'?", m.currentNode.Name),
		"",
		m.userInput.View(),
		"",
		subtitleStyle.Render("(Press Enter to use 'root')"),
	)

	content = lipgloss.JoinVertical(lipgloss.Left, content, helpStyle.Render("[Enter] Continue • [ctrl+c] Quit"))

	return content
}

func (m *Model) addMoreNodesView() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Add More Nodes? "),
		"",
		fmt.Sprintf("You've added %d node(s):", len(m.nodes)),
	)

	for _, node := range m.nodes {
		content = lipgloss.JoinVertical(lipgloss.Left, content, fmt.Sprintf("  • %s (%s)", node.Name, node.IP))
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content,
		"",
		"Would you like to add another node?",
		"",
		promptStyle.Render("[y] Yes, add another • [n] No, continue"),
		helpStyle.Render("[ctrl+c] Quit"),
	)

	return content
}

func (m *Model) selectMetricsView() string {
	allMetrics := m.getAllMetrics()

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Select Metrics "),
		"",
		"Which metrics would you like to monitor?",
		"",
		"(Use ↑/↓ to navigate, Space to toggle)",
		"",
	)

	for i, metric := range allMetrics {
		style := unselectedStyle
		prefix := "[ ]"

		if m.metricSelected[metric] {
			style = selectedStyle
			prefix = "[✓]"
		}

		if i == m.cursor {
			prefix = "> " + prefix
		} else {
			prefix = "  " + prefix
		}

		content = lipgloss.JoinVertical(lipgloss.Left, content, style.Render(fmt.Sprintf("%s %s", prefix, m.metricDisplayName(metric))))
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content,
		"",
		helpStyle.Render("[Enter] Continue • [Space] Toggle • [ctrl+c] Quit"),
	)

	return content
}

func (m *Model) completeView() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Setup Complete! "),
		"",
		"Your configuration has been saved to:",
		fmt.Sprintf("  %s", m.configDir),
		"",
		fmt.Sprintf("• %d node(s) configured", len(m.nodes)),
		fmt.Sprintf("• %d metric(s) enabled", len(m.getSelectedMetrics())),
		"",
		"You can now run 'fleettui' to start monitoring!",
		"",
		promptStyle.Render("Press Enter to exit..."),
	)
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	m.errMsg = ""

	switch m.step {
	case stepWelcome:
		m.step = stepAddNode

	case stepAddNode:
		m.step = stepNodeName
		m.nameInput.Focus()

	case stepNodeName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.errMsg = "Please enter a name for this node"
			return m, nil
		}
		m.currentNode.Name = name
		m.step = stepNodeIP
		m.ipInput.Focus()

	case stepNodeIP:
		ip := strings.TrimSpace(m.ipInput.Value())
		if ip == "" {
			m.errMsg = "Please enter an IP address or hostname"
			return m, nil
		}
		m.currentNode.IP = ip
		m.step = stepNodeUser
		m.userInput.Focus()

	case stepNodeUser:
		user := strings.TrimSpace(m.userInput.Value())
		if user == "" {
			user = "root"
		}
		m.currentNode.User = user
		m.nodes = append(m.nodes, m.currentNode)

		// Reset for next node
		m.currentNode = domain.HostConfig{}
		m.nameInput.SetValue("")
		m.ipInput.SetValue("")
		m.userInput.SetValue("")

		m.step = stepAddMoreNodes

	case stepSelectMetrics:
		m.metrics = m.getSelectedMetrics()
		if err := m.saveConfig(); err != nil {
			m.errMsg = fmt.Sprintf("Error saving config: %v", err)
			return m, nil
		}
		m.step = stepComplete

	case stepComplete:
		m.complete = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *Model) handleTab() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepAddMoreNodes:
		// Default to 'n' (no more nodes)
		m.step = stepSelectMetrics
	}
	return m, nil
}

func (m *Model) getAllMetrics() []domain.MetricType {
	return []domain.MetricType{
		domain.MetricCPU,
		domain.MetricRAM,
		domain.MetricNetwork,
		domain.MetricConnectivity,
		domain.MetricUptime,
		domain.MetricSystemd,
		domain.MetricOS,
	}
}

func (m *Model) getSelectedMetrics() []domain.MetricType {
	var selected []domain.MetricType
	for metric, isSelected := range m.metricSelected {
		if isSelected {
			selected = append(selected, metric)
		}
	}
	return selected
}

func (m *Model) metricDisplayName(metric domain.MetricType) string {
	switch metric {
	case domain.MetricCPU:
		return "CPU Usage"
	case domain.MetricRAM:
		return "RAM Usage"
	case domain.MetricNetwork:
		return "Network I/O"
	case domain.MetricConnectivity:
		return "Connectivity"
	case domain.MetricUptime:
		return "Uptime"
	case domain.MetricSystemd:
		return "Systemd Units"
	case domain.MetricOS:
		return "Operating System"
	default:
		return string(metric)
	}
}

func (m *Model) saveConfig() error {
	// Create config directory
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save hosts.yaml
	hostsConfig := &domain.HostsConfig{Hosts: m.nodes}
	loader := config.NewLoader()

	hostsPath := filepath.Join(m.configDir, "hosts.yaml")
	if err := loader.SaveHosts(hostsPath, hostsConfig); err != nil {
		return fmt.Errorf("failed to save hosts: %w", err)
	}

	// Save config.yaml
	appConfig := &domain.Config{
		EnabledMetrics: m.metrics,
	}
	configPath := filepath.Join(m.configDir, "config.yaml")
	if err := loader.SaveConfig(configPath, appConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func (m *Model) IsComplete() bool {
	return m.complete
}

// Run starts the onboarding process and returns true if config was created
func Run() (bool, error) {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "fleettui")

	// Check if config already exists
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		// Config already exists, skip onboarding
		return false, nil
	}

	m := NewModel()
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return false, err
	}

	return m.IsComplete(), nil
}

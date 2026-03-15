package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fleettui/internal/adapters/input/tui"
	"fleettui/internal/adapters/output/config"
	"fleettui/internal/domain"
	"fleettui/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "fleettui")
	configPath := filepath.Join(configDir, "config.yaml")
	hostsPath := filepath.Join(configDir, "hosts.yaml")

	loader := config.NewLoader()

	cfg, err := loader.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	hostsCfg, err := loader.LoadHosts(hostsPath)
	if err != nil {
		log.Fatalf("Failed to load hosts: %v", err)
	}

	nodes := make([]*domain.Node, len(hostsCfg.Hosts))
	for i, host := range hostsCfg.Hosts {
		nodes[i] = &domain.Node{
			Name:       host.Name,
			IP:         host.IP,
			User:       host.User,
			SSHKeyPath: host.SSHKeyPath,
		}
	}

	if len(nodes) == 0 {
		fmt.Println("No hosts configured.")
		fmt.Printf("Please add hosts to %s\n", hostsPath)
		fmt.Println("\nExample hosts.yaml:")
		fmt.Println("hosts:")
		fmt.Println("  - name: server-01")
		fmt.Println("    ip: 192.168.1.10")
		fmt.Println("    user: root")
		os.Exit(1)
	}

	collector := service.NewMetricsCollector(cfg, nodes, nil)

	model := tui.NewModel(nodes, cfg, collector)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

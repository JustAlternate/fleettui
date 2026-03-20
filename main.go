package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justalternate/fleetui/internal/adapters/input/tui"
	"github.com/justalternate/fleetui/internal/adapters/output/config"
	"github.com/justalternate/fleetui/internal/adapters/output/ssh"
	"github.com/justalternate/fleetui/internal/domain"
	"github.com/justalternate/fleetui/internal/ports/output"
	"github.com/justalternate/fleetui/internal/service"
)

func main() {
	hostsFlag := flag.String("hosts", "", "Path to hosts.yaml (default: ~/.config/fleettui/hosts.yaml)")
	flag.Parse()

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "fleettui")
	configPath := filepath.Join(configDir, "config.yaml")
	hostsPath := filepath.Join(configDir, "hosts.yaml")

	if *hostsFlag != "" {
		hostsPath = *hostsFlag
	}

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

	collectorFactory := func(client output.SSHClient) output.MetricsCollector {
		return ssh.NewCollector(client)
	}

	pool := ssh.NewConnectionPool()
	collector := service.NewMetricsCollector(cfg, nodes, pool, collectorFactory)

	model := tui.NewModel(nodes, cfg, collector)

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

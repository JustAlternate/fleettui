package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fleettui/internal/adapters/input/tui"
	"fleettui/internal/adapters/output/config"
	"fleettui/internal/adapters/output/ssh"
	"fleettui/internal/domain"
	"fleettui/internal/onboarding"
	"fleettui/internal/ports/output"
	"fleettui/internal/service"
	tea "github.com/charmbracelet/bubbletea"
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

	if *hostsFlag == "" {
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			fmt.Println("Welcome to FleetTUI!")
			fmt.Println("It looks like this is your first time running the application.")
			fmt.Println()

			completed, err := onboarding.Run()
			if err != nil {
				log.Fatalf("Onboarding failed: %v", err)
			}

			if !completed {
				fmt.Println("\nOnboarding cancelled. Exiting...")
				os.Exit(0)
			}

			fmt.Println("\nConfiguration complete! Starting FleetTUI...")
			fmt.Println()
		}
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

	collector := service.NewMetricsCollector(cfg, nodes, collectorFactory)

	model := tui.NewModel(nodes, cfg, collector)

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

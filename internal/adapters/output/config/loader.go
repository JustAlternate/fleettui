package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fleettui/internal/domain"
	"gopkg.in/yaml.v3"
)

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

type yamlConfig struct {
	RefreshRate string   `yaml:"refresh_rate"`
	Metrics     []string `yaml:"metrics"`
}

func (l *Loader) LoadConfig(path string) (*domain.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return l.defaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlCfg yamlConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	config := &domain.Config{
		EnabledMetrics: make([]domain.MetricType, 0, len(yamlCfg.Metrics)),
	}

	if yamlCfg.RefreshRate != "" {
		duration, err := time.ParseDuration(yamlCfg.RefreshRate)
		if err != nil {
			return nil, fmt.Errorf("invalid refresh_rate: %w", err)
		}
		config.RefreshRate = duration
	} else {
		config.RefreshRate = 5 * time.Second
	}

	for _, m := range yamlCfg.Metrics {
		config.EnabledMetrics = append(config.EnabledMetrics, domain.MetricType(m))
	}

	if len(config.EnabledMetrics) == 0 {
		config.EnabledMetrics = []domain.MetricType{
			domain.MetricCPU,
			domain.MetricRAM,
			domain.MetricNetwork,
			domain.MetricConnectivity,
			domain.MetricUptime,
			domain.MetricSystemd,
			domain.MetricOS,
		}
	}

	return config, nil
}

func (l *Loader) defaultConfig() *domain.Config {
	return &domain.Config{
		RefreshRate: 5 * time.Second,
		EnabledMetrics: []domain.MetricType{
			domain.MetricCPU,
			domain.MetricRAM,
			domain.MetricNetwork,
			domain.MetricConnectivity,
			domain.MetricUptime,
			domain.MetricSystemd,
			domain.MetricOS,
		},
	}
}

func (l *Loader) LoadHosts(path string) (*domain.HostsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.HostsConfig{Hosts: []domain.HostConfig{}}, nil
		}
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	var hostsCfg domain.HostsConfig
	if err := yaml.Unmarshal(data, &hostsCfg); err != nil {
		return nil, fmt.Errorf("failed to parse hosts file: %w", err)
	}

	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	defaultKey := l.findFirstSSHKey(sshDir)

	for i := range hostsCfg.Hosts {
		if hostsCfg.Hosts[i].User == "" {
			hostsCfg.Hosts[i].User = "root"
		}
		if hostsCfg.Hosts[i].SSHKeyPath == "" && defaultKey != "" {
			hostsCfg.Hosts[i].SSHKeyPath = defaultKey
		}
	}

	return &hostsCfg, nil
}

func (l *Loader) findFirstSSHKey(sshDir string) string {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "config" || name == "known_hosts" || name == "authorized_keys" {
			continue
		}
		if filepath.Ext(name) == ".pub" {
			continue
		}
		return filepath.Join(sshDir, name)
	}

	return ""
}

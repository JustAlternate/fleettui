package domain

import (
	"time"
)

type MetricType string

const (
	MetricCPU          MetricType = "cpu"
	MetricRAM          MetricType = "ram"
	MetricNetwork      MetricType = "network"
	MetricConnectivity MetricType = "connectivity"
	MetricUptime       MetricType = "uptime"
	MetricSystemd      MetricType = "systemd"
	MetricOS           MetricType = "os"
)

type Config struct {
	RefreshRate    time.Duration
	EnabledMetrics []MetricType
}

type HostConfig struct {
	Name       string `yaml:"name"`
	IP         string `yaml:"ip"`
	User       string `yaml:"user,omitempty"`
	SSHKeyPath string `yaml:"ssh_key_path,omitempty"`
}

type HostsConfig struct {
	Hosts []HostConfig `yaml:"hosts"`
}

func (c *Config) IsMetricEnabled(metric MetricType) bool {
	for _, m := range c.EnabledMetrics {
		if m == metric {
			return true
		}
	}
	return false
}

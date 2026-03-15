package domain

import (
	"testing"
	"time"
)

func TestConfig_IsMetricEnabled(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		metric      MetricType
		wantEnabled bool
	}{
		{
			name: "metric is enabled when in list",
			config: Config{
				EnabledMetrics: []MetricType{MetricCPU, MetricRAM, MetricNetwork},
			},
			metric:      MetricCPU,
			wantEnabled: true,
		},
		{
			name: "metric is disabled when not in list",
			config: Config{
				EnabledMetrics: []MetricType{MetricCPU, MetricRAM},
			},
			metric:      MetricNetwork,
			wantEnabled: false,
		},
		{
			name: "all metrics disabled when list is empty",
			config: Config{
				EnabledMetrics: []MetricType{},
			},
			metric:      MetricCPU,
			wantEnabled: false,
		},
		{
			name: "all metrics disabled when list is nil",
			config: Config{
				EnabledMetrics: nil,
			},
			metric:      MetricCPU,
			wantEnabled: false,
		},
		{
			name: "metric enabled as last item",
			config: Config{
				EnabledMetrics: []MetricType{MetricCPU, MetricRAM, MetricNetwork, MetricConnectivity, MetricUptime},
			},
			metric:      MetricUptime,
			wantEnabled: true,
		},
		{
			name: "check each metric type exists",
			config: Config{
				EnabledMetrics: []MetricType{
					MetricCPU,
					MetricRAM,
					MetricNetwork,
					MetricConnectivity,
					MetricUptime,
					MetricSystemd,
					MetricOS,
				},
			},
			metric:      MetricOS,
			wantEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsMetricEnabled(tt.metric)
			if got != tt.wantEnabled {
				t.Errorf("IsMetricEnabled(%q) = %v, want %v", tt.metric, got, tt.wantEnabled)
			}
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	refreshRate := 5 * time.Second
	config := Config{
		RefreshRate:    refreshRate,
		EnabledMetrics: []MetricType{MetricCPU, MetricRAM},
	}

	if config.RefreshRate != refreshRate {
		t.Errorf("RefreshRate = %v, want %v", config.RefreshRate, refreshRate)
	}
	if len(config.EnabledMetrics) != 2 {
		t.Errorf("len(EnabledMetrics) = %v, want %v", len(config.EnabledMetrics), 2)
	}
}

func TestHostsConfig_Fields(t *testing.T) {
	hosts := HostsConfig{
		Hosts: []HostConfig{
			{
				Name:       "server-01",
				IP:         "192.168.1.10",
				User:       "root",
				SSHKeyPath: "/home/user/.ssh/id_rsa",
			},
			{
				Name:       "server-02",
				IP:         "192.168.1.11",
				User:       "admin",
				SSHKeyPath: "",
			},
		},
	}

	if len(hosts.Hosts) != 2 {
		t.Errorf("len(Hosts) = %v, want %v", len(hosts.Hosts), 2)
	}

	if hosts.Hosts[0].Name != "server-01" {
		t.Errorf("Hosts[0].Name = %v, want %v", hosts.Hosts[0].Name, "server-01")
	}

	if hosts.Hosts[1].User != "admin" {
		t.Errorf("Hosts[1].User = %v, want %v", hosts.Hosts[1].User, "admin")
	}
}

func TestMetricType_Constants(t *testing.T) {
	tests := []struct {
		metric MetricType
		want   string
	}{
		{MetricCPU, "cpu"},
		{MetricRAM, "ram"},
		{MetricNetwork, "network"},
		{MetricConnectivity, "connectivity"},
		{MetricUptime, "uptime"},
		{MetricSystemd, "systemd"},
		{MetricOS, "os"},
	}

	for _, tt := range tests {
		t.Run(string(tt.metric), func(t *testing.T) {
			if string(tt.metric) != tt.want {
				t.Errorf("MetricType = %v, want %v", tt.metric, tt.want)
			}
		})
	}
}

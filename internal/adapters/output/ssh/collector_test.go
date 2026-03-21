package ssh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/justalternate/fleettui/internal/domain"
	"github.com/justalternate/fleettui/internal/mocks"
	"github.com/stretchr/testify/mock"
)

func TestCollector_ParseCPU(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantUsage float64
		wantErr   bool
	}{
		{
			name:      "parses valid CPU usage",
			output:    "cpu  1000 0 1000 8000 0 0 0 0 0 0",
			wantUsage: 20.0,
			wantErr:   false,
		},
		{
			name:      "parses zero CPU usage",
			output:    "cpu  0 0 0 10000 0 0 0 0 0 0",
			wantUsage: 0.0,
			wantErr:   false,
		},
		{
			name:      "returns error on invalid output",
			output:    "cpu invalid",
			wantUsage: 0,
			wantErr:   true,
		},
		{
			name:      "returns error on empty output",
			output:    "",
			wantUsage: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			metrics, err := collector.parseCPU(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.UsagePercent < tt.wantUsage-0.01 || metrics.UsagePercent > tt.wantUsage+0.01 {
				t.Errorf("UsagePercent = %v, want %v", metrics.UsagePercent, tt.wantUsage)
			}
		})
	}
}

func TestCollector_ParseRAM(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		wantTotal     uint64
		wantUsed      uint64
		wantUsagePerc float64
		wantErr       bool
	}{
		{
			name:          "parses valid RAM metrics",
			output:        "MemTotal:       8388608 kB\nMemAvailable:   4194304 kB\n",
			wantTotal:     8589934592,
			wantUsed:      4294967296,
			wantUsagePerc: 50.0,
			wantErr:       false,
		},
		{
			name:          "parses zero usage",
			output:        "MemTotal:       8388608 kB\nMemAvailable:   8388608 kB\n",
			wantTotal:     8589934592,
			wantUsed:      0,
			wantUsagePerc: 0.0,
			wantErr:       false,
		},
		{
			name:          "parses full usage",
			output:        "MemTotal:       8388608 kB\nMemAvailable:   0 kB\n",
			wantTotal:     8589934592,
			wantUsed:      8589934592,
			wantUsagePerc: 100.0,
			wantErr:       false,
		},
		{
			name:    "returns error on invalid format",
			output:  "MemTotal: invalid",
			wantErr: true,
		},
		{
			name:    "returns error on missing MemTotal",
			output:  "MemAvailable:   4194304 kB\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			metrics, err := collector.parseRAM(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.TotalBytes != tt.wantTotal {
				t.Errorf("TotalBytes = %v, want %v", metrics.TotalBytes, tt.wantTotal)
			}
			if metrics.UsedBytes != tt.wantUsed {
				t.Errorf("UsedBytes = %v, want %v", metrics.UsedBytes, tt.wantUsed)
			}
			if metrics.UsagePercent < tt.wantUsagePerc-0.01 || metrics.UsagePercent > tt.wantUsagePerc+0.01 {
				t.Errorf("UsagePercent = %v, want %v", metrics.UsagePercent, tt.wantUsagePerc)
			}
		})
	}
}

func TestCollector_ParseUptime(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantMin time.Duration
		wantMax time.Duration
		wantErr bool
	}{
		{
			name:    "parses valid uptime in seconds",
			output:  "3600.50 1234.5",
			wantMin: 3600 * time.Second,
			wantMax: 3601 * time.Second,
			wantErr: false,
		},
		{
			name:    "parses zero uptime",
			output:  "0.0 0.0",
			wantMin: 0,
			wantMax: 1 * time.Second,
			wantErr: false,
		},
		{
			name:    "returns error on invalid format",
			output:  "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			duration, err := collector.parseUptime(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if duration < tt.wantMin || duration > tt.wantMax {
				t.Errorf("duration = %v, want between %v and %v", duration, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCollector_ParseNetwork(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name:    "parses valid network output",
			output:  "eth0: 1048576 100 0 0 0 0 0 0 1048576 100 0 0 0 0 0 0",
			wantErr: false,
		},
		{
			name:    "returns error on invalid format",
			output:  "invalid",
			wantErr: true,
		},
		{
			name:    "returns error on missing fields",
			output:  "eth0: 100 100",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			_, err := collector.parseNetwork(tt.output, &domain.Node{})

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}

func TestCollector_ParseSystemd(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantFailed int
		wantTotal  int
		wantErr    bool
	}{
		{
			name:       "parses valid systemd metrics",
			output:     "2\n150",
			wantFailed: 2,
			wantTotal:  150,
			wantErr:    false,
		},
		{
			name:       "parses zero failed units",
			output:     "0\n200",
			wantFailed: 0,
			wantTotal:  200,
			wantErr:    false,
		},
		{
			name:    "returns error on insufficient fields",
			output:  "2",
			wantErr: true,
		},
		{
			name:    "returns error on invalid failed count",
			output:  "invalid\n100",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			metrics, err := collector.parseSystemd(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.FailedCount != tt.wantFailed {
				t.Errorf("FailedCount = %v, want %v", metrics.FailedCount, tt.wantFailed)
			}
			if metrics.TotalCount != tt.wantTotal {
				t.Errorf("TotalCount = %v, want %v", metrics.TotalCount, tt.wantTotal)
			}
		})
	}
}

func TestCollector_ParseOS(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantOS  string
		wantErr bool
	}{
		{
			name:    "parses valid OS info",
			output:  "NAME=\"Ubuntu\"\nPRETTY_NAME=\"Ubuntu 22.04.3 LTS\"\n",
			wantOS:  "Ubuntu 22.04.3 LTS",
			wantErr: false,
		},
		{
			name:    "returns error on empty output",
			output:  "",
			wantOS:  "",
			wantErr: true,
		},
		{
			name:    "returns error on missing PRETTY_NAME",
			output:  "NAME=\"Ubuntu\"\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			collector := &Collector{client: mockClient}
			os, err := collector.parseOS(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if os != tt.wantOS {
				t.Errorf("OS = %q, want %q", os, tt.wantOS)
			}
		})
	}
}

func TestCollector_Connectivity_ViaDirect_Connect(t *testing.T) {
	tests := []struct {
		name       string
		connectErr error
		want       bool
	}{
		{
			name:       "connected when connect succeeds",
			connectErr: nil,
			want:       true,
		},
		{
			name:       "not connected when connect fails",
			connectErr: errors.New("connection refused"),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("Connect", mock.Anything, mock.Anything).Return(tt.connectErr)

			if tt.connectErr == nil {
				mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return("---FLEETTUI_SEP_OS---\nPRETTY_NAME=\"Ubuntu 22.04\"", nil)
			}

			collector := &Collector{client: mockClient}
			node := &domain.Node{Name: "test", IP: "192.168.1.1"}
			config := &domain.Config{
				RefreshRate:    5 * time.Second,
				EnabledMetrics: []domain.MetricType{domain.MetricOS},
			}

			metrics, err := collector.CollectMetrics(context.Background(), node, config)

			if tt.connectErr != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if metrics.Connectivity != tt.want {
				t.Errorf("Connectivity = %v, want %v", metrics.Connectivity, tt.want)
			}
		})
	}
}

func TestCollector_CollectMetrics_Connectivity(t *testing.T) {
	tests := []struct {
		name       string
		connectErr error
		wantConn   bool
		wantErr    bool
	}{
		{
			name:       "returns metrics with connectivity when connected",
			connectErr: nil,
			wantConn:   true,
			wantErr:    false,
		},
		{
			name:       "returns metrics without connectivity when disconnected",
			connectErr: errors.New("connection failed"),
			wantConn:   false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("Connect", mock.Anything, mock.Anything).Return(tt.connectErr)

			if tt.connectErr == nil {
				mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return("---FLEETTUI_SEP_OS---\nPRETTY_NAME=\"Ubuntu 22.04\"", nil)
			}

			collector := &Collector{client: mockClient}
			config := &domain.Config{
				RefreshRate:    5 * time.Second,
				EnabledMetrics: []domain.MetricType{domain.MetricOS},
			}
			node := &domain.Node{Name: "test", IP: "192.168.1.1"}

			metrics, err := collector.CollectMetrics(context.Background(), node, config)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.Connectivity != tt.wantConn {
				t.Errorf("Connectivity = %v, want %v", metrics.Connectivity, tt.wantConn)
			}
		})
	}
}

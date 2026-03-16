package ssh

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleettui/internal/domain"
	"fleettui/internal/mocks"
	"github.com/stretchr/testify/mock"
)

func TestCollector_CollectCPU(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		cmdError  error
		wantUsage float64
		wantErr   bool
	}{
		{
			name:      "parses valid CPU usage",
			output:    "45.3",
			cmdError:  nil,
			wantUsage: 45.3,
			wantErr:   false,
		},
		{
			name:      "parses zero CPU usage",
			output:    "0.0",
			cmdError:  nil,
			wantUsage: 0.0,
			wantErr:   false,
		},
		{
			name:      "parses 100% CPU usage",
			output:    "100.0",
			cmdError:  nil,
			wantUsage: 100.0,
			wantErr:   false,
		},
		{
			name:      "handles whitespace in output",
			output:    "  67.5  ",
			cmdError:  nil,
			wantUsage: 67.5,
			wantErr:   false,
		},
		{
			name:      "returns error on command failure",
			output:    "",
			cmdError:  errors.New("connection failed"),
			wantUsage: 0,
			wantErr:   true,
		},
		{
			name:      "returns error on invalid output",
			output:    "invalid",
			cmdError:  nil,
			wantUsage: 0,
			wantErr:   true,
		},
		{
			name:      "returns error on empty output",
			output:    "",
			cmdError:  nil,
			wantUsage: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(tt.output, tt.cmdError)

			collector := &Collector{client: mockClient}
			metrics, err := collector.collectCPU(context.Background())

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

			if metrics.UsagePercent != tt.wantUsage {
				t.Errorf("UsagePercent = %v, want %v", metrics.UsagePercent, tt.wantUsage)
			}
		})
	}
}

func TestCollector_CollectRAM(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		cmdError      error
		wantTotal     uint64
		wantUsed      uint64
		wantUsagePerc float64
		wantErr       bool
	}{
		{
			name:          "parses valid RAM metrics",
			output:        "8589934592 4294967296",
			cmdError:      nil,
			wantTotal:     8589934592,
			wantUsed:      4294967296,
			wantUsagePerc: 50.0,
			wantErr:       false,
		},
		{
			name:          "parses zero usage",
			output:        "8589934592 0",
			cmdError:      nil,
			wantTotal:     8589934592,
			wantUsed:      0,
			wantUsagePerc: 0.0,
			wantErr:       false,
		},
		{
			name:          "parses full usage",
			output:        "8589934592 8589934592",
			cmdError:      nil,
			wantTotal:     8589934592,
			wantUsed:      8589934592,
			wantUsagePerc: 100.0,
			wantErr:       false,
		},
		{
			name:          "handles whitespace",
			output:        "  8589934592   4294967296  ",
			cmdError:      nil,
			wantTotal:     8589934592,
			wantUsed:      4294967296,
			wantUsagePerc: 50.0,
			wantErr:       false,
		},
		{
			name:     "returns error on command failure",
			output:   "",
			cmdError: errors.New("connection failed"),
			wantErr:  true,
		},
		{
			name:     "returns error on invalid format",
			output:   "only_one_value",
			cmdError: nil,
			wantErr:  true,
		},
		{
			name:     "returns error on non-numeric values",
			output:   "total used",
			cmdError: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(tt.output, tt.cmdError)

			collector := &Collector{client: mockClient}
			metrics, err := collector.collectRAM(context.Background())

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
			// Allow small floating point differences
			if metrics.UsagePercent < tt.wantUsagePerc-0.01 || metrics.UsagePercent > tt.wantUsagePerc+0.01 {
				t.Errorf("UsagePercent = %v, want %v", metrics.UsagePercent, tt.wantUsagePerc)
			}
		})
	}
}

func TestCollector_CollectUptime(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		cmdError error
		wantMin  time.Duration
		wantMax  time.Duration
		wantErr  bool
	}{
		{
			name:     "parses valid uptime in seconds",
			output:   "3600.50",
			cmdError: nil,
			wantMin:  3600 * time.Second,
			wantMax:  3601 * time.Second,
			wantErr:  false,
		},
		{
			name:     "parses zero uptime",
			output:   "0.0",
			cmdError: nil,
			wantMin:  0,
			wantMax:  1 * time.Second,
			wantErr:  false,
		},
		{
			name:     "parses large uptime",
			output:   "86400.00",
			cmdError: nil,
			wantMin:  24 * time.Hour,
			wantMax:  24*time.Hour + time.Second,
			wantErr:  false,
		},
		{
			name:     "handles whitespace",
			output:   "  7200.25  ",
			cmdError: nil,
			wantMin:  2 * time.Hour,
			wantMax:  2*time.Hour + time.Second,
			wantErr:  false,
		},
		{
			name:     "returns error on command failure",
			output:   "",
			cmdError: errors.New("connection failed"),
			wantErr:  true,
		},
		{
			name:     "returns error on invalid format",
			output:   "invalid",
			cmdError: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(tt.output, tt.cmdError)

			collector := &Collector{client: mockClient}
			duration, err := collector.collectUptime(context.Background())

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

func TestCollector_CollectSystemd(t *testing.T) {
	tests := []struct {
		name         string
		failedOutput string
		totalOutput  string
		failedError  error
		totalError   error
		wantFailed   int
		wantTotal    int
		wantErr      bool
	}{
		{
			name:         "parses valid systemd metrics",
			failedOutput: "2",
			totalOutput:  "150",
			failedError:  nil,
			totalError:   nil,
			wantFailed:   2,
			wantTotal:    150,
			wantErr:      false,
		},
		{
			name:         "parses zero failed units",
			failedOutput: "0",
			totalOutput:  "200",
			failedError:  nil,
			totalError:   nil,
			wantFailed:   0,
			wantTotal:    200,
			wantErr:      false,
		},
		{
			name:         "handles whitespace",
			failedOutput: "  5  ",
			totalOutput:  "  180  ",
			failedError:  nil,
			totalError:   nil,
			wantFailed:   5,
			wantTotal:    180,
			wantErr:      false,
		},
		{
			name:        "returns error on failed command error",
			failedError: errors.New("command failed"),
			wantErr:     true,
		},
		{
			name:         "returns error on total command error",
			failedOutput: "0",
			failedError:  nil,
			totalError:   errors.New("command failed"),
			wantErr:      true,
		},
		{
			name:         "returns error on invalid failed count",
			failedOutput: "invalid",
			failedError:  nil,
			totalOutput:  "100",
			totalError:   nil,
			wantErr:      true,
		},
		{
			name:         "returns error on invalid total count",
			failedOutput: "0",
			failedError:  nil,
			totalOutput:  "invalid",
			totalError:   nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)

			// Set up mock expectations based on command content
			mockClient.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(cmd string) bool {
				return contains(cmd, "--failed")
			})).Return(tt.failedOutput, tt.failedError)

			mockClient.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(cmd string) bool {
				return contains(cmd, "list-units")
			})).Return(tt.totalOutput, tt.totalError)

			collector := &Collector{client: mockClient}
			metrics, err := collector.collectSystemd(context.Background())

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

func TestCollector_CollectOS(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		cmdError error
		wantOS   string
		wantErr  bool
	}{
		{
			name:     "parses valid OS info",
			output:   "Ubuntu 22.04.3 LTS",
			cmdError: nil,
			wantOS:   "Ubuntu 22.04.3 LTS",
			wantErr:  false,
		},
		{
			name:     "handles whitespace",
			output:   "  Debian GNU/Linux 11  ",
			cmdError: nil,
			wantOS:   "Debian GNU/Linux 11",
			wantErr:  false,
		},
		{
			name:     "handles empty output",
			output:   "",
			cmdError: nil,
			wantOS:   "",
			wantErr:  false,
		},
		{
			name:     "returns error on command failure",
			output:   "",
			cmdError: errors.New("connection failed"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockSSHClient)
			mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return(tt.output, tt.cmdError)

			collector := &Collector{client: mockClient}
			os, err := collector.collectOS(context.Background())

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
				mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return("Ubuntu 22.04", nil)
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
				mockClient.On("ExecuteCommand", mock.Anything, mock.Anything).Return("Ubuntu 22.04", nil)
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

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

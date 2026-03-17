package domain

import (
	"testing"
	"time"
)

func TestNode_IsAvailable(t *testing.T) {
	tests := []struct {
		name          string
		node          Node
		wantAvailable bool
	}{
		{
			name: "available when no error and connected",
			node: Node{
				Error:   "",
				Metrics: Metrics{Connectivity: true},
			},
			wantAvailable: true,
		},
		{
			name: "not available when has error even if connected",
			node: Node{
				Error:   "connection failed",
				Metrics: Metrics{Connectivity: true},
			},
			wantAvailable: false,
		},
		{
			name: "not available when not connected even if no error",
			node: Node{
				Error:   "",
				Metrics: Metrics{Connectivity: false},
			},
			wantAvailable: false,
		},
		{
			name: "not available when has error and not connected",
			node: Node{
				Error:   "connection failed",
				Metrics: Metrics{Connectivity: false},
			},
			wantAvailable: false,
		},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.IsAvailable()
			if got != tt.wantAvailable {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.wantAvailable)
			}
		})
	}
}

func TestNode_HasFailedUnits(t *testing.T) {
	tests := []struct {
		name          string
		node          Node
		wantHasFailed bool
	}{
		{
			name: "has failed when count is positive",
			node: Node{
				Metrics: Metrics{Systemd: SystemdMetrics{FailedCount: 5}},
			},
			wantHasFailed: true,
		},
		{
			name: "no failed when count is zero",
			node: Node{
				Metrics: Metrics{Systemd: SystemdMetrics{FailedCount: 0}},
			},
			wantHasFailed: false,
		},
		{
			name: "no failed when count is negative",
			node: Node{
				Metrics: Metrics{Systemd: SystemdMetrics{FailedCount: -1}},
			},
			wantHasFailed: false,
		},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.HasFailedUnits()
			if got != tt.wantHasFailed {
				t.Errorf("HasFailedUnits() = %v, want %v", got, tt.wantHasFailed)
			}
		})
	}
}

func TestNode_Fields(t *testing.T) {
	now := time.Now()
	node := Node{
		Name:        "test-server",
		IP:          "192.168.1.100",
		User:        "admin",
		SSHKeyPath:  "/home/user/.ssh/id_rsa",
		OSInfo:      "Ubuntu 22.04",
		Metrics:     Metrics{Connectivity: true},
		LastUpdated: now,
		Error:       "",
	}

	if node.Name != "test-server" {
		t.Errorf("Name = %v, want %v", node.Name, "test-server")
	}
	if node.IP != "192.168.1.100" {
		t.Errorf("IP = %v, want %v", node.IP, "192.168.1.100")
	}
	if node.User != "admin" {
		t.Errorf("User = %v, want %v", node.User, "admin")
	}
	if node.LastUpdated != now {
		t.Error("LastUpdated mismatch")
	}
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/justalternate/fleettui/internal/adapters/output/ssh"
	"github.com/justalternate/fleettui/internal/domain"
	"github.com/justalternate/fleettui/internal/mocks"
	"github.com/justalternate/fleettui/internal/ports/output"
	"github.com/stretchr/testify/mock"
)

func TestNewMetricsCollector(t *testing.T) {
	tests := []struct {
		name    string
		config  *domain.Config
		nodes   []*domain.Node
		factory CollectorFactory
		wantNil bool
	}{
		{
			name:   "creates collector with valid config and nodes",
			config: &domain.Config{RefreshRate: 5 * time.Second},
			nodes: []*domain.Node{
				{Name: "node1", IP: "192.168.1.1"},
				{Name: "node2", IP: "192.168.1.2"},
			},
			factory: nil,
			wantNil: false,
		},
		{
			name:    "creates collector with empty nodes",
			config:  &domain.Config{RefreshRate: 5 * time.Second},
			nodes:   []*domain.Node{},
			factory: nil,
			wantNil: false,
		},
		{
			name:    "creates collector with nil nodes",
			config:  &domain.Config{RefreshRate: 5 * time.Second},
			nodes:   nil,
			factory: nil,
			wantNil: false,
		},
		{
			name:    "creates collector with nil config",
			config:  nil,
			nodes:   []*domain.Node{},
			factory: nil,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := ssh.NewConnectionPool()
			collector := NewMetricsCollector(tt.config, tt.nodes, pool, tt.factory)

			if tt.wantNil && collector != nil {
				t.Error("expected nil collector, got non-nil")
			}
			if !tt.wantNil && collector == nil {
				t.Error("expected non-nil collector, got nil")
			}

			if collector != nil {
				if collector.config != tt.config {
					t.Error("config mismatch")
				}
				if len(collector.nodes) != len(tt.nodes) {
					t.Errorf("expected %d nodes, got %d", len(tt.nodes), len(collector.nodes))
				}
			}
		})
	}
}

func TestMetricsCollector_GetNodes(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []*domain.Node
		wantCount int
	}{
		{
			name: "returns copy of nodes",
			nodes: []*domain.Node{
				{Name: "node1", IP: "192.168.1.1", Metrics: domain.Metrics{Connectivity: true}},
				{Name: "node2", IP: "192.168.1.2", Metrics: domain.Metrics{Connectivity: false}},
			},
			wantCount: 2,
		},
		{
			name:      "returns empty slice for no nodes",
			nodes:     []*domain.Node{},
			wantCount: 0,
		},
		{
			name:      "returns empty slice for nil nodes",
			nodes:     nil,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := ssh.NewConnectionPool()
			mc := NewMetricsCollector(&domain.Config{}, tt.nodes, pool, nil)

			result := mc.GetNodes()

			if len(result) != tt.wantCount {
				t.Errorf("GetNodes() returned %d nodes, want %d", len(result), tt.wantCount)
			}

			if tt.wantCount > 0 {
				for i, node := range tt.nodes {
					if result[i].Name != node.Name || result[i].IP != node.IP {
						t.Errorf("GetNodes()[%d] = %v, want matching Name/IP", i, result[i])
					}
				}

				// Note: GetNodes() does a deep copy now.
				// Modifying result[i] will NOT modify the original node
				result[0].Name = "modified"
				if tt.nodes[0].Name == "modified" {
					t.Error("GetNodes() returned a shallow copy, expected deep copy")
				}
			}
		})
	}
}

func TestMetricsCollector_GetNodes_ThreadSafety(t *testing.T) {
	pool := ssh.NewConnectionPool()
	mc := NewMetricsCollector(&domain.Config{}, []*domain.Node{
		{Name: "node1", IP: "192.168.1.1"},
		{Name: "node2", IP: "192.168.1.2"},
	}, pool, nil)

	done := make(chan bool)
	results := make(chan []*domain.Node, 100)

	for i := 0; i < 100; i++ {
		go func() {
			nodes := mc.GetNodes()
			results <- nodes
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	close(results)
	count := 0
	for range results {
		count++
	}

	if count != 100 {
		t.Errorf("expected 100 results, got %d", count)
	}
}

func TestMetricsCollector_CollectAll_Success(t *testing.T) {
	nodes := []*domain.Node{
		{Name: "node1", IP: "192.168.1.1"},
		{Name: "node2", IP: "192.168.1.2"},
	}

	// Create mock collector factory that returns a mock for each node
	mockFactory := func(client output.SSHClient) output.MetricsCollector {
		mockCollector := new(mocks.MockMetricsCollector)
		mockCollector.On("CollectMetrics",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(&domain.Metrics{
			Connectivity: true,
			CPU:          domain.CPUMetrics{UsagePercent: 50.0},
		}, nil)
		return mockCollector
	}

	mc := NewMetricsCollector(
		&domain.Config{RefreshRate: 5 * time.Second},
		nodes,
		ssh.NewConnectionPool(),
		mockFactory,
	)

	mockClientFactory := func() output.SSHClient {
		mockClient := new(mocks.MockSSHClient)
		mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)
		mockClient.On("Disconnect").Return(nil)
		return mockClient
	}
	mc.pool.WithClientFactory(mockClientFactory)

	ctx := context.Background()
	mc.CollectAll(ctx)

	// Verify nodes were updated
	for _, node := range nodes {
		if node.Error != "" {
			t.Errorf("expected no error for node %s, got: %s", node.Name, node.Error)
		}
		if !node.Metrics.Connectivity {
			t.Errorf("expected node %s to be connected", node.Name)
		}
		if node.LastUpdated.IsZero() {
			t.Errorf("expected LastUpdated to be set for node %s", node.Name)
		}
	}
}

func TestMetricsCollector_CollectAll_WithErrors(t *testing.T) {
	nodes := []*domain.Node{
		{Name: "node1", IP: "192.168.1.1"},
	}

	testError := errors.New("connection failed")

	mockFactory := func(client output.SSHClient) output.MetricsCollector {
		mockCollector := new(mocks.MockMetricsCollector)
		mockCollector.On("CollectMetrics",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil, testError)
		return mockCollector
	}

	mc := NewMetricsCollector(
		&domain.Config{RefreshRate: 5 * time.Second},
		nodes,
		ssh.NewConnectionPool(),
		mockFactory,
	)

	mockClientFactory := func() output.SSHClient {
		mockClient := new(mocks.MockSSHClient)
		mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)
		mockClient.On("Disconnect").Return(nil)
		return mockClient
	}
	mc.pool.WithClientFactory(mockClientFactory)

	ctx := context.Background()
	mc.CollectAll(ctx)

	if nodes[0].Error != testError.Error() {
		t.Errorf("expected error %q, got %q", testError.Error(), nodes[0].Error)
	}
	if nodes[0].Metrics.Connectivity {
		t.Error("expected node to be disconnected after error")
	}
}

func TestMetricsCollector_CollectAll_NoFactory(t *testing.T) {
	nodes := []*domain.Node{
		{Name: "node1", IP: "192.168.1.1"},
	}

	mc := NewMetricsCollector(
		&domain.Config{RefreshRate: 5 * time.Second},
		nodes,
		ssh.NewConnectionPool(),
		nil, // No factory provided
	)
	mockClientFactory := func() output.SSHClient {
		mockClient := new(mocks.MockSSHClient)
		mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)
		mockClient.On("Disconnect").Return(nil)
		return mockClient
	}
	mc.pool.WithClientFactory(mockClientFactory)

	ctx := context.Background()
	mc.CollectAll(ctx)

	if nodes[0].Error != "collector factory not configured" {
		t.Errorf("expected factory not configured error, got: %s", nodes[0].Error)
	}
	if nodes[0].Metrics.Connectivity {
		t.Error("expected node to be disconnected when factory missing")
	}
}

func TestMetricsCollector_CollectAll_EmptyNodes(t *testing.T) {
	callCount := 0
	mockFactory := func(client output.SSHClient) output.MetricsCollector {
		callCount++
		return new(mocks.MockMetricsCollector)
	}

	mc := NewMetricsCollector(
		&domain.Config{RefreshRate: 5 * time.Second},
		[]*domain.Node{}, // Empty nodes
		ssh.NewConnectionPool(),
		mockFactory,
	)
	mockClientFactory := func() output.SSHClient {
		mockClient := new(mocks.MockSSHClient)
		mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)
		mockClient.On("Disconnect").Return(nil)
		return mockClient
	}
	mc.pool.WithClientFactory(mockClientFactory)

	ctx := context.Background()
	mc.CollectAll(ctx)

	if callCount != 0 {
		t.Errorf("expected factory to not be called for empty nodes, got %d calls", callCount)
	}
}

func TestMetricsCollector_Start(t *testing.T) {
	tests := []struct {
		name        string
		refreshRate time.Duration
		cancelAfter time.Duration
	}{
		{
			name:        "starts and stops via context cancellation",
			refreshRate: 100 * time.Millisecond,
			cancelAfter: 250 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := NewMetricsCollector(
				&domain.Config{RefreshRate: tt.refreshRate},
				[]*domain.Node{},
				ssh.NewConnectionPool(),
				nil,
			)

			ctx, cancel := context.WithTimeout(context.Background(), tt.cancelAfter)
			defer cancel()

			done := make(chan bool)
			go func() {
				mc.Start(ctx)
				done <- true
			}()

			select {
			case <-done:
				// Success - Start stopped when context was cancelled
			case <-time.After(tt.cancelAfter + 500*time.Millisecond):
				t.Error("Start did not stop after context cancellation")
			}
		})
	}
}

func TestMetricsCollector_CollectAll_Concurrent(t *testing.T) {
	nodes := make([]*domain.Node, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = &domain.Node{
			Name:       "node",
			IP:         "192.168.1.1",
			User:       "test",
			SSHKeyPath: "/nonexistent",
		}
	}

	mockFactory := func(client output.SSHClient) output.MetricsCollector {
		mockCollector := new(mocks.MockMetricsCollector)
		mockCollector.On("CollectMetrics",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(&domain.Metrics{Connectivity: true}, nil)
		return mockCollector
	}

	mc := NewMetricsCollector(
		&domain.Config{RefreshRate: 5 * time.Second},
		nodes,
		ssh.NewConnectionPool(),
		mockFactory,
	)

	mockClientFactory := func() output.SSHClient {
		mockClient := new(mocks.MockSSHClient)
		mockClient.On("Connect", mock.Anything, mock.Anything).Return(nil)
		mockClient.On("Disconnect").Return(nil)
		return mockClient
	}
	mc.pool.WithClientFactory(mockClientFactory)

	start := time.Now()
	ctx := context.Background()

	mc.CollectAll(ctx)

	duration := time.Since(start)

	// With mocks, should complete quickly (concurrently)
	if duration > 2*time.Second {
		t.Errorf("CollectAll took %v, expected faster concurrent execution", duration)
	}

	// Verify all nodes were updated
	for i, node := range nodes {
		if node.LastUpdated.IsZero() {
			t.Errorf("node %d was not updated", i)
		}
	}
}

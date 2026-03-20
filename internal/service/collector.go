package service

import (
	"context"
	"sync"
	"time"

	"github.com/justalternate/fleetui/internal/adapters/output/ssh"
	"github.com/justalternate/fleetui/internal/domain"
	"github.com/justalternate/fleetui/internal/ports/output"
)

// CollectorFactory creates a MetricsCollector using the provided SSHClient
type CollectorFactory func(client output.SSHClient) output.MetricsCollector

type MetricsCollector struct {
	config           *domain.Config
	nodes            []*domain.Node
	pool             *ssh.ConnectionPool
	collectorFactory CollectorFactory
	mu               sync.RWMutex
}

func NewMetricsCollector(config *domain.Config, nodes []*domain.Node, pool *ssh.ConnectionPool, collectorFactory CollectorFactory) *MetricsCollector {
	return &MetricsCollector{
		config:           config,
		nodes:            nodes,
		pool:             pool,
		collectorFactory: collectorFactory,
	}
}

func (mc *MetricsCollector) CollectAll(ctx context.Context) {
	// Create timeout context for entire collection cycle
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for _, node := range mc.nodes {
		wg.Add(1)
		go func(n *domain.Node) {
			defer wg.Done()
			mc.collectNode(ctx, n)
		}(node)
	}

	wg.Wait()
}

func (mc *MetricsCollector) collectNode(ctx context.Context, node *domain.Node) {
	// Check if node is in backoff
	if mc.pool.IsInBackoff(node.IP) {
		node.Mu.Lock()
		node.Error = "Connection failed, backing off..."
		node.Metrics.Connectivity = false
		node.LastUpdated = time.Now()
		node.Mu.Unlock()
		return
	}

	var collector output.MetricsCollector

	if mc.collectorFactory == nil {
		node.Mu.Lock()
		node.Error = "collector factory not configured"
		node.Metrics.Connectivity = false
		node.LastUpdated = time.Now()
		node.Mu.Unlock()
		return
	}

	client, err := mc.pool.Get(ctx, node)
	if err != nil {
		mc.pool.RecordFailure(node.IP)
		node.Mu.Lock()
		node.Error = err.Error()
		node.Metrics.Connectivity = false
		node.LastUpdated = time.Now()
		node.Mu.Unlock()
		return
	}
	defer mc.pool.Return(node.IP)

	collector = mc.collectorFactory(client)

	metrics, err := collector.CollectMetrics(ctx, node, mc.config)

	node.Mu.Lock()
	defer node.Mu.Unlock()

	if err != nil {
		mc.pool.RecordFailure(node.IP)
		node.Error = err.Error()
		node.Metrics.Connectivity = false
	} else {
		mc.pool.RecordSuccess(node.IP)
		node.Error = ""
		node.Metrics = *metrics
	}

	node.LastUpdated = time.Now()
}

func (mc *MetricsCollector) GetNodes() []*domain.Node {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make([]*domain.Node, len(mc.nodes))
	for i, n := range mc.nodes {
		n.Mu.RLock()
		copyNode := &domain.Node{
			Name:        n.Name,
			IP:          n.IP,
			User:        n.User,
			SSHKeyPath:  n.SSHKeyPath,
			OSInfo:      n.OSInfo,
			Metrics:     n.Metrics,
			LastUpdated: n.LastUpdated,
			Error:       n.Error,
		}
		n.Mu.RUnlock()
		result[i] = copyNode
	}
	return result
}

func (mc *MetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(mc.config.RefreshRate)
	defer ticker.Stop()

	// Cleanup idle connections periodically
	cleanupTicker := time.NewTicker(1 * time.Minute)
	defer cleanupTicker.Stop()

	mc.CollectAll(ctx)

	for {
		select {
		case <-ctx.Done():
			_ = mc.pool.CloseAll()
			return
		case <-ticker.C:
			mc.CollectAll(ctx)
		case <-cleanupTicker.C:
			mc.pool.CleanupIdle()
		}
	}
}

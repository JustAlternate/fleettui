package service

import (
	"context"
	"sync"
	"time"

	"fleettui/internal/adapters/output/ssh"
	"fleettui/internal/domain"
	"fleettui/internal/ports/output"
)

// CollectorFactory creates a MetricsCollector for a specific node
type CollectorFactory func(node *domain.Node) output.MetricsCollector

type MetricsCollector struct {
	config           *domain.Config
	nodes            []*domain.Node
	pool             *ssh.ConnectionPool
	collectorFactory CollectorFactory
	mu               sync.RWMutex
}

func NewMetricsCollector(config *domain.Config, nodes []*domain.Node, collectorFactory CollectorFactory) *MetricsCollector {
	return &MetricsCollector{
		config:           config,
		nodes:            nodes,
		pool:             ssh.NewConnectionPool(),
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
		node.Error = "Connection failed, backing off..."
		node.Metrics.Connectivity = false
		node.LastUpdated = time.Now()
		return
	}

	var collector output.MetricsCollector

	if mc.collectorFactory == nil {
		node.Error = "collector factory not configured"
		node.Metrics.Connectivity = false
		node.LastUpdated = time.Now()
		return
	}

	collector = mc.collectorFactory(node)

	metrics, err := collector.CollectMetrics(ctx, node, mc.config)
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
	copy(result, mc.nodes)
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
			mc.pool.CloseAll()
			return
		case <-ticker.C:
			mc.CollectAll(ctx)
		case <-cleanupTicker.C:
			mc.pool.CleanupIdle()
		}
	}
}

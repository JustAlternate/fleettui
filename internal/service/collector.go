package service

import (
	"context"
	"sync"
	"time"

	"fleettui/internal/adapters/output/ssh"
	"fleettui/internal/domain"
	"fleettui/internal/ports/output"
)

type MetricsCollector struct {
	config    *domain.Config
	nodes     []*domain.Node
	collector output.MetricsCollector
	mu        sync.RWMutex
}

func NewMetricsCollector(config *domain.Config, nodes []*domain.Node, collector output.MetricsCollector) *MetricsCollector {
	return &MetricsCollector{
		config:    config,
		nodes:     nodes,
		collector: collector,
	}
}

func (mc *MetricsCollector) CollectAll(ctx context.Context) {
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
	client := ssh.NewClient()
	collector := ssh.NewCollector(client)

	metrics, err := collector.CollectMetrics(ctx, node, mc.config)
	if err != nil {
		node.Error = err.Error()
		node.Metrics.Connectivity = false
	} else {
		node.Error = ""
		node.Metrics = *metrics
	}

	node.LastUpdated = time.Now()
	client.Disconnect()
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

	mc.CollectAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.CollectAll(ctx)
		}
	}
}

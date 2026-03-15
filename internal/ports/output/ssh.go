package output

import (
	"context"
	"fleettui/internal/domain"
)

type SSHClient interface {
	Connect(ctx context.Context, node *domain.Node) error
	Disconnect() error
	ExecuteCommand(ctx context.Context, command string) (string, error)
	IsConnected() bool
}

type MetricsCollector interface {
	CollectMetrics(ctx context.Context, node *domain.Node, config *domain.Config) (*domain.Metrics, error)
}

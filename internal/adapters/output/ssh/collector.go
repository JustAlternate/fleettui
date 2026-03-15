package ssh

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fleettui/internal/domain"
	"fleettui/internal/ports/output"
)

type Collector struct {
	client output.SSHClient
}

func NewCollector(client output.SSHClient) output.MetricsCollector {
	return &Collector{client: client}
}

func (c *Collector) CollectMetrics(ctx context.Context, node *domain.Node, config *domain.Config) (*domain.Metrics, error) {
	metrics := &domain.Metrics{}

	// Always check connectivity first (required for SSH)
	metrics.Connectivity = c.checkConnectivity(ctx, node)

	if !metrics.Connectivity {
		return metrics, nil
	}

	if config.IsMetricEnabled(domain.MetricOS) {
		os, err := c.collectOS(ctx)
		if err == nil {
			node.OSInfo = os
		}
	}

	if config.IsMetricEnabled(domain.MetricCPU) {
		cpu, err := c.collectCPU(ctx)
		if err == nil {
			metrics.CPU = cpu
		}
	}

	if config.IsMetricEnabled(domain.MetricRAM) {
		ram, err := c.collectRAM(ctx)
		if err == nil {
			metrics.RAM = ram
		}
	}

	if config.IsMetricEnabled(domain.MetricNetwork) {
		net, err := c.collectNetwork(ctx, node)
		if err == nil {
			metrics.Network = net
		}
	}

	if config.IsMetricEnabled(domain.MetricUptime) {
		uptime, err := c.collectUptime(ctx)
		if err == nil {
			metrics.Uptime = uptime
		}
	}

	if config.IsMetricEnabled(domain.MetricSystemd) {
		systemd, err := c.collectSystemd(ctx)
		if err == nil {
			metrics.Systemd = systemd
		}
	}

	return metrics, nil
}

func (c *Collector) collectOS(ctx context.Context) (string, error) {
	output, err := c.client.ExecuteCommand(ctx, "cat /etc/os-release | grep PRETTY_NAME | cut -d'=' -f2 | tr -d '\"'")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func (c *Collector) checkConnectivity(ctx context.Context, node *domain.Node) bool {
	err := c.client.Connect(ctx, node)
	if err != nil {
		return false
	}
	_, err = c.client.ExecuteCommand(ctx, "echo 'ping'")
	return err == nil
}

func (c *Collector) collectCPU(ctx context.Context) (domain.CPUMetrics, error) {
	output, err := c.client.ExecuteCommand(ctx, "top -bn1 | grep 'Cpu(s)' | awk '{print $2}' | cut -d'%' -f1")
	if err != nil {
		return domain.CPUMetrics{}, err
	}

	usage, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return domain.CPUMetrics{}, err
	}

	return domain.CPUMetrics{UsagePercent: usage}, nil
}

func (c *Collector) collectRAM(ctx context.Context) (domain.RAMMetrics, error) {
	output, err := c.client.ExecuteCommand(ctx, "free -b | grep Mem | awk '{print $2, $3}'")
	if err != nil {
		return domain.RAMMetrics{}, err
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 2 {
		return domain.RAMMetrics{}, fmt.Errorf("unexpected output format")
	}

	total, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return domain.RAMMetrics{}, err
	}

	used, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return domain.RAMMetrics{}, err
	}

	usagePercent := float64(used) / float64(total) * 100

	return domain.RAMMetrics{
		UsagePercent: usagePercent,
		UsedBytes:    used,
		TotalBytes:   total,
	}, nil
}

func (c *Collector) collectNetwork(ctx context.Context, node *domain.Node) (domain.NetworkMetrics, error) {
	client := c.client.(*Client)

	output, err := c.client.ExecuteCommand(ctx, "cat /proc/net/dev | grep -E '^(\\s*\\w+):' | awk '{print $1, $2, $10}' | head -1")
	if err != nil {
		return domain.NetworkMetrics{}, err
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 3 {
		return domain.NetworkMetrics{}, fmt.Errorf("unexpected output format")
	}

	rxBytes, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return domain.NetworkMetrics{}, err
	}

	txBytes, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return domain.NetworkMetrics{}, err
	}

	now := time.Now()

	if client.lastNet != nil {
		deltaTime := now.Sub(client.lastNet.timestamp).Seconds()
		if deltaTime > 0 {
			rxRate := float64(rxBytes-client.lastNet.rxBytes) / deltaTime / 1024 / 1024
			txRate := float64(txBytes-client.lastNet.txBytes) / deltaTime / 1024 / 1024

			client.lastNet = &netStats{rxBytes: rxBytes, txBytes: txBytes, timestamp: now}

			return domain.NetworkMetrics{
				InRateMBps:  rxRate,
				OutRateMBps: txRate,
			}, nil
		}
	}

	client.lastNet = &netStats{rxBytes: rxBytes, txBytes: txBytes, timestamp: now}
	return domain.NetworkMetrics{InRateMBps: 0, OutRateMBps: 0}, nil
}

func (c *Collector) collectUptime(ctx context.Context) (time.Duration, error) {
	output, err := c.client.ExecuteCommand(ctx, "cat /proc/uptime | awk '{print $1}'")
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return 0, err
	}

	return time.Duration(seconds) * time.Second, nil
}

func (c *Collector) collectSystemd(ctx context.Context) (domain.SystemdMetrics, error) {
	failedOutput, err := c.client.ExecuteCommand(ctx, "systemctl --failed --no-legend 2>/dev/null | wc -l")
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	failedCount, err := strconv.Atoi(strings.TrimSpace(failedOutput))
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	totalOutput, err := c.client.ExecuteCommand(ctx, "systemctl list-units --type=service --no-legend 2>/dev/null | wc -l")
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	totalCount, err := strconv.Atoi(strings.TrimSpace(totalOutput))
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	return domain.SystemdMetrics{
		FailedCount: failedCount,
		TotalCount:  totalCount,
	}, nil
}

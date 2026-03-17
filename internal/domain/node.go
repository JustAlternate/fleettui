package domain

import (
	"sync"
	"time"
)

type Node struct {
	Mu          sync.RWMutex
	Name        string
	IP          string
	User        string
	SSHKeyPath  string
	OSInfo      string
	Metrics     Metrics
	LastUpdated time.Time
	Error       string
}

type Metrics struct {
	Connectivity bool
	CPU          CPUMetrics
	RAM          RAMMetrics
	Network      NetworkMetrics
	Uptime       time.Duration
	Systemd      SystemdMetrics
}

type CPUMetrics struct {
	UsagePercent float64
}

type RAMMetrics struct {
	UsagePercent float64
	UsedBytes    uint64
	TotalBytes   uint64
}

type NetworkMetrics struct {
	InRateMBps  float64
	OutRateMBps float64
}

type SystemdMetrics struct {
	FailedCount int
	TotalCount  int
}

func (n *Node) IsAvailable() bool {
	return n.Error == "" && n.Metrics.Connectivity
}

func (n *Node) IsPending() bool {
	return n.LastUpdated.IsZero()
}

func (n *Node) HasFailedUnits() bool {
	return n.Metrics.Systemd.FailedCount > 0
}

package ssh

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/justalternate/fleettui/internal/domain"
	"github.com/justalternate/fleettui/internal/ports/output"
)

type Collector struct {
	client output.SSHClient
}

func NewCollector(client output.SSHClient) output.MetricsCollector {
	return &Collector{client: client}
}

func (c *Collector) CollectMetrics(ctx context.Context, node *domain.Node, config *domain.Config) (*domain.Metrics, error) {
	metrics := &domain.Metrics{}

	if err := c.client.Connect(ctx, node); err != nil {
		return metrics, err
	}

	metrics.Connectivity = true

	// Build the script
	var cmds []string

	if config.IsMetricEnabled(domain.MetricOS) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_OS---'", "cat /etc/os-release 2>/dev/null || true")
	}
	if config.IsMetricEnabled(domain.MetricCPU) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_CPU---'", "grep '^cpu ' /proc/stat 2>/dev/null || true", "echo '---FLEETTUI_SEP_CORES---'", "grep -c '^processor' /proc/cpuinfo 2>/dev/null || true", "echo '---FLEETTUI_SEP_LOAD---'", "awk '{print $1}' /proc/loadavg 2>/dev/null || true")
	}
	if config.IsMetricEnabled(domain.MetricRAM) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_RAM---'", "cat /proc/meminfo 2>/dev/null || true")
	}
	if config.IsMetricEnabled(domain.MetricNetwork) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_NET---'", "INTF=$(ip route show default 2>/dev/null | awk '{print $5}' | head -n1)", "if [ -n \"$INTF\" ]; then grep \"$INTF:\" /proc/net/dev 2>/dev/null || true; fi")
	}
	if config.IsMetricEnabled(domain.MetricUptime) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_UPTIME---'", "cat /proc/uptime 2>/dev/null || true")
	}
	if config.IsMetricEnabled(domain.MetricSystemd) {
		cmds = append(cmds, "echo '---FLEETTUI_SEP_SYSTEMD---'", "systemctl --failed --no-legend 2>/dev/null | wc -l || true", "systemctl list-units --type=service --no-legend 2>/dev/null | wc -l || true")
	}

	if len(cmds) == 0 {
		return metrics, nil
	}

	script := strings.Join(cmds, " ; ")
	outputStr, err := c.client.ExecuteCommand(ctx, script)
	if err != nil {
		return metrics, err
	}

	success := false

	if config.IsMetricEnabled(domain.MetricOS) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_OS---"); out != "" {
			if osName, err := c.parseOS(out); err == nil {
				node.Mu.Lock()
				node.OSInfo = osName
				node.Mu.Unlock()
				success = true
			}
		}
	}

	if config.IsMetricEnabled(domain.MetricCPU) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_CPU---"); out != "" {
			if cpu, err := c.parseCPU(out); err == nil {
				metrics.CPU = cpu
				success = true
			}
		}
		if out := extractSection(outputStr, "---FLEETTUI_SEP_CORES---"); out != "" {
			if cores, err := c.parseCores(out); err == nil {
				metrics.CPU.Cores = cores
			}
		}
		if out := extractSection(outputStr, "---FLEETTUI_SEP_LOAD---"); out != "" {
			if load, err := c.parseLoadAvg(out); err == nil {
				metrics.CPU.LoadAvg = load
			}
		}
	}

	if config.IsMetricEnabled(domain.MetricRAM) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_RAM---"); out != "" {
			if ram, err := c.parseRAM(out); err == nil {
				metrics.RAM = ram
				success = true
			}
		}
	}

	if config.IsMetricEnabled(domain.MetricNetwork) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_NET---"); out != "" {
			if net, err := c.parseNetwork(out, node); err == nil {
				metrics.Network = net
				success = true
			}
		}
	}

	if config.IsMetricEnabled(domain.MetricUptime) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_UPTIME---"); out != "" {
			if uptime, err := c.parseUptime(out); err == nil {
				metrics.Uptime = uptime
				success = true
			}
		}
	}

	if config.IsMetricEnabled(domain.MetricSystemd) {
		if out := extractSection(outputStr, "---FLEETTUI_SEP_SYSTEMD---"); out != "" {
			if systemd, err := c.parseSystemd(out); err == nil {
				metrics.Systemd = systemd
				success = true
			}
		}
	}

	if !success {
		metrics.Connectivity = false
		return metrics, fmt.Errorf("failed to parse any SSH metrics")
	}

	return metrics, nil
}

func extractSection(output, sectionHeader string) string {
	idx := strings.Index(output, sectionHeader)
	if idx == -1 {
		return ""
	}
	start := idx + len(sectionHeader)
	// Find the next section header
	end := strings.Index(output[start:], "---FLEETTUI_SEP_")
	if end == -1 {
		return strings.TrimSpace(output[start:])
	}
	return strings.TrimSpace(output[start : start+end])
}

func (c *Collector) parseOS(output string) (string, error) {
	if output == "" {
		return "", fmt.Errorf("empty OS output")
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			val = strings.Trim(val, `"'`)
			return strings.TrimSpace(val), nil
		}
	}
	return "", fmt.Errorf("PRETTY_NAME not found")
}

func (c *Collector) parseCPU(output string) (domain.CPUMetrics, error) {
	if output == "" {
		return domain.CPUMetrics{}, fmt.Errorf("empty CPU output")
	}
	// Output: cpu  123 34 1234 12345 12 ...
	fields := strings.Fields(output)
	if len(fields) < 5 {
		return domain.CPUMetrics{}, fmt.Errorf("invalid cpu format")
	}

	var user, nice, system, idle, iowait, irq, softirq, steal uint64
	var err error

	parse := func(s string) uint64 {
		v, e := strconv.ParseUint(s, 10, 64)
		if e != nil {
			err = e
		}
		return v
	}

	user = parse(fields[1])
	nice = parse(fields[2])
	system = parse(fields[3])
	idle = parse(fields[4])
	if len(fields) > 5 {
		iowait = parse(fields[5])
	}
	if len(fields) > 6 {
		irq = parse(fields[6])
	}
	if len(fields) > 7 {
		softirq = parse(fields[7])
	}
	if len(fields) > 8 {
		steal = parse(fields[8])
	}

	if err != nil {
		return domain.CPUMetrics{}, err
	}

	idleTime := idle + iowait
	totalTime := user + nice + system + idleTime + irq + softirq + steal

	var usagePercent float64

	client, ok := c.client.(*Client)
	if ok {
		if client.lastCPU != nil {
			deltaTotal := totalTime - client.lastCPU.total
			deltaIdle := idleTime - client.lastCPU.idle
			if deltaTotal > 0 {
				usagePercent = (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
			}
		}
		client.lastCPU = &cpuStats{total: totalTime, idle: idleTime}
	} else {
		// Fallback without state
		usagePercent = (1.0 - float64(idleTime)/float64(totalTime)) * 100.0
	}

	return domain.CPUMetrics{UsagePercent: usagePercent}, nil
}

func (c *Collector) parseCores(output string) (uint, error) {
	if output == "" {
		return 0, fmt.Errorf("empty cores output")
	}
	cores, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(cores), nil
}

func (c *Collector) parseLoadAvg(output string) (float64, error) {
	if output == "" {
		return 0, fmt.Errorf("empty load avg output")
	}
	load, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return 0, err
	}
	return load, nil
}

func (c *Collector) parseRAM(output string) (domain.RAMMetrics, error) {
	if output == "" {
		return domain.RAMMetrics{}, fmt.Errorf("empty RAM output")
	}
	var total, available uint64
	var err error

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return domain.RAMMetrics{}, err
			}
			total *= 1024 // KB to Bytes
		case "MemAvailable:":
			available, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return domain.RAMMetrics{}, err
			}
			available *= 1024
		}
	}

	if total == 0 {
		return domain.RAMMetrics{}, fmt.Errorf("MemTotal not found")
	}

	used := total - available
	usagePercent := float64(used) / float64(total) * 100

	return domain.RAMMetrics{
		UsagePercent: usagePercent,
		UsedBytes:    used,
		TotalBytes:   total,
	}, nil
}

func (c *Collector) parseNetwork(output string, node *domain.Node) (domain.NetworkMetrics, error) {
	if output == "" {
		return domain.NetworkMetrics{}, fmt.Errorf("empty network output")
	}
	// Output like: eth0: 123456 123 0 0 0 0 0 0 654321 321 ...
	idx := strings.Index(output, ":")
	if idx == -1 {
		return domain.NetworkMetrics{}, fmt.Errorf("invalid network output format")
	}

	fields := strings.Fields(output[idx+1:])
	if len(fields) < 9 {
		return domain.NetworkMetrics{}, fmt.Errorf("insufficient network fields")
	}

	rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return domain.NetworkMetrics{}, err
	}

	txBytes, err := strconv.ParseUint(fields[8], 10, 64)
	if err != nil {
		return domain.NetworkMetrics{}, err
	}

	now := time.Now()
	var inRateMBps, outRateMBps float64

	client, ok := c.client.(*Client)
	if ok {
		if client.lastNet != nil {
			deltaTime := now.Sub(client.lastNet.timestamp).Seconds()
			if deltaTime > 0 {
				inRateMBps = float64(rxBytes-client.lastNet.rxBytes) / deltaTime / 1024 / 1024
				outRateMBps = float64(txBytes-client.lastNet.txBytes) / deltaTime / 1024 / 1024
			}
		}
		client.lastNet = &netStats{rxBytes: rxBytes, txBytes: txBytes, timestamp: now}
	}

	return domain.NetworkMetrics{
		InRateMBps:  inRateMBps,
		OutRateMBps: outRateMBps,
	}, nil
}

func (c *Collector) parseUptime(output string) (time.Duration, error) {
	if output == "" {
		return 0, fmt.Errorf("empty uptime output")
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid uptime format")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func (c *Collector) parseSystemd(output string) (domain.SystemdMetrics, error) {
	if output == "" {
		return domain.SystemdMetrics{}, fmt.Errorf("empty systemd output")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return domain.SystemdMetrics{}, fmt.Errorf("insufficient systemd fields")
	}

	failedCount, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	totalCount, err := strconv.Atoi(strings.TrimSpace(lines[1]))
	if err != nil {
		return domain.SystemdMetrics{}, err
	}

	return domain.SystemdMetrics{
		FailedCount: failedCount,
		TotalCount:  totalCount,
	}, nil
}

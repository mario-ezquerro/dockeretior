package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
)

// ContainerMetrics holds detailed real-time statistics for a container.
type ContainerMetrics struct {
	ContainerID   string
	Name          string
	Image         string
	State         string
	Status        string
	IsRunning     bool
	CPUPercent    float64
	OnlineCPUs    int
	CPUSparkline  []float64
	SystemDeltaMs float64
	MemUsage      uint64
	MemLimit      uint64
	MemPercent    float64
	MemCache      uint64
	MemMaxUsage   uint64
	NetRxBytes    uint64
	NetTxBytes    uint64
	NetRxPackets  uint64
	NetTxPackets  uint64
	BlkRead       uint64
	BlkWrite      uint64
	PIDs          uint32
	IPAddress     string
	NetworkName   string
	Ports         string
	Uptime        string
	RestartCount  int
	LastUpdated   time.Time
	ErrorMessage  string
}

// MetricsTracker keeps historical sparkline points for containers.
type MetricsTracker struct {
	mu      sync.RWMutex
	history map[string][]float64
}

var globalTracker = &MetricsTracker{
	history: make(map[string][]float64),
}

func (t *MetricsTracker) RecordCPU(id string, val float64) []float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	pts := t.history[id]
	pts = append(pts, val)
	if len(pts) > 20 {
		pts = pts[len(pts)-20:]
	}
	t.history[id] = pts

	// Return a copy
	res := make([]float64, len(pts))
	copy(res, pts)
	return res
}

// FetchContainerMetrics retrieves and parses one-shot metrics for a container.
func (c *Client) FetchContainerMetrics(ctx context.Context, cInfo types.Container) (*ContainerMetrics, error) {
	name := "unnamed"
	if len(cInfo.Names) > 0 {
		name = strings.TrimPrefix(cInfo.Names[0], "/")
	}

	ipAddr := "-"
	netName := "-"
	if cInfo.NetworkSettings != nil {
		for nName, nConf := range cInfo.NetworkSettings.Networks {
			netName = nName
			if nConf.IPAddress != "" {
				ipAddr = nConf.IPAddress
				break
			}
		}
	}

	var ports []string
	for _, p := range cInfo.Ports {
		if p.PublicPort > 0 {
			ports = append(ports, fmt.Sprintf("%d->%d", p.PublicPort, p.PrivatePort))
		}
	}
	portsStr := strings.Join(ports, ", ")
	if portsStr == "" {
		portsStr = "-"
	}

	metrics := &ContainerMetrics{
		ContainerID: cInfo.ID,
		Name:        name,
		Image:       cInfo.Image,
		State:       cInfo.State,
		Status:      cInfo.Status,
		IsRunning:   strings.ToLower(cInfo.State) == "running",
		IPAddress:   ipAddr,
		NetworkName: netName,
		Ports:       portsStr,
		Uptime:      cInfo.Status,
		LastUpdated: time.Now(),
	}

	if !metrics.IsRunning {
		metrics.CPUSparkline = globalTracker.RecordCPU(cInfo.ID, 0.0)
		return metrics, nil
	}

	// For running containers, fetch one-shot stats from the engine
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	statsResp, err := c.cli.ContainerStatsOneShot(reqCtx, cInfo.ID)
	if err != nil {
		metrics.ErrorMessage = err.Error()
		metrics.CPUSparkline = globalTracker.RecordCPU(cInfo.ID, 0.0)
		return metrics, err
	}
	defer statsResp.Body.Close()

	var stats types.StatsJSON
	if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
		metrics.ErrorMessage = err.Error()
		return metrics, err
	}

	// 1. CPU calculation
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	onlineCPUs := int(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = len(stats.CPUStats.CPUUsage.PercpuUsage)
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}
	metrics.OnlineCPUs = onlineCPUs

	var cpuPercent float64
	if sysDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / sysDelta) * float64(onlineCPUs) * 100.0
		if cpuPercent > float64(onlineCPUs)*100.0 {
			cpuPercent = float64(onlineCPUs) * 100.0
		}
	}
	metrics.CPUPercent = cpuPercent
	metrics.CPUSparkline = globalTracker.RecordCPU(cInfo.ID, cpuPercent)
	if sysDelta > 0 {
		metrics.SystemDeltaMs = sysDelta / 1e6
	}

	// 2. Memory calculation
	memUsage := stats.MemoryStats.Usage
	if v, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok && memUsage > v {
		memUsage -= v
		metrics.MemCache = v
	} else if v, ok := stats.MemoryStats.Stats["inactive_file"]; ok && memUsage > v {
		memUsage -= v
		metrics.MemCache = v
	}
	metrics.MemUsage = memUsage
	metrics.MemLimit = stats.MemoryStats.Limit
	metrics.MemMaxUsage = stats.MemoryStats.MaxUsage
	if metrics.MemLimit > 0 {
		metrics.MemPercent = (float64(metrics.MemUsage) / float64(metrics.MemLimit)) * 100.0
	}

	// 3. Network I/O calculation
	for _, net := range stats.Networks {
		metrics.NetRxBytes += net.RxBytes
		metrics.NetTxBytes += net.TxBytes
		metrics.NetRxPackets += net.RxPackets
		metrics.NetTxPackets += net.TxPackets
	}

	// 4. Block I/O & PIDs
	for _, bio := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(bio.Op) {
		case "read":
			metrics.BlkRead += bio.Value
		case "write":
			metrics.BlkWrite += bio.Value
		}
	}
	metrics.PIDs = uint32(stats.PidsStats.Current)

	return metrics, nil
}

// FormatBytes formats a byte count into human-readable B, KB, MB, GB.
func FormatBytes(b uint64) string {
	const unit = 1024.0
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	prefix := []string{"KB", "MB", "GB", "TB"}[exp]
	return fmt.Sprintf("%.2f %s", float64(b)/float64(div), prefix)
}

// FormatPackets formats packet numbers with thousand separators.
func FormatPackets(p uint64) string {
	if p < 1000 {
		return fmt.Sprintf("%d pkts", p)
	}
	return fmt.Sprintf("%.1fk pkts", float64(p)/1000.0)
}

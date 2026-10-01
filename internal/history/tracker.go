package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Snapshot represents a point-in-time metrics sample.
type Snapshot struct {
	Timestamp      time.Time                  `json:"timestamp"`
	DockerBytes    int64                      `json:"docker_bytes"`
	Containers     map[string]ContainerSample `json:"containers"` // name -> sample
}

// ContainerSample tracks key metrics for trend detection.
type ContainerSample struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	MemoryBytes  uint64  `json:"memory_bytes"`
	CPUPercent   float64 `json:"cpu_percent"`
	RestartCount int     `json:"restart_count"`
	State        string  `json:"state"`
}

// TrendFinding highlights significant changes over time.
type TrendFinding struct {
	Target      string    `json:"target"`
	Metric      string    `json:"metric"`
	ChangeText  string    `json:"change_text"`
	OldValue    string    `json:"old_value"`
	NewValue    string    `json:"new_value"`
	IsWarning   bool      `json:"is_warning"`
	DetectedAt  time.Time `json:"detected_at"`
}

// HistoryStore persists and analyzes past snapshots.
type HistoryStore struct {
	mu        sync.RWMutex
	filePath  string
	Snapshots []Snapshot `json:"snapshots"`
}

// DefaultHistoryPath returns the path to the persistent history file.
func DefaultHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".dockeretior")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "metrics_history.json")
}

// LoadHistory initializes the store from disk.
func LoadHistory(customPath string) *HistoryStore {
	path := customPath
	if path == "" {
		path = DefaultHistoryPath()
	}

	hs := &HistoryStore{
		filePath: path,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &hs.Snapshots)
	}

	return hs
}

// RecordSnapshot appends a new snapshot and prunes older ones (keeping last 500 samples).
func (hs *HistoryStore) RecordSnapshot(snap Snapshot) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.Snapshots = append(hs.Snapshots, snap)

	// Keep last 500 snapshots to prevent unbounded growth
	if len(hs.Snapshots) > 500 {
		hs.Snapshots = hs.Snapshots[len(hs.Snapshots)-500:]
	}

	data, err := json.MarshalIndent(hs.Snapshots, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(hs.filePath, data, 0644)
}

// AnalyzeTrends compares the latest snapshot against earlier ones to detect drift.
func (hs *HistoryStore) AnalyzeTrends() []TrendFinding {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.Snapshots) < 2 {
		return nil
	}

	latest := hs.Snapshots[len(hs.Snapshots)-1]
	// Compare against a snapshot from at least 5 minutes ago (or the oldest available)
	baseline := hs.Snapshots[0]
	for i := len(hs.Snapshots) - 2; i >= 0; i-- {
		if latest.Timestamp.Sub(hs.Snapshots[i].Timestamp) >= 5*time.Minute {
			baseline = hs.Snapshots[i]
			break
		}
	}

	var findings []TrendFinding

	// 1. Analyze Docker disk creep
	if baseline.DockerBytes > 0 && latest.DockerBytes > baseline.DockerBytes {
		diff := latest.DockerBytes - baseline.DockerBytes
		pct := float64(diff) / float64(baseline.DockerBytes) * 100.0
		if diff > 1024*1024*1024 || pct > 20.0 { // > 1GB or > 20%
			findings = append(findings, TrendFinding{
				Target:     "Almacenamiento Docker",
				Metric:     "Espacio en Disco",
				ChangeText: fmt.Sprintf("Creció un %.1f%% (+%s)", pct, formatBytes(uint64(diff))),
				OldValue:   formatBytes(uint64(baseline.DockerBytes)),
				NewValue:   formatBytes(uint64(latest.DockerBytes)),
				IsWarning:  true,
				DetectedAt: time.Now(),
			})
		}
	}

	// 2. Analyze Container Memory Creep and Restarts
	for name, curSample := range latest.Containers {
		prevSample, ok := baseline.Containers[name]
		if !ok {
			continue
		}

		// Check memory creep (+25% increase and > 50MB increase)
		if prevSample.MemoryBytes > 0 && curSample.MemoryBytes > prevSample.MemoryBytes {
			memDiff := curSample.MemoryBytes - prevSample.MemoryBytes
			memPct := float64(memDiff) / float64(prevSample.MemoryBytes) * 100.0
			if memDiff > 50*1024*1024 && memPct >= 25.0 {
				findings = append(findings, TrendFinding{
					Target:     name,
					Metric:     "Consumo de RAM",
					ChangeText: fmt.Sprintf("Aumentó un %.1f%% (+%s) respecto al histórico", memPct, formatBytes(memDiff)),
					OldValue:   formatBytes(prevSample.MemoryBytes),
					NewValue:   formatBytes(curSample.MemoryBytes),
					IsWarning:  memPct >= 50.0,
					DetectedAt: time.Now(),
				})
			}
		}

		// Check restart spikes
		if curSample.RestartCount > prevSample.RestartCount {
			restartsDiff := curSample.RestartCount - prevSample.RestartCount
			findings = append(findings, TrendFinding{
				Target:     name,
				Metric:     "Reinicios",
				ChangeText: fmt.Sprintf("%d nuevos reinicios registrados en este periodo", restartsDiff),
				OldValue:   fmt.Sprintf("%d", prevSample.RestartCount),
				NewValue:   fmt.Sprintf("%d", curSample.RestartCount),
				IsWarning:  true,
				DetectedAt: time.Now(),
			})
		}
	}

	return findings
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryRecordingAndTrends(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test_history.json")

	store := LoadHistory(storePath)

	// Snapshot 1 (Baseline)
	snap1 := Snapshot{
		Timestamp:   time.Now().Add(-10 * time.Minute),
		DockerBytes: 10 * 1024 * 1024 * 1024, // 10 GB
		Containers: map[string]ContainerSample{
			"postgres": {
				ID:           "cid1",
				Name:         "postgres",
				MemoryBytes:  200 * 1024 * 1024, // 200 MB
				CPUPercent:   2.5,
				RestartCount: 0,
				State:        "running",
			},
			"api": {
				ID:           "cid2",
				Name:         "api",
				MemoryBytes:  100 * 1024 * 1024,
				CPUPercent:   1.0,
				RestartCount: 1,
				State:        "running",
			},
		},
	}
	if err := store.RecordSnapshot(snap1); err != nil {
		t.Fatalf("failed to record snap1: %v", err)
	}

	// Snapshot 2 (Current with memory creep on postgres, restart spike on api, and disk growth)
	snap2 := Snapshot{
		Timestamp:   time.Now(),
		DockerBytes: 15 * 1024 * 1024 * 1024, // +5 GB (+50%)
		Containers: map[string]ContainerSample{
			"postgres": {
				ID:           "cid1",
				Name:         "postgres",
				MemoryBytes:  320 * 1024 * 1024, // 320 MB (+60% > 50MB)
				CPUPercent:   3.0,
				RestartCount: 0,
				State:        "running",
			},
			"api": {
				ID:           "cid2",
				Name:         "api",
				MemoryBytes:  102 * 1024 * 1024,
				CPUPercent:   1.2,
				RestartCount: 5, // +4 restarts!
				State:        "running",
			},
		},
	}
	if err := store.RecordSnapshot(snap2); err != nil {
		t.Fatalf("failed to record snap2: %v", err)
	}

	trends := store.AnalyzeTrends()
	if len(trends) < 3 {
		t.Fatalf("expected at least 3 trend findings, got %d", len(trends))
	}

	var foundDisk, foundMem, foundRestart bool
	for _, tr := range trends {
		if tr.Metric == "Espacio en Disco" {
			foundDisk = true
		}
		if tr.Target == "postgres" && tr.Metric == "Consumo de RAM" {
			foundMem = true
		}
		if tr.Target == "api" && tr.Metric == "Reinicios" {
			foundRestart = true
		}
	}

	if !foundDisk {
		t.Errorf("expected disk trend finding")
	}
	if !foundMem {
		t.Errorf("expected postgres RAM trend finding")
	}
	if !foundRestart {
		t.Errorf("expected api restart spike finding")
	}
}

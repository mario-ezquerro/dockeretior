package docker

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{15 * 1024 * 1024, "15.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}

	for _, tc := range tests {
		res := FormatBytes(tc.input)
		if res != tc.expected {
			t.Errorf("FormatBytes(%d) = %q, expected %q", tc.input, res, tc.expected)
		}
	}
}

func TestFormatPackets(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 pkts"},
		{999, "999 pkts"},
		{1000, "1.0k pkts"},
		{2500, "2.5k pkts"},
	}

	for _, tc := range tests {
		res := FormatPackets(tc.input)
		if res != tc.expected {
			t.Errorf("FormatPackets(%d) = %q, expected %q", tc.input, res, tc.expected)
		}
	}
}

func TestMetricsTracker_RecordCPU(t *testing.T) {
	tracker := &MetricsTracker{
		history: make(map[string][]float64),
	}

	containerID := "test-container-123"

	// Add 25 values
	for i := 1; i <= 25; i++ {
		pts := tracker.RecordCPU(containerID, float64(i))
		if len(pts) > 20 {
			t.Fatalf("expected max 20 points, got %d", len(pts))
		}
	}

	pts := tracker.history[containerID]
	if len(pts) != 20 {
		t.Fatalf("expected 20 points, got %d", len(pts))
	}

	// First should be 6 (since 1..5 were dropped), last should be 25
	if pts[0] != 6.0 {
		t.Errorf("expected first point 6.0, got %f", pts[0])
	}
	if pts[19] != 25.0 {
		t.Errorf("expected last point 25.0, got %f", pts[19])
	}
}

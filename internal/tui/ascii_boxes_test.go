package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
)

func TestDrawASCIIBox(t *testing.T) {
	lines := []string{"Línea 1 de prueba", "Línea 2 de prueba"}
	box := DrawASCIIBox("TEST TITLE", lines, 40, lipgloss.Color("#FFFFFF"), lipgloss.Color("#6366F1"))

	if !strings.Contains(box, "TEST TITLE") {
		t.Errorf("box does not contain title: %s", box)
	}
	if !strings.Contains(box, "Línea 1 de prueba") {
		t.Errorf("box does not contain line 1: %s", box)
	}
	if !strings.Contains(box, "┌─") || !strings.Contains(box, "└") {
		t.Errorf("box does not have ASCII box borders: %s", box)
	}
}

func TestRenderProgressBar(t *testing.T) {
	bar0 := RenderProgressBar(0, 10)
	if !strings.Contains(bar0, "░░░░░░░░░░") {
		t.Errorf("expected empty bar for 0%%, got %s", bar0)
	}

	bar100 := RenderProgressBar(100, 10)
	if !strings.Contains(bar100, "██████████") {
		t.Errorf("expected full bar for 100%%, got %s", bar100)
	}

	bar50 := RenderProgressBar(50, 10)
	if !strings.Contains(bar50, "█████") || !strings.Contains(bar50, "░░░░░") {
		t.Errorf("expected half filled bar for 50%%, got %s", bar50)
	}
}

func TestRenderSparkline(t *testing.T) {
	values := []float64{0, 25, 50, 75, 100}
	spark := RenderSparkline(values, 100.0, 10)
	if len(spark) == 0 {
		t.Fatalf("expected non-empty sparkline")
	}
}

func TestRenderWindows(t *testing.T) {
	m := &docker.ContainerMetrics{
		ContainerID:   "60ee145d3271",
		Name:          "test-web",
		Image:         "nginx:alpine",
		State:         "running",
		IsRunning:     true,
		CPUPercent:    15.5,
		OnlineCPUs:    4,
		CPUSparkline:  []float64{10.0, 12.0, 15.5},
		MemUsage:      15 * 1024 * 1024,
		MemLimit:      1024 * 1024 * 1024,
		MemPercent:    1.5,
		NetRxBytes:    5000,
		NetTxBytes:    10000,
		PIDs:          12,
		IPAddress:     "172.17.0.2",
		NetworkName:   "bridge",
		Ports:         "80->80",
		Uptime:        "Up 10 minutes",
	}

	wCPU := RenderCPUWindow(m, 50, false)
	if !strings.Contains(wCPU, "CARGA CPU") || !strings.Contains(wCPU, "15.50%") {
		t.Errorf("RenderCPUWindow missing metrics: %s", wCPU)
	}

	wMem := RenderMemoryWindow(m, 50, false)
	if !strings.Contains(wMem, "CARGA MEMORIA") || !strings.Contains(wMem, "1.5%") {
		t.Errorf("RenderMemoryWindow missing metrics: %s", wMem)
	}

	wNet := RenderNetworkWindow(m, 50, false)
	if !strings.Contains(wNet, "RED (I/O)") {
		t.Errorf("RenderNetworkWindow missing metrics: %s", wNet)
	}

	wDet := RenderDetailsWindow(m, 50, false)
	if !strings.Contains(wDet, "RESTO DE USOS") || !strings.Contains(wDet, "172.17.0.2") {
		t.Errorf("RenderDetailsWindow missing metrics: %s", wDet)
	}

	rightPane := RenderRightPane(m, 50, 30)
	if !strings.Contains(rightPane, "CARGA CPU") || !strings.Contains(rightPane, "RESTO DE USOS") {
		t.Errorf("RenderRightPane incomplete: %s", rightPane)
	}
}

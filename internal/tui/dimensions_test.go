package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mattn/go-runewidth"
)

func TestDimensionsFit(t *testing.T) {
	cli, err := docker.NewClient()
	if err != nil {
		t.Skip("docker not available")
	}
	defer cli.Close()

	containers, err := cli.ListContainers(context.Background(), true)
	if err != nil {
		t.Fatalf("list error: %v", err)
	}

	testSizes := []struct {
		w int
		h int
	}{
		{80, 24},
		{90, 24},
		{100, 24},
		{100, 30},
		{120, 24},
		{120, 30},
		{140, 40},
	}

	for _, sz := range testSizes {
		out := renderContainersDashboard(containers, 0, nil, true, "", false, false, sz.w, sz.h)
		lines := strings.Split(out, "\n")

		for i, line := range lines {
			visWidth := runewidth.StringWidth(stripANSI(line))
			if visWidth > sz.w {
				t.Errorf("[%dx%d] Line %d exceeds width %d! visWidth=%d: %q", sz.w, sz.h, i, sz.w, visWidth, line)
			}
		}

		if len(lines) > sz.h+1 {
			t.Errorf("[%dx%d] Total lines %d exceeds height %d!", sz.w, sz.h, len(lines), sz.h)
		}
	}
}

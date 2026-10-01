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
		out := renderContainersDashboard(containers, 0, nil, nil, true, "", false, false, sz.w, sz.h)
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

		// Also verify AutoDoctor view fits dimensions perfectly
		docOut := renderAutoDoctorView(nil, false, "Test status", sz.w, sz.h)
		docLines := strings.Split(docOut, "\n")
		for i, line := range docLines {
			visWidth := runewidth.StringWidth(stripANSI(line))
			if visWidth > sz.w {
				t.Errorf("[AutoDoctor %dx%d] Line %d exceeds width %d! visWidth=%d: %q", sz.w, sz.h, i, sz.w, visWidth, line)
			}
		}
		if len(docLines) > sz.h+1 {
			t.Errorf("[AutoDoctor %dx%d] Total lines %d exceeds height %d!", sz.w, sz.h, len(docLines), sz.h)
		}

		// Also verify Compose view fits dimensions perfectly (both file mode and topology mode)
		compOut := renderComposeView(".", nil, nil, 0, false, "", "", sz.w, sz.h)
		compLines := strings.Split(compOut, "\n")
		for i, line := range compLines {
			visWidth := runewidth.StringWidth(stripANSI(line))
			if visWidth > sz.w {
				t.Errorf("[Compose %dx%d] Line %d exceeds width %d! visWidth=%d: %q", sz.w, sz.h, i, sz.w, visWidth, line)
			}
		}
		if len(compLines) > sz.h+1 {
			t.Errorf("[Compose %dx%d] Total lines %d exceeds height %d!", sz.w, sz.h, len(compLines), sz.h)
		}
	}
}

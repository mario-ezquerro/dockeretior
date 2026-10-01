package tui

import (
	"strings"
	"testing"
)

func TestRenderBottomBar(t *testing.T) {
	// Wide terminal
	barWide := RenderBottomBar(true, true, false, 120)
	if !strings.Contains(barWide, "F1") || !strings.Contains(barWide, "Ayuda") {
		t.Errorf("bottom bar missing F1: %s", barWide)
	}
	if !strings.Contains(barWide, "F2") || !strings.Contains(barWide, "Ver:Todos") {
		t.Errorf("bottom bar missing F2 Ver:Todos: %s", barWide)
	}
	if !strings.Contains(barWide, "F4") || !strings.Contains(barWide, "Exec") {
		t.Errorf("bottom bar missing F4 Exec: %s", barWide)
	}
	if !strings.Contains(barWide, "F6") || !strings.Contains(barWide, "Stop") {
		t.Errorf("bottom bar missing F6 Stop: %s", barWide)
	}
	if !strings.Contains(barWide, "F8") || !strings.Contains(barWide, "Borrar") {
		t.Errorf("bottom bar missing F8 Borrar: %s", barWide)
	}

	// Compact terminal (width 100)
	barCompact := RenderBottomBar(false, false, true, 100)
	if !strings.Contains(barCompact, "Activos") {
		t.Errorf("bottom bar missing Activos: %s", barCompact)
	}
	if !strings.Contains(barCompact, "Start") {
		t.Errorf("bottom bar missing Start: %s", barCompact)
	}
	if !strings.Contains(barCompact, "Reanudar") {
		t.Errorf("bottom bar missing Reanudar: %s", barCompact)
	}
}

func TestRenderDeleteConfirmModal(t *testing.T) {
	modal := RenderDeleteConfirmModal("test-container", "123456789012345", 60)
	if !strings.Contains(modal, "ELIMINAR CONTENEDOR") {
		t.Errorf("delete modal missing title: %s", modal)
	}
	if !strings.Contains(modal, "test-container") {
		t.Errorf("delete modal missing container name: %s", modal)
	}
	if !strings.Contains(modal, "[y / Enter]") || !strings.Contains(modal, "[f]") {
		t.Errorf("delete modal missing action instructions: %s", modal)
	}
}

func TestRenderHelpModal(t *testing.T) {
	help := RenderHelpModal(70)
	if !strings.Contains(help, "AYUDA Y ATAJOS") {
		t.Errorf("help modal missing title: %s", help)
	}
	if !strings.Contains(help, "F4") || !strings.Contains(strings.ToLower(help), "exec") {
		t.Errorf("help modal missing F4 exec explanation: %s", help)
	}
	if !strings.Contains(help, "F8") || !strings.Contains(help, "Borrar") {
		t.Errorf("help modal missing F8 delete explanation: %s", help)
	}
}

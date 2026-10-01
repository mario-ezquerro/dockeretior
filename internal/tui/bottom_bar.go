package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// BottomBarItem represents an action displayed in the bottom menu.
type BottomBarItem struct {
	Key   string
	Label string
}

// RenderBottomBar formats the function keys menu bar at the bottom of the screen.
func RenderBottomBar(filterRunningOnly bool, isSelectedRunning bool, isSelectedPaused bool, width int) string {
	f2Label := "Ver:Todos"
	if !filterRunningOnly {
		f2Label = "Ver:Activos"
	}

	f6Label := "Stop"
	if !isSelectedRunning {
		f6Label = "Start"
	}

	f7Label := "Pausar"
	if isSelectedPaused {
		f7Label = "Reanudar"
	}

	// For narrow terminals, use compact labels
	isCompact := width < 110
	if isCompact {
		if f2Label == "Ver:Todos" {
			f2Label = "Todos"
		} else {
			f2Label = "Activos"
		}
	}

	items := []BottomBarItem{
		{Key: "F1", Label: "Ayuda"},
		{Key: "F2", Label: f2Label},
		{Key: "F3", Label: "Logs"},
		{Key: "F4", Label: "Exec"},
		{Key: "F5", Label: "Restart"},
		{Key: "F6", Label: f6Label},
		{Key: "F7", Label: f7Label},
		{Key: "F8", Label: "Borrar"},
		{Key: "F9", Label: "Inspect"},
		{Key: "F10", Label: "Salir"},
	}

	padH := 1
	if isCompact {
		padH = 0
	}

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4F46E5")).
		Padding(0, padH)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, padH)

	sep := " "
	if width < 85 {
		sep = ""
	}

	var renderedItems []string
	curLen := 0
	for _, item := range items {
		btn := keyStyle.Render(item.Key) + labelStyle.Render(item.Label)
		btnLen := runewidth.StringWidth(stripANSI(btn))
		if curLen+btnLen+1 > width && curLen > 0 {
			// Don't overflow width
			break
		}
		renderedItems = append(renderedItems, btn)
		curLen += btnLen + len(sep)
	}

	return strings.Join(renderedItems, sep)
}

// RenderDeleteConfirmModal generates the confirmation dialog for F8 (Borrar).
func RenderDeleteConfirmModal(containerName, containerID string, width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	highlightStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))

	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	lines := []string{
		titleStyle.Render("⚠️  CONFIRMAR ELIMINACIÓN DE CONTENEDOR"),
		fmt.Sprintf("¿Eliminar %s (%s)?", highlightStyle.Render(containerName), shortID),
		"",
		" [y / Enter] Eliminar normal",
		" [f]         Forzar (-f)",
		" [n / Esc]   Cancelar",
	}

	return DrawASCIIBox("ELIMINAR CONTENEDOR", lines, width, lipgloss.Color("#EF4444"), lipgloss.Color("#EF4444"))
}

// RenderHelpModal generates the F1 overlay modal with keyboard shortcuts and functions.
func RenderHelpModal(width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8"))
	kStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5B4FC"))

	lines := []string{
		titleStyle.Render("⚓ DOCKERETIOR - ATAJOS DE TECLADO Y FUNCIONES"),
		"",
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F1 / ? / 1"), "Muestra esta ayuda"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F2 / f / 2"), "Alternar filtro: Solo en ejecución / Todos"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F3 / l / 3"), "Ver logs en tiempo real"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F4 / e / 4"), "Exec: Entrar con shell interactiva (bash/sh)"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F5 / r / 5"), "Reiniciar contenedor seleccionado"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F6 / s / 6"), "Detener (Stop) o Iniciar (Start)"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F7 / p / 7"), "Pausar o Reanudar procesos"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F8 / x / 8"), "Borrar contenedor (confirmar o forzar con f)"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F9 / i / 9"), "Inspeccionar configuración JSON"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("F10 / q / 0"), "Salir de Dockeretior"),
		fmt.Sprintf(" %-14s : %s", kStyle.Render("↑ / ↓ o k / j"), "Navegar por la lista de contenedores"),
		"",
		DimStyle.Render("Pulsa [Esc] o [F1] para volver al panel."),
	}

	boxWidth := width
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 46 {
		boxWidth = 46
	}

	return DrawASCIIBox("AYUDA Y ATAJOS", lines, boxWidth, lipgloss.Color("#38BDF8"), lipgloss.Color("#6366F1"))
}

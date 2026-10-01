package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mario-ezquerro/dockeretior/internal/compose"
	"github.com/mattn/go-runewidth"
)

// renderComposeView renders the directory browser and the visual topology tree of Docker Compose.
func renderComposeView(
	currentDir string,
	entries []compose.FileEntry,
	selectedProject *compose.ComposeProject,
	cursor int,
	showTopology bool,
	statusMsg string,
	outputText string,
	width int,
	height int,
) string {
	if width < 50 {
		width = 80
	}
	if height < 15 {
		height = 24
	}

	var lines []string

	// 1. Output modal from compose up/down
	if outputText != "" {
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8")).Render("── SALIDA DE DOCKER COMPOSE ─────────────────────────────────────"), width))
		lines = append(lines, "")
		for _, l := range strings.Split(outputText, "\n") {
			lines = append(lines, padOrTruncate("  "+l, width))
			if len(lines) >= height-3 {
				break
			}
		}
		lines = append(lines, "")
		lines = append(lines, padOrTruncate(HelpBarStyle.Render("[q / Esc: Volver al visor Compose]"), width))
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", width))
		}
		return strings.Join(lines[:height], "\n")
	}

	// 2. Top Header Bar
	headerTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0F172A")).
		Background(lipgloss.Color("#818CF8")).
		Padding(0, 1).
		Render("🐙 DOCKER COMPOSE")

	modeBadge := "[t: Ver Archivos]"
	if !showTopology || selectedProject == nil {
		modeBadge = "[t: Ver Topología]"
	}
	navInfo := DimStyle.Render(fmt.Sprintf("%s [u: Up -d] [d: Down] [Esc/q: Volver]", modeBadge))

	pad := width - runewidth.StringWidth(stripANSI(headerTitle)) - runewidth.StringWidth(stripANSI(navInfo))
	topLine := headerTitle
	if pad > 0 {
		topLine += strings.Repeat(" ", pad) + navInfo
	} else {
		topLine += " " + navInfo
	}
	lines = append(lines, padOrTruncate(topLine, width))

	// Active directory indicator
	dirLine := fmt.Sprintf(" %s %s", DimStyle.Render("Directorio:"), lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(currentDir))
	lines = append(lines, padOrTruncate(dirLine, width))
	lines = append(lines, "")

	// 3. Main Body: Topology Graph or Directory Explorer
	if showTopology && selectedProject != nil && selectedProject.Stack != nil {
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5B4FC")).Render(
			fmt.Sprintf("── MAPA TOPOLÓGICO: %s (%d servicios) ──────────────", selectedProject.FileName, len(selectedProject.Stack.Services))), width))

		asciiGraph := compose.RenderTopologyASCII(selectedProject.Stack, selectedProject.Graph, width)
		for _, gl := range strings.Split(asciiGraph, "\n") {
			lines = append(lines, padOrTruncate(gl, width))
			if len(lines) >= height-4 {
				break
			}
		}
	} else {
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#94A3B8")).Render("── ARCHIVOS Y PROYECTOS COMPOSE ────────────────────────────────"), width))

		if len(entries) == 0 {
			lines = append(lines, padOrTruncate("  (No se encontraron subcarpetas ni archivos .yml / .yaml)", width))
		}

		maxEntries := height - 9
		if maxEntries < 3 {
			maxEntries = 3
		}

		startIdx := 0
		if cursor >= maxEntries {
			startIdx = cursor - maxEntries + 1
		}
		endIdx := startIdx + maxEntries
		if endIdx > len(entries) {
			endIdx = len(entries)
		}

		for i := startIdx; i < endIdx; i++ {
			entry := entries[i]
			prefix := "   "
			if i == cursor {
				prefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8")).Render(" ▶ ")
			}

			var namePart string
			var metaPart string

			if entry.IsDir {
				namePart = fmt.Sprintf("📁 %-32s", entry.Name)
				metaPart = DimStyle.Render("[Carpeta]")
			} else {
				namePart = fmt.Sprintf("📄 %-32s", entry.Name)
				if entry.IsCompose {
					metaPart = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render(
						fmt.Sprintf("[%d servicios detectados - Pulsa Enter]", entry.ServiceCount))
				} else {
					metaPart = DimStyle.Render("(YAML)")
				}
			}

			rowStr := prefix + namePart + " " + metaPart
			if i == cursor {
				rowStr = lipgloss.NewStyle().Background(lipgloss.Color("#1E293B")).Render(padOrTruncate(rowStr, width-2))
			}
			lines = append(lines, padOrTruncate(rowStr, width))
		}

		// Show summary of selected project if any
		if selectedProject != nil {
			lines = append(lines, "")
			lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render(
				fmt.Sprintf(" ✔ Proyecto cargado: %s (%d servicios). Pulsa [t] para ver topología.", selectedProject.FileName, len(selectedProject.Services))), width))
		}
	}

	// 4. Status message
	if statusMsg != "" {
		lines = append(lines, "")
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("⚡ "+statusMsg), width))
	}

	// 5. Bottom Instructions Bar
	botBar := HelpBarStyle.Render("[↑/↓: Mover] [Enter: Cargar/Entrar] [t: Topología visual] [u: Up -d] [d: Down] [q: Menú]")
	lines = append(lines, padOrTruncate(botBar, width))

	// Ensure exact height
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

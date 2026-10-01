package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/mario-ezquerro/dockeretior/internal/autodoctor"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mattn/go-runewidth"
)

// renderContainersDashboard renders the split-screen view: container list on the left and ASCII windows on the right.
func renderContainersDashboard(
	containers []types.Container,
	cursor int,
	currentMetrics *docker.ContainerMetrics,
	healthReport *autodoctor.HealthReport,
	filterRunningOnly bool,
	statusMsg string,
	confirmDelete bool,
	showHelp bool,
	width int,
	height int,
) string {
	if width < 50 {
		width = 80
	}
	if height < 15 {
		height = 24
	}

	// Calculate widths for split layout: leftWidth + 2 (sep) + rightWidth == width
	rightWidth := 38
	if width >= 120 {
		rightWidth = 46
	} else if width < 85 {
		rightWidth = 34
	}
	leftWidth := width - rightWidth - 2
	if leftWidth < 28 {
		leftWidth = 28
		rightWidth = width - leftWidth - 2
		if rightWidth < 20 {
			rightWidth = 20
		}
	}

	// 1. Header Line
	runningCount := 0
	stoppedCount := 0
	for _, c := range containers {
		if strings.ToLower(c.State) == "running" {
			runningCount++
		} else {
			stoppedCount++
		}
	}

	filterBadge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Background(lipgloss.Color("#064E3B")).Render(" [ACTIVOS] ")
	if !filterRunningOnly {
		filterBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Background(lipgloss.Color("#78350F")).Render(" [TODOS] ")
	}

	headerLeft := HeaderTitleStyle.Render(" ⚓ DOCKERETIOR ") + filterBadge
	countStr := fmt.Sprintf("%d activos", runningCount)
	if !filterRunningOnly {
		countStr = fmt.Sprintf("%d activos, %d detenidos", runningCount, stoppedCount)
	}

	healthBadge := ""
	if healthReport != nil {
		healthBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8")).Render(fmt.Sprintf("🩺 %s %d/100 [a:AutoDoctor] ", healthReport.HealthBadge, healthReport.HealthScore))
	}

	headerRight := healthBadge + DimStyle.Render(countStr) + " "

	headerLine := headerLeft + headerRight
	pad := width - runewidth.StringWidth(stripANSI(headerLeft)) - runewidth.StringWidth(stripANSI(headerRight))
	if pad > 0 {
		headerLine = headerLeft + strings.Repeat(" ", pad) + headerRight
	}
	headerLine = padOrTruncate(headerLine, width)

	// If Help Modal is open
	if showHelp {
		modal := RenderHelpModal(width)
		return headerLine + "\n\n" + modal + "\n\n" + RenderBottomBar(filterRunningOnly, isRunning(containers, cursor), isPaused(containers, cursor), width)
	}

	// If Delete Confirmation is active
	if confirmDelete && len(containers) > 0 && cursor < len(containers) {
		c := containers[cursor]
		name := getContainerName(c)
		deleteModal := RenderDeleteConfirmModal(name, c.ID, width-6)
		return headerLine + "\n\n" + deleteModal + "\n\n" + RenderBottomBar(filterRunningOnly, isRunning(containers, cursor), isPaused(containers, cursor), width)
	}

	// Calculate vertical budget
	overhead := 2 // 1 line header + 1 line bottom bar
	if statusMsg != "" {
		overhead = 3 // + 1 line status
	}
	availLines := height - overhead
	if availLines < 16 {
		availLines = 16
	}

	// 2. Right Pane: 4 ASCII Windows filling availLines exactly
	rightPaneRaw := RenderRightPane(currentMetrics, rightWidth, availLines)
	rightLines := strings.Split(rightPaneRaw, "\n")
	for i, rl := range rightLines {
		rightLines[i] = padOrTruncate(rl, rightWidth)
	}

	// 3. Left Pane: Table of Containers filling availLines exactly
	leftLines := renderContainersTableLines(containers, cursor, leftWidth, availLines)

	// Equalize lines to availLines
	for len(leftLines) < availLines {
		leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
	}
	if len(leftLines) > availLines {
		leftLines = leftLines[:availLines]
	}
	for len(rightLines) < availLines {
		rightLines = append(rightLines, strings.Repeat(" ", rightWidth))
	}
	if len(rightLines) > availLines {
		rightLines = rightLines[:availLines]
	}

	// 4. Combine Left and Right Panes line-by-line
	var splitLines []string
	for i := 0; i < availLines; i++ {
		combined := leftLines[i] + "  " + rightLines[i]
		splitLines = append(splitLines, padOrTruncate(combined, width))
	}
	splitView := strings.Join(splitLines, "\n")

	// 5. Status / Message Line
	statusLine := ""
	if statusMsg != "" {
		statusLine = padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("⚡ "+statusMsg), width) + "\n"
	}

	// 6. Bottom Menu Bar with Function Keys
	isSelectedRun := isRunning(containers, cursor)
	isSelectedPau := isPaused(containers, cursor)
	bottomBar := padOrTruncate(RenderBottomBar(filterRunningOnly, isSelectedRun, isSelectedPau, width), width)

	return headerLine + "\n" + splitView + "\n" + statusLine + bottomBar
}

func renderContainersTableLines(containers []types.Container, cursor int, width int, targetLines int) []string {
	var lines []string

	// Column widths
	colID := 12
	colStatus := 11 // "● Up 45m"
	colPorts := 11
	if width < 45 {
		colPorts = 8
	}
	avail := width - colID - colStatus - colPorts - 6
	if avail < 8 {
		avail = 8
	}
	colName := avail / 2
	colImage := avail - colName

	// Header
	headerStr := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s",
		colID, "ID",
		colName, "NOMBRE",
		colImage, "IMAGEN",
		colStatus, "ESTADO",
		colPorts, "PUERTOS",
	)
	lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#94A3B8")).Render(headerStr), width))
	lines = append(lines, padOrTruncate(DimStyle.Render(strings.Repeat("─", width)), width))

	if len(containers) == 0 {
		lines = append(lines, padOrTruncate("  "+DimStyle.Render("No hay contenedores en ejecución."), width))
		lines = append(lines, padOrTruncate("  "+DimStyle.Render("Pulsa [F2] para ver todos."), width))
		for len(lines) < targetLines {
			lines = append(lines, strings.Repeat(" ", width))
		}
		return lines
	}

	// Visible table row capacity
	maxRows := targetLines - 2
	if maxRows < 1 {
		maxRows = 1
	}

	startIdx := 0
	if cursor >= maxRows {
		startIdx = cursor - maxRows + 1
	}
	endIdx := startIdx + maxRows
	if endIdx > len(containers) {
		endIdx = len(containers)
	}

	for i := startIdx; i < endIdx; i++ {
		c := containers[i]
		id := c.ID
		if len(id) > colID {
			id = id[:colID]
		}

		name := getContainerName(c)
		name = truncateString(name, colName)

		image := c.Image
		image = truncateString(image, colImage)

		// Status with symbol
		stateLower := strings.ToLower(c.State)
		var statusDot string
		var statusColor lipgloss.Color
		if strings.HasPrefix(stateLower, "running") {
			statusDot = "●"
			statusColor = SuccessColor
		} else if strings.HasPrefix(stateLower, "paused") {
			statusDot = "▲"
			statusColor = WarningColor
		} else {
			statusDot = "■"
			statusColor = DangerColor
		}

		statusText := truncateString(c.Status, colStatus-3)
		dotRendered := lipgloss.NewStyle().Foreground(statusColor).Render(statusDot) + " "

		// Ports
		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d->%d", p.PublicPort, p.PrivatePort))
			}
		}
		portsStr := strings.Join(ports, ",")
		if portsStr == "" {
			portsStr = "-"
		}
		portsStr = truncateString(portsStr, colPorts)

		row := fmt.Sprintf("%-*s %-*s %-*s %s%-*s %-*s",
			colID, id,
			colName, name,
			colImage, image,
			dotRendered, colStatus-2, statusText,
			colPorts, portsStr,
		)

		if cursor == i {
			prefix := SelectedRowStyle.Render("▶ ")
			highlighted := lipgloss.NewStyle().Bold(true).Foreground(TextLight).Background(lipgloss.Color("#1E1B4B")).Render(row)
			lines = append(lines, padOrTruncate(prefix+highlighted, width))
		} else {
			lines = append(lines, padOrTruncate("  "+row, width))
		}
	}

	// Pad with empty rows to fill targetLines exactly
	for len(lines) < targetLines {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return lines
}

func padOrTruncate(s string, targetLen int) string {
	visLen := runewidth.StringWidth(stripANSI(s))
	if visLen > targetLen {
		return truncateANSI(s, targetLen)
	}
	if visLen < targetLen {
		return s + strings.Repeat(" ", targetLen-visLen)
	}
	return s
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= maxLen {
		return s
	}
	var sb strings.Builder
	cur := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if cur+rw > maxLen-2 {
			break
		}
		sb.WriteRune(r)
		cur += rw
	}
	sb.WriteString("..")
	return sb.String()
}

func getContainerName(c types.Container) string {
	name := "unnamed"
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	return name
}

func isRunning(containers []types.Container, cursor int) bool {
	if len(containers) > 0 && cursor >= 0 && cursor < len(containers) {
		return strings.ToLower(containers[cursor].State) == "running"
	}
	return false
}

func isPaused(containers []types.Container, cursor int) bool {
	if len(containers) > 0 && cursor >= 0 && cursor < len(containers) {
		return strings.ToLower(containers[cursor].State) == "paused"
	}
	return false
}

func formatInspectJSON(data types.ContainerJSON) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formateando JSON: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 60 {
		return strings.Join(lines[:60], "\n") + fmt.Sprintf("\n... (%d líneas adicionales)", len(lines)-60)
	}
	return string(b)
}

func renderInspectView(jsonText string, width int, height int) string {
	lines := strings.Split(jsonText, "\n")
	return DrawASCIIBox("INSPECCIÓN DETALLADA DEL CONTENEDOR (JSON)", lines, width, height-3, lipgloss.Color("#38BDF8"), PrimaryColor) +
		"\n\n" + HelpBarStyle.Render("[Esc / q: Volver a la lista de contenedores]")
}

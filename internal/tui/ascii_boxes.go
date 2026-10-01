package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mattn/go-runewidth"
)

// DrawASCIIBox creates a window drawn with ASCII / UTF-8 box characters ("rayitas").
func DrawASCIIBox(title string, lines []string, width int, titleColor lipgloss.Color, borderColor lipgloss.Color) string {
	if width < 25 {
		width = 25
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(titleColor)

	// Top line: ┌─ [ TITLE ] ──────────────────────┐
	titleRendered := " " + titleStyle.Render(title) + " "
	titleVisibleLen := runewidth.StringWidth(title) + 2

	dashesNeeded := width - 3 - titleVisibleLen
	if dashesNeeded < 1 {
		dashesNeeded = 1
	}

	topLine := borderStyle.Render("┌─") + titleRendered + borderStyle.Render(strings.Repeat("─", dashesNeeded)+"┐")

	// Middle lines: │ content │
	contentWidth := width - 4
	var middleLines []string
	for _, l := range lines {
		plainLen := runewidth.StringWidth(stripANSI(l))
		if plainLen > contentWidth {
			l = truncateANSI(l, contentWidth)
			plainLen = runewidth.StringWidth(stripANSI(l))
		}
		pad := contentWidth - plainLen
		if pad < 0 {
			pad = 0
		}
		middleLines = append(middleLines, borderStyle.Render("│ ") + l + strings.Repeat(" ", pad) + borderStyle.Render(" │"))
	}

	// Bottom line: └──────────────────────────────┘
	bottomLine := borderStyle.Render("└" + strings.Repeat("─", width-2) + "┘")

	return topLine + "\n" + strings.Join(middleLines, "\n") + "\n" + bottomLine
}

// RenderProgressBar generates a colored horizontal bar gauge.
func RenderProgressBar(percent float64, barWidth int) string {
	if barWidth < 5 {
		barWidth = 8
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int((percent / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	fillColor := SuccessColor
	if percent > 85 {
		fillColor = DangerColor
	} else if percent > 60 {
		fillColor = WarningColor
	}

	fillStr := lipgloss.NewStyle().Foreground(fillColor).Render(strings.Repeat("█", filled))
	emptyStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#334155")).Render(strings.Repeat("░", empty))

	return "[" + fillStr + emptyStr + "]"
}

// RenderSparkline builds a mini trend chart from past samples using UTF-8 runes.
func RenderSparkline(values []float64, maxVal float64, width int) string {
	runes := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	if width < 5 {
		width = 6
	}

	if maxVal <= 0 {
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 100.0
	}

	start := 0
	if len(values) > width {
		start = len(values) - width
	}
	subset := values[start:]

	var b strings.Builder
	pad := width - len(subset)
	for i := 0; i < pad; i++ {
		b.WriteString(" ")
	}

	for _, v := range subset {
		ratio := v / maxVal
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		idx := int(ratio * float64(len(runes)-1))
		b.WriteRune(runes[idx])
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(b.String())
}

// RenderCPUWindow creates ASCII Window 1: Carga de CPU.
func RenderCPUWindow(m *docker.ContainerMetrics, width int, compact bool) string {
	title := "1. CARGA CPU"
	borderColor := PrimaryColor
	if m == nil || !m.IsRunning {
		lines := []string{
			"Estado : " + StatusStoppedStyle.Render("Detenido"),
			"Carga  : " + DimStyle.Render("Inactivo (0.0%)"),
		}
		if !compact {
			lines = append(lines, "Nota   : Iniciar con [F6]")
		}
		return DrawASCIIBox(title, lines, width, lipgloss.Color("#94A3B8"), lipgloss.Color("#475569"))
	}

	cpuColor := SuccessColor
	if m.CPUPercent > 85 {
		cpuColor = DangerColor
	} else if m.CPUPercent > 60 {
		cpuColor = WarningColor
	}
	pctStr := lipgloss.NewStyle().Bold(true).Foreground(cpuColor).Render(fmt.Sprintf("%5.2f%%", m.CPUPercent))

	barWidth := width - 24
	if barWidth < 6 {
		barWidth = 6
	}

	sparkWidth := width - 24
	if sparkWidth < 6 {
		sparkWidth = 6
	}

	spark := RenderSparkline(m.CPUSparkline, 100.0, sparkWidth)

	var lines []string
	if compact {
		lines = []string{
			fmt.Sprintf("CPU: %s (%dc) %s", pctStr, m.OnlineCPUs, RenderProgressBar(m.CPUPercent, barWidth)),
			fmt.Sprintf("Tendencia: %s %s", spark, DimStyle.Render("(histórico)")),
		}
	} else {
		lines = []string{
			fmt.Sprintf("Uso CPU   : %s  (Cores: %d)", pctStr, m.OnlineCPUs),
			fmt.Sprintf("Barra     : %s %s", RenderProgressBar(m.CPUPercent, barWidth), pctStr),
			fmt.Sprintf("Tendencia : %s %s", spark, DimStyle.Render("(histórico)")),
		}
	}

	return DrawASCIIBox(title, lines, width, lipgloss.Color("#38BDF8"), borderColor)
}

// RenderMemoryWindow creates ASCII Window 2: Carga de Memoria.
func RenderMemoryWindow(m *docker.ContainerMetrics, width int, compact bool) string {
	title := "2. CARGA MEMORIA"
	borderColor := PrimaryColor
	if m == nil || !m.IsRunning {
		lines := []string{
			"Estado : " + StatusStoppedStyle.Render("Inactivo"),
			"Uso RAM: 0 B / 0 B (0%)",
		}
		return DrawASCIIBox(title, lines, width, lipgloss.Color("#94A3B8"), lipgloss.Color("#475569"))
	}

	memColor := SuccessColor
	if m.MemPercent > 85 {
		memColor = DangerColor
	} else if m.MemPercent > 60 {
		memColor = WarningColor
	}
	memPctStr := lipgloss.NewStyle().Bold(true).Foreground(memColor).Render(fmt.Sprintf("%.1f%%", m.MemPercent))

	barWidth := width - 24
	if barWidth < 6 {
		barWidth = 6
	}

	var lines []string
	if compact {
		lines = []string{
			fmt.Sprintf("RAM: %s/%s (%s)", docker.FormatBytes(m.MemUsage), docker.FormatBytes(m.MemLimit), memPctStr),
			fmt.Sprintf("Barra: %s  Pico: %s", RenderProgressBar(m.MemPercent, barWidth), docker.FormatBytes(m.MemMaxUsage)),
		}
	} else {
		lines = []string{
			fmt.Sprintf("Uso Mem : %s / %s (%s)", docker.FormatBytes(m.MemUsage), docker.FormatBytes(m.MemLimit), memPctStr),
			fmt.Sprintf("Barra   : %s %s", RenderProgressBar(m.MemPercent, barWidth), memPctStr),
			fmt.Sprintf("Pico    : %s │ Caché: %s", docker.FormatBytes(m.MemMaxUsage), docker.FormatBytes(m.MemCache)),
		}
	}

	return DrawASCIIBox(title, lines, width, lipgloss.Color("#A855F7"), borderColor)
}

// RenderNetworkWindow creates ASCII Window 3: Carga de Red (I/O).
func RenderNetworkWindow(m *docker.ContainerMetrics, width int, compact bool) string {
	title := "3. RED (I/O)"
	borderColor := PrimaryColor
	if m == nil || !m.IsRunning {
		lines := []string{
			"Estado : " + StatusStoppedStyle.Render("Desconectado"),
			"Tráfico: 0 B",
		}
		return DrawASCIIBox(title, lines, width, lipgloss.Color("#94A3B8"), lipgloss.Color("#475569"))
	}

	rxStr := lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render(docker.FormatBytes(m.NetRxBytes))
	txStr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8")).Render(docker.FormatBytes(m.NetTxBytes))

	var lines []string
	if compact {
		lines = []string{
			fmt.Sprintf("▼ RX: %s │ ▲ TX: %s", rxStr, txStr),
			fmt.Sprintf("Pkts: RX %s │ TX %s", docker.FormatPackets(m.NetRxPackets), docker.FormatPackets(m.NetTxPackets)),
		}
	} else {
		lines = []string{
			fmt.Sprintf("Tráfico RX (↓): %s (%s)", rxStr, docker.FormatPackets(m.NetRxPackets)),
			fmt.Sprintf("Tráfico TX (↑): %s (%s)", txStr, docker.FormatPackets(m.NetTxPackets)),
			fmt.Sprintf("Total Red     : %s", docker.FormatBytes(m.NetRxBytes+m.NetTxBytes)),
		}
	}

	return DrawASCIIBox(title, lines, width, lipgloss.Color("#10B981"), borderColor)
}

// RenderDetailsWindow creates ASCII Window 4: Resto de Usos y Estado.
func RenderDetailsWindow(m *docker.ContainerMetrics, width int, compact bool) string {
	title := "4. RESTO DE USOS"
	borderColor := PrimaryColor
	if m == nil {
		lines := []string{
			"Estado : " + DimStyle.Render("Sin contenedor"),
		}
		return DrawASCIIBox(title, lines, width, lipgloss.Color("#94A3B8"), lipgloss.Color("#475569"))
	}

	stateRendered := StatusRunningStyle.Render("● " + m.State)
	if !m.IsRunning {
		stateRendered = StatusStoppedStyle.Render("■ " + m.State)
	}

	blkReadStr := docker.FormatBytes(m.BlkRead)
	blkWriteStr := docker.FormatBytes(m.BlkWrite)

	var lines []string
	if compact {
		lines = []string{
			fmt.Sprintf("Disco : R %s │ W %s", blkReadStr, blkWriteStr),
			fmt.Sprintf("PIDs  : %d │ IP: %s", m.PIDs, m.IPAddress),
			fmt.Sprintf("Estado: %s (%s)", stateRendered, m.Uptime),
		}
	} else {
		lines = []string{
			fmt.Sprintf("Disco I/O: R %s │ W %s", blkReadStr, blkWriteStr),
			fmt.Sprintf("Procesos : %d PIDs │ IP: %s", m.PIDs, m.IPAddress),
			fmt.Sprintf("Red & IP : %s (%s)", m.IPAddress, m.NetworkName),
			fmt.Sprintf("Estado   : %s (%s)", stateRendered, m.Uptime),
		}
	}

	return DrawASCIIBox(title, lines, width, lipgloss.Color("#F59E0B"), borderColor)
}

// RenderRightPane stacks the 4 ASCII windows vertically.
func RenderRightPane(m *docker.ContainerMetrics, width int, height int) string {
	if width < 26 {
		width = 26
	}

	compact := height < 28

	w1 := RenderCPUWindow(m, width, compact)
	w2 := RenderMemoryWindow(m, width, compact)
	w3 := RenderNetworkWindow(m, width, compact)
	w4 := RenderDetailsWindow(m, width, compact)

	return w1 + "\n" + w2 + "\n" + w3 + "\n" + w4
}

// stripANSI removes ANSI escape codes to compute visible character length correctly.
func stripANSI(str string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range str {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// truncateANSI truncates a string preserving ANSI escape boundaries.
func truncateANSI(s string, maxLen int) string {
	if runewidth.StringWidth(stripANSI(s)) <= maxLen {
		return s
	}
	var sb strings.Builder
	cur := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		rw := runewidth.RuneWidth(r)
		if cur+rw > maxLen-2 {
			break
		}
		sb.WriteRune(r)
		cur += rw
	}
	sb.WriteString("..")
	sb.WriteString("\x1b[0m")
	return sb.String()
}

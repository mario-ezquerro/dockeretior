package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mario-ezquerro/dockeretior/internal/autodoctor"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mattn/go-runewidth"
)

// renderAutoDoctorView renders the intelligent diagnostics screen bounded strictly to width and height.
func renderAutoDoctorView(
	report *autodoctor.HealthReport,
	isAuditing bool,
	statusMsg string,
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

	// 1. Top Header Banner
	headerLeft := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0F172A")).
		Background(lipgloss.Color("#38BDF8")).
		Padding(0, 1).
		Render("🩺 AUTODOCTOR") +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#0284C7")).
			Padding(0, 1).
			Render("Diagnóstico Inteligente del Servidor")

	refreshBadge := DimStyle.Render("[r: Actualizar] [c: Limpiar Espacio] [Esc / q: Volver]")
	pad := width - runewidth.StringWidth(stripANSI(headerLeft)) - runewidth.StringWidth(stripANSI(refreshBadge))
	headerLine := headerLeft
	if pad > 0 {
		headerLine += strings.Repeat(" ", pad) + refreshBadge
	} else {
		headerLine += " " + refreshBadge
	}
	lines = append(lines, padOrTruncate(headerLine, width))
	lines = append(lines, "")

	if isAuditing && report == nil {
		lines = append(lines, padOrTruncate("  ⚡ Analizando contenedores, exit codes, memoria y volúmenes...", width))
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", width))
		}
		return strings.Join(lines[:height], "\n")
	}

	if report == nil {
		lines = append(lines, padOrTruncate("  ⚠️ No hay informe de diagnóstico disponible. Pulsa [r] para auditar.", width))
		for len(lines) < height {
			lines = append(lines, strings.Repeat(" ", width))
		}
		return strings.Join(lines[:height], "\n")
	}

	// 2. Health Score Bar
	scoreColor := lipgloss.Color("#10B981") // Green
	if report.HealthScore < 65 {
		scoreColor = lipgloss.Color("#EF4444") // Red
	} else if report.HealthScore < 85 {
		scoreColor = lipgloss.Color("#F59E0B") // Yellow
	}

	scoreBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(scoreColor).
		Padding(0, 2).
		Render(fmt.Sprintf("%s SALUD DEL SERVIDOR: %d/100 (%s)", report.HealthBadge, report.HealthScore, report.HealthLabel))

	timeStr := DimStyle.Render(fmt.Sprintf("Último escaneo: %s", report.GeneratedAt.Format("15:04:05")))
	padScore := width - runewidth.StringWidth(stripANSI(scoreBadge)) - runewidth.StringWidth(stripANSI(timeStr))
	scoreLine := scoreBadge
	if padScore > 0 {
		scoreLine += strings.Repeat(" ", padScore) + timeStr
	} else {
		scoreLine += " " + timeStr
	}
	lines = append(lines, padOrTruncate(scoreLine, width))
	lines = append(lines, "")

	// 3. Subsystems Status Grid (Engine, Containers, Storage, Security)
	lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#94A3B8")).Render("── ESTADO DE SUBSISTEMAS ──────────────────────────────────────────"), width))
	for _, sub := range report.Subsystems {
		statusIcon := "🟢"
		subColor := lipgloss.Color("#10B981")
		if sub.Status == autodoctor.SeverityCritical {
			statusIcon = "🔴"
			subColor = lipgloss.Color("#EF4444")
		} else if sub.Status == autodoctor.SeverityWarning {
			statusIcon = "🟡"
			subColor = lipgloss.Color("#F59E0B")
		}
		nameBadge := lipgloss.NewStyle().Bold(true).Foreground(subColor).Render(fmt.Sprintf("%s %-15s", statusIcon, sub.Name))
		subLine := fmt.Sprintf("  %s : %s", nameBadge, sub.Summary)
		lines = append(lines, padOrTruncate(subLine, width))
	}
	lines = append(lines, "")

	// 4. Prioritized Actions / Recommendations (Top 3)
	lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8")).Render("── ACCIONES PRIORITARIAS RECOMENDADAS (TOP 3) ──────────────────────"), width))

	if len(report.TopActions) == 0 {
		lines = append(lines, padOrTruncate("  🎉 ¡Sin problemas detectados! El entorno Docker funciona de forma óptima.", width))
	} else {
		for i, act := range report.TopActions {
			badge := "🔴 CRÍTICO"
			badgeColor := lipgloss.Color("#EF4444")
			if act.Severity == autodoctor.SeverityWarning {
				badge = "🟡 ALERTA"
				badgeColor = lipgloss.Color("#F59E0B")
			} else if act.Severity == autodoctor.SeverityInfo {
				badge = "ℹ️ INFO"
				badgeColor = lipgloss.Color("#38BDF8")
			}

			tag := lipgloss.NewStyle().Bold(true).Foreground(badgeColor).Render(fmt.Sprintf("[%s]", badge))
			actHeader := fmt.Sprintf(" %d. %s %s (%s)", i+1, tag, act.Title, act.Target)
			lines = append(lines, padOrTruncate(actHeader, width))

			cause := fmt.Sprintf("    ↳ Por qué: %s", act.RootCause)
			lines = append(lines, padOrTruncate(DimStyle.Render(cause), width))

			rec := fmt.Sprintf("    ↳ Solución: %s", act.Recommendation)
			lines = append(lines, padOrTruncate(lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Render(rec), width))

			if i < len(report.TopActions)-1 {
				lines = append(lines, "")
			}
		}
	}

	// 5. Historical Trends & Drift
	if len(report.Trends) > 0 {
		lines = append(lines, "")
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Render("── DERIVA Y TENDENCIAS HISTÓRICAS ─────────────────────────────────"), width))
		for _, tr := range report.Trends {
			icon := "📈"
			if tr.IsWarning {
				icon = "⚠️"
			}
			trendLine := fmt.Sprintf("  %s %s (%s): %s", icon, tr.Target, tr.Metric, tr.ChangeText)
			lines = append(lines, padOrTruncate(trendLine, width))
		}
	}

	// 6. Storage Summary callout if reclaimable > 0
	if report.Storage.TotalReclaimableBytes > 0 {
		lines = append(lines, "")
		recStr := docker.FormatBytes(uint64(report.Storage.TotalReclaimableBytes))
		callout := fmt.Sprintf(" 💡 ESPACIO RECUPERABLE: %s en imágenes sin usar y caché. Pulsa [c] para liberar ahora.", recStr)
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render(callout), width))
	}

	// Bottom Status & Navigation
	if statusMsg != "" {
		lines = append(lines, "")
		lines = append(lines, padOrTruncate(lipgloss.NewStyle().Bold(true).Foreground(SuccessColor).Render("⚡ "+statusMsg), width))
	}

	// Ensure exact height lines
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

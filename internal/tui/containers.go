package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

func renderContainersView(containers []types.Container, cursor int, statusMsg string, inspectingJSON string) string {
	if inspectingJSON != "" {
		var s string
		s += SelectedRowStyle.Render("─── INSPECCIÓN DETALLADA DEL CONTENEDOR (JSON) ───") + "\n\n"
		s += inspectingJSON + "\n\n"
		s += HelpBarStyle.Render("[q / Esc: Cerrar inspección]")
		return s
	}

	var s string
	s += fmt.Sprintf("%-14s %-25s %-25s %-16s %-20s\n", "CONTAINER ID", "NAME", "IMAGE", "STATUS", "PORTS")
	s += DimStyle.Render(strings.Repeat("─", 105)) + "\n"

	if len(containers) == 0 {
		s += DimStyle.Render("No se encontraron contenedores en este host.\n")
	}

	for i, c := range containers {
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}

		name := "unnamed"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
			if len(name) > 23 {
				name = name[:20] + "..."
			}
		}

		image := c.Image
		if len(image) > 23 {
			image = image[:20] + "..."
		}

		status := c.Status
		if len(status) > 14 {
			status = status[:14]
		}

		// Port summary
		var portList []string
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				portList = append(portList, fmt.Sprintf("%d:%d", p.PublicPort, p.PrivatePort))
			}
		}
		portsStr := strings.Join(portList, ", ")
		if portsStr == "" {
			portsStr = "-"
		}
		if len(portsStr) > 18 {
			portsStr = portsStr[:16] + "..."
		}

		// Color status
		statusFormatted := status
		if strings.HasPrefix(strings.ToLower(c.State), "running") {
			statusFormatted = StatusRunningStyle.Render(status)
		} else if strings.HasPrefix(strings.ToLower(c.State), "paused") {
			statusFormatted = StatusPausedStyle.Render(status)
		} else {
			statusFormatted = StatusStoppedStyle.Render(status)
		}

		row := fmt.Sprintf("%-14s %-25s %-25s %-16s %-20s", id, name, image, statusFormatted, portsStr)

		if cursor == i {
			s += SelectedRowStyle.Render("▶ ") + row + "\n"
		} else {
			s += "  " + row + "\n"
		}
	}

	if statusMsg != "" {
		s += "\n" + StatusRunningStyle.Render("⚡ "+statusMsg) + "\n"
	}

	s += "\n" + HelpBarStyle.Render("[↑/↓: Mover] [s: Parar] [a: Iniciar] [r: Restart] [p: Pause] [l: Logs] [d: Inspect] [q: Menú]")
	return s
}

func formatInspectJSON(data types.ContainerJSON) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formateando JSON: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 30 {
		return strings.Join(lines[:30], "\n") + fmt.Sprintf("\n... (%d líneas adicionales)", len(lines)-30)
	}
	return string(b)
}

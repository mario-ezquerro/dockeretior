package tui

import (
	"fmt"
	"strings"

	"github.com/mario-ezquerro/dockeretior/internal/compose"
)

func renderComposeView(files []string, selectedProject *compose.ComposeProject, cursor int, statusMsg string, outputText string) string {
	if outputText != "" {
		var s string
		s += SelectedRowStyle.Render("─── SALIDA DEL COMANDO DOCKER COMPOSE ───") + "\n\n"
		s += outputText + "\n\n"
		s += HelpBarStyle.Render("[q / Esc: Volver al explorador Compose]")
		return s
	}

	var s string
	s += "Ficheros Docker Compose detectados en el directorio actual:\n\n"

	if len(files) == 0 {
		s += DimStyle.Render(" No se detectaron archivos docker-compose.yml / compose.yaml en este directorio.\n")
		s += "\n" + HelpBarStyle.Render("[q / Esc: Volver al menú principal]")
		return s
	}

	for i, f := range files {
		prefix := "  "
		line := f
		if cursor == i {
			prefix = SelectedRowStyle.Render("▶ ")
			line = SelectedRowStyle.Render(f)
		}
		s += fmt.Sprintf("%s%s\n", prefix, line)
	}

	if selectedProject != nil {
		s += "\n" + DimStyle.Render(strings.Repeat("─", 80)) + "\n"
		s += HeaderTagStyle.Render(fmt.Sprintf(" Servicios en %s ", selectedProject.FileName)) + "\n\n"

		if len(selectedProject.Services) == 0 {
			s += DimStyle.Render("  (No se encontraron servicios definidos)\n")
		}

		for svcName, svc := range selectedProject.Services {
			s += fmt.Sprintf("  • %s (Image: %s, Restart: %s)\n",
				SelectedRowStyle.Render(svcName),
				svc.Image,
				svc.Restart,
			)
			if len(svc.Ports) > 0 {
				s += fmt.Sprintf("    %s %s\n", DimStyle.Render("Puertos:"), strings.Join(svc.Ports, ", "))
			}
		}
	}

	if statusMsg != "" {
		s += "\n" + StatusRunningStyle.Render("⚡ "+statusMsg) + "\n"
	}

	s += "\n" + HelpBarStyle.Render("[↑/↓: Seleccionar archivo] [Enter: Inspeccionar] [u: Compose Up] [d: Compose Down] [r: Restart] [q: Menú]")
	return s
}

package tui

import (
	"fmt"
	"strings"

	"github.com/mario-ezquerro/dockeretior/internal/compose"
)

func renderComposeView(currentDir string, entries []compose.FileEntry, selectedProject *compose.ComposeProject, cursor int, statusMsg string, outputText string) string {
	if outputText != "" {
		var s string
		s += SelectedRowStyle.Render("─── SALIDA DEL COMANDO DOCKER COMPOSE ───") + "\n\n"
		s += outputText + "\n\n"
		s += HelpBarStyle.Render("[q / Esc: Volver al explorador Compose]")
		return s
	}

	var s string
	s += fmt.Sprintf("%s %s\n\n", DimStyle.Render("📂 Directorio activo:"), SelectedRowStyle.Render(currentDir))

	if len(entries) == 0 {
		s += DimStyle.Render("  (No se encontraron subcarpetas ni archivos .yml / .yaml en esta ruta)\n")
	}

	for i, entry := range entries {
		prefix := "  "
		var line string

		if entry.IsDir {
			line = fmt.Sprintf("📁 %-35s %s", entry.Name, DimStyle.Render("[Carpeta]"))
		} else {
			compTag := DimStyle.Render("(Archivo YAML)")
			if entry.IsCompose {
				compTag = StatusRunningStyle.Render(fmt.Sprintf("[%d servicio(s) detectados]", entry.ServiceCount))
			}
			line = fmt.Sprintf("📄 %-35s %s", entry.Name, compTag)
		}

		if cursor == i {
			prefix = SelectedRowStyle.Render("▶ ")
			line = SelectedRowStyle.Render(line)
		}
		s += fmt.Sprintf("%s%s\n", prefix, line)
	}

	if selectedProject != nil {
		s += "\n" + DimStyle.Render(strings.Repeat("─", 80)) + "\n"
		s += HeaderTagStyle.Render(fmt.Sprintf(" Proyecto: %s (%s) ", selectedProject.FileName, selectedProject.FilePath)) + "\n\n"

		if len(selectedProject.Services) == 0 {
			s += DimStyle.Render("  (No se encontraron servicios definidos en el YAML)\n")
		}

		for svcName, svc := range selectedProject.Services {
			s += fmt.Sprintf("  • %s  %s: %s  %s: %s\n",
				SelectedRowStyle.Render(svcName),
				DimStyle.Render("Image"),
				svc.Image,
				DimStyle.Render("Restart"),
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

	s += "\n" + HelpBarStyle.Render("[↑/↓: Mover] [Enter: Entrar/Inspeccionar] [u: Compose Up -d] [d: Down] [r: Restart] [q: Menú]")
	return s
}

package tui

import (
	"fmt"
)

type menuItem struct {
	Title string
	Desc  string
	State viewState
}

var defaultMenuItems = []menuItem{
	{
		Title: "1. Contenedores Globales (Lifecycle & Logs)",
		Desc:  "Inspecciona, reinicia, detén y visualiza logs de contenedores en ejecución.",
		State: viewContainers,
	},
	{
		Title: "2. Proyectos Docker Compose",
		Desc:  "Detecta archivos compose.yml en el directorio actual y controla sus servicios.",
		State: viewCompose,
	},
	{
		Title: "3. Información del Docker Daemon",
		Desc:  "Consulta versiones de Docker, drivers de almacenamiento y uso global de recursos.",
		State: viewSystemInfo,
	},
	{
		Title: "4. Volver a la Shell en 2º plano (q / Esc)",
		Desc:  "Oculta la TUI y regresa a tu sesión de terminal activa sin cerrarla.",
		State: viewQuit,
	},
	{
		Title: "5. Finalizar y Salir de Dockeretior (Ctrl+Q / Q)",
		Desc:  "Detiene el supervisor Dockeretior y regresa a tu terminal original.",
		State: viewFullExit,
	},
}

func renderMenuView(cursor int) string {
	var s string
	s += "Selecciona una sección para comenzar:\n\n"

	for i, item := range defaultMenuItems {
		cursorStr := "  "
		itemLine := fmt.Sprintf("%-45s %s", item.Title, DimStyle.Render(item.Desc))

		if cursor == i {
			cursorStr = SelectedRowStyle.Render("▶ ")
			itemLine = SelectedRowStyle.Render(fmt.Sprintf("%-45s", item.Title)) + " " + DimStyle.Render(item.Desc)
		}

		s += fmt.Sprintf("%s%s\n", cursorStr, itemLine)
	}

	s += "\n" + HelpBarStyle.Render("[↑/↓: Mover] [Enter: Seleccionar] [q: Ocultar TUI] [Ctrl+Q o Q: Salir del programa]")
	return s
}

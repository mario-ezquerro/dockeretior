package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/mario-ezquerro/dockeretior/internal/compose"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
)

type viewState int

const (
	viewMenu viewState = iota
	viewContainers
	viewCompose
	viewSystemInfo
	viewLogs
	viewInspect
	viewQuit
	viewFullExit
)

// AppModel is the root Bubble Tea model.
type AppModel struct {
	cli                *docker.Client
	state              viewState
	cursor             int
	containers         []types.Container
	composeFiles       []string
	selectedProj       *compose.ComposeProject
	activeLogName      string
	activeLogID        string
	activeLogText      string
	inspectingJSON     string
	composeOutput      string
	sysInfo            system.Info
	statusMsg          string
	fullExitRequested  bool
	err                error
	width              int
	height             int
}

// ShouldExitSupervisor returns true if the user requested a full termination of the supervisor.
func (m AppModel) ShouldExitSupervisor() bool {
	return m.fullExitRequested
}

// NewApp creates a new AppModel.
func NewApp(cli *docker.Client) AppModel {
	cwd, _ := os.Getwd()
	compFiles, _ := compose.FindComposeFiles(cwd)

	m := AppModel{
		cli:          cli,
		state:        viewMenu,
		cursor:       0,
		composeFiles: compFiles,
	}

	if cli != nil {
		list, err := cli.ListContainers(context.Background())
		m.containers = list
		m.err = err

		info, err := cli.ServerInfo(context.Background())
		if err == nil {
			m.sysInfo = info
		}
	}

	return m
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.fullExitRequested = true
			return m, tea.Quit
		}

		key := msg.String()

		// Atajo universal para salir completamente de Dockeretior
		if key == "ctrl+q" || key == "Q" {
			m.fullExitRequested = true
			return m, tea.Quit
		}

		switch key {
		case "q", "esc":
			if m.inspectingJSON != "" {
				m.inspectingJSON = ""
				return m, nil
			}
			if m.composeOutput != "" {
				m.composeOutput = ""
				return m, nil
			}
			if m.state == viewLogs {
				m.state = viewContainers
				return m, nil
			}
			if m.state != viewMenu {
				m.state = viewMenu
				m.cursor = 0
				m.statusMsg = ""
				return m, nil
			}
			// En el menú principal, 'q' oculta la TUI y vuelve al shell
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxIdx := 0
			switch m.state {
			case viewMenu:
				maxIdx = len(defaultMenuItems) - 1
			case viewContainers:
				maxIdx = len(m.containers) - 1
			case viewCompose:
				maxIdx = len(m.composeFiles) - 1
			}
			if m.cursor < maxIdx {
				m.cursor++
			}

		case "enter":
			if m.state == viewMenu {
				switch m.cursor {
				case 0: // Contenedores
					m.refreshContainers()
					m.cursor = 0
					m.state = viewContainers
				case 1: // Compose
					m.cursor = 0
					m.state = viewCompose
					if len(m.composeFiles) > 0 {
						proj, _ := compose.ParseComposeFile(m.composeFiles[0])
						m.selectedProj = proj
					}
				case 2: // Info
					m.refreshSysInfo()
					m.state = viewSystemInfo
				case 3: // Ocultar TUI y volver al shell
					return m, tea.Quit
				case 4: // Salir completamente
					m.fullExitRequested = true
					return m, tea.Quit
				}
			} else if m.state == viewCompose && len(m.composeFiles) > 0 && m.cursor < len(m.composeFiles) {
				proj, err := compose.ParseComposeFile(m.composeFiles[m.cursor])
				if err == nil {
					m.selectedProj = proj
				}
			}

		// Acciones sobre contenedores
		case "s": // Stop
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor].ID
				_ = m.cli.StopContainer(context.Background(), target)
				m.statusMsg = fmt.Sprintf("Contenedor detenido: %s", target[:min(12, len(target))])
				m.refreshContainers()
			}
		case "a": // Start
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor].ID
				_ = m.cli.StartContainer(context.Background(), target)
				m.statusMsg = fmt.Sprintf("Contenedor iniciado: %s", target[:min(12, len(target))])
				m.refreshContainers()
			}
		case "r": // Restart
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor].ID
				_ = m.cli.RestartContainer(context.Background(), target)
				m.statusMsg = fmt.Sprintf("Contenedor reiniciado: %s", target[:min(12, len(target))])
				m.refreshContainers()
			} else if m.state == viewCompose && m.selectedProj != nil {
				out, err := compose.ExecuteCommand(context.Background(), m.selectedProj.FilePath, "restart")
				if err != nil {
					m.composeOutput = fmt.Sprintf("Error: %v\n%s", err, out)
				} else {
					m.composeOutput = out
				}
			} else if m.state == viewLogs && m.activeLogID != "" {
				logs, _ := fetchLogs(m.cli, m.activeLogID, "100")
				m.activeLogText = logs
			}
		case "p": // Pause/Unpause
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor]
				if strings.HasPrefix(strings.ToLower(target.State), "paused") {
					_ = m.cli.UnpauseContainer(context.Background(), target.ID)
					m.statusMsg = fmt.Sprintf("Contenedor reanudado: %s", target.ID[:min(12, len(target.ID))])
				} else {
					_ = m.cli.PauseContainer(context.Background(), target.ID)
					m.statusMsg = fmt.Sprintf("Contenedor pausado: %s", target.ID[:min(12, len(target.ID))])
				}
				m.refreshContainers()
			}
		case "l": // Logs
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				c := m.containers[m.cursor]
				name := c.ID[:min(12, len(c.ID))]
				if len(c.Names) > 0 {
					name = strings.TrimPrefix(c.Names[0], "/")
				}
				m.activeLogName = name
				m.activeLogID = c.ID
				logs, _ := fetchLogs(m.cli, c.ID, "100")
				m.activeLogText = logs
				m.state = viewLogs
			}
		case "d": // Inspect
			if m.state == viewContainers && len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor].ID
				data, err := m.cli.InspectContainer(context.Background(), target)
				if err == nil {
					m.inspectingJSON = formatInspectJSON(data)
				}
			}

		// Acciones Compose
		case "u": // Compose Up
			if m.state == viewCompose && m.selectedProj != nil {
				out, err := compose.ExecuteCommand(context.Background(), m.selectedProj.FilePath, "up", "-d")
				if err != nil {
					m.composeOutput = fmt.Sprintf("Error al arrancar Compose: %v\n%s", err, out)
				} else {
					m.composeOutput = out
				}
			}
		}
	}

	return m, nil
}

func (m *AppModel) refreshContainers() {
	if m.cli != nil {
		list, err := m.cli.ListContainers(context.Background())
		m.containers = list
		m.err = err
	}
}

func (m *AppModel) refreshSysInfo() {
	if m.cli != nil {
		info, err := m.cli.ServerInfo(context.Background())
		if err == nil {
			m.sysInfo = info
		}
	}
}

func (m AppModel) View() string {
	header := HeaderTitleStyle.Render(" DOCKERETIOR ") + " " +
		HeaderTagStyle.Render(" LATENT SUPERVISOR ") + "  " +
		DimStyle.Render("Toggle: [Ctrl+\\] | [Cmd+Opt+Espacio]") + "\n\n"

	if m.err != nil {
		return header + fmt.Sprintf("⚠️  Error conectando con Docker Daemon: %v\n\nPresiona 'q' para volver al shell o 'Q' para salir.", m.err)
	}

	switch m.state {
	case viewMenu:
		return header + renderMenuView(m.cursor)
	case viewContainers:
		return header + renderContainersView(m.containers, m.cursor, m.statusMsg, m.inspectingJSON)
	case viewCompose:
		return header + renderComposeView(m.composeFiles, m.selectedProj, m.cursor, m.statusMsg, m.composeOutput)
	case viewLogs:
		return header + renderLogsView(m.activeLogName, m.activeLogText)
	case viewSystemInfo:
		return header + renderSystemInfoView(m.sysInfo)
	}

	return header
}

func renderSystemInfoView(info system.Info) string {
	var s string
	s += SelectedRowStyle.Render("─── INFORMACIÓN DEL DOCKER DAEMON ───") + "\n\n"
	s += fmt.Sprintf(" • %-25s: %s\n", "Docker Server Version", info.ServerVersion)
	s += fmt.Sprintf(" • %-25s: %s\n", "Sistema Operativo", info.OperatingSystem)
	s += fmt.Sprintf(" • %-25s: %s (%s)\n", "Kernel & Arquitectura", info.KernelVersion, info.Architecture)
	s += fmt.Sprintf(" • %-25s: %d (En ejecución: %d, Pausados: %d, Detenidos: %d)\n",
		"Contenedores Totales", info.Containers, info.ContainersRunning, info.ContainersPaused, info.ContainersStopped)
	s += fmt.Sprintf(" • %-25s: %d\n", "Imágenes Locales", info.Images)
	s += fmt.Sprintf(" • %-25s: %s\n", "Storage Driver", info.Driver)
	s += fmt.Sprintf(" • %-25s: %d CPUs | %.2f GB RAM\n", "Recursos Host", info.NCPU, float64(info.MemTotal)/(1024*1024*1024))
	s += "\n" + HelpBarStyle.Render("[q / Esc: Volver al menú principal]")
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

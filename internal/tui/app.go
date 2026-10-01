package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/mario-ezquerro/dockeretior/internal/compose"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"golang.org/x/term"
)

type viewState int

const (
	viewContainers viewState = iota
	viewCompose
	viewSystemInfo
	viewLogs
	viewInspect
)

type tickMsg time.Time

type metricsMsg struct {
	containerID string
	metrics     *docker.ContainerMetrics
	err         error
}

type actionResultMsg struct {
	message string
	err     error
}

type execFinishedMsg struct {
	containerName string
	err           error
}

// AppModel is the root Bubble Tea model.
type AppModel struct {
	cli                *docker.Client
	state              viewState
	cursor             int
	containers         []types.Container
	filterRunningOnly  bool
	currentMetrics     *docker.ContainerMetrics
	currentDir         string
	composeEntries     []compose.FileEntry
	selectedProj       *compose.ComposeProject
	activeLogName      string
	activeLogID        string
	activeLogText      string
	inspectingJSON     string
	composeOutput      string
	sysInfo            system.Info
	statusMsg          string
	confirmDelete      bool
	showHelp           bool
	fullExitRequested  bool
	err                error
	width              int
	height             int
}

// ShouldExitSupervisor returns true if the user requested a full termination of the supervisor.
func (m AppModel) ShouldExitSupervisor() bool {
	return m.fullExitRequested
}

// NewApp creates a new AppModel. By default, it opens the Containers Dashboard.
func NewApp(cli *docker.Client, initialDir string) AppModel {
	if initialDir == "" {
		initialDir, _ = os.Getwd()
	}

	entries, _ := compose.ScanDirectory(initialDir)

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		w, h, _ = term.GetSize(int(os.Stdin.Fd()))
	}
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}

	m := AppModel{
		cli:               cli,
		state:             viewContainers, // Default view is Containers dashboard!
		filterRunningOnly: true,           // Default: only running containers
		cursor:            0,
		currentDir:        initialDir,
		composeEntries:    entries,
		width:             w,
		height:            h,
	}

	for _, e := range entries {
		if !e.IsDir && e.IsCompose {
			proj, _ := compose.ParseComposeFile(e.Path)
			m.selectedProj = proj
			break
		}
	}

	if cli != nil {
		list, err := cli.ListContainers(context.Background(), false)
		if err == nil && len(list) == 0 {
			// Si no hay contenedores activos, carga automáticamente todos los existentes
			allList, errAll := cli.ListContainers(context.Background(), true)
			if errAll == nil && len(allList) > 0 {
				list = allList
				m.filterRunningOnly = false
				m.statusMsg = "Mostrando todos los contenedores."
			}
		}
		m.containers = list
		m.err = err

		info, err := cli.ServerInfo(context.Background())
		if err == nil {
			m.sysInfo = info
		}

		if len(m.containers) > 0 {
			metrics, _ := cli.FetchContainerMetrics(context.Background(), m.containers[0])
			m.currentMetrics = metrics
		}
	}

	return m
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.tickCmd(), m.fetchSelectedMetricsCmd())
}

func (m AppModel) tickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m AppModel) fetchSelectedMetricsCmd() tea.Cmd {
	if m.cli == nil || len(m.containers) == 0 || m.cursor >= len(m.containers) {
		return nil
	}
	target := m.containers[m.cursor]
	return func() tea.Msg {
		metrics, err := m.cli.FetchContainerMetrics(context.Background(), target)
		return metricsMsg{
			containerID: target.ID,
			metrics:     metrics,
			err:         err,
		}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		var cmd tea.Cmd
		if m.state == viewContainers && !m.confirmDelete && !m.showHelp {
			cmd = m.fetchSelectedMetricsCmd()
		} else if m.state == viewLogs && m.activeLogID != "" {
			logs, _ := fetchLogs(m.cli, m.activeLogID, "100")
			m.activeLogText = logs
		}
		return m, tea.Batch(cmd, m.tickCmd())

	case metricsMsg:
		if len(m.containers) > 0 && m.cursor < len(m.containers) {
			if m.containers[m.cursor].ID == msg.containerID && msg.metrics != nil {
				m.currentMetrics = msg.metrics
			}
		}
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMsg = msg.message
		}
		m.refreshContainers()
		return m, m.fetchSelectedMetricsCmd()

	case execFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Sesión exec finalizada con código: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("✔ Sesión exec en '%s' finalizada.", msg.containerName)
		}
		m.refreshContainers()
		return m, m.fetchSelectedMetricsCmd()

	case tea.KeyMsg:
		// Universal interrupt
		if msg.Type == tea.KeyCtrlC {
			m.fullExitRequested = true
			return m, tea.Quit
		}

		key := strings.ToLower(msg.String())

		// If delete confirmation dialog is open
		if m.confirmDelete {
			switch key {
			case "y", "enter":
				m.confirmDelete = false
				if len(m.containers) > 0 && m.cursor < len(m.containers) {
					target := m.containers[m.cursor]
					name := getContainerName(target)
					return m, func() tea.Msg {
						err := m.cli.RemoveContainer(context.Background(), target.ID, false)
						if err != nil {
							return actionResultMsg{
								message: "",
								err:     fmt.Errorf("no se pudo borrar '%s' (usa 'f' para forzar): %w", name, err),
							}
						}
						return actionResultMsg{
							message: fmt.Sprintf("✔ Contenedor '%s' borrado correctamente.", name),
							err:     nil,
						}
					}
				}
			case "f": // Forzar borrado (-f)
				m.confirmDelete = false
				if len(m.containers) > 0 && m.cursor < len(m.containers) {
					target := m.containers[m.cursor]
					name := getContainerName(target)
					return m, func() tea.Msg {
						err := m.cli.RemoveContainer(context.Background(), target.ID, true)
						if err != nil {
							return actionResultMsg{
								message: "",
								err:     fmt.Errorf("fallo al forzar borrado de '%s': %w", name, err),
							}
						}
						return actionResultMsg{
							message: fmt.Sprintf("✔ Contenedor '%s' forzado y borrado con éxito.", name),
							err:     nil,
						}
					}
				}
			case "n", "esc", "q":
				m.confirmDelete = false
				m.statusMsg = "Operación de borrado cancelada."
				return m, nil
			}
			return m, nil
		}

		// If Help modal is open
		if m.showHelp {
			if key == "esc" || key == "f1" || key == "?" || key == "q" || key == "enter" {
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		// In sub-views (inspect, logs, compose), handle back
		if m.state == viewInspect {
			if key == "q" || key == "esc" || key == "enter" {
				m.state = viewContainers
				return m, nil
			}
			return m, nil
		}
		if m.state == viewLogs {
			if key == "q" || key == "esc" {
				m.state = viewContainers
				return m, nil
			}
			if key == "r" || key == "f5" {
				logs, _ := fetchLogs(m.cli, m.activeLogID, "100")
				m.activeLogText = logs
				return m, nil
			}
			return m, nil
		}
		if m.state == viewCompose {
			if key == "q" || key == "esc" {
				if m.composeOutput != "" {
					m.composeOutput = ""
					return m, nil
				}
				m.state = viewContainers
				return m, nil
			}
			if key == "enter" && len(m.composeEntries) > 0 && m.cursor < len(m.composeEntries) {
				selected := m.composeEntries[m.cursor]
				if selected.IsDir {
					m.currentDir = selected.Path
					m.refreshComposeEntries()
					m.cursor = 0
				} else {
					proj, err := compose.ParseComposeFile(selected.Path)
					if err == nil {
						m.selectedProj = proj
						m.statusMsg = fmt.Sprintf("Archivo cargado: %s (%d servicios)", selected.Name, len(proj.Services))
					} else {
						m.statusMsg = fmt.Sprintf("Error al leer YAML: %v", err)
					}
				}
				return m, nil
			}
			if key == "u" && m.selectedProj != nil {
				out, err := compose.ExecuteCommand(context.Background(), m.selectedProj.FilePath, "up", "-d")
				if err != nil {
					m.composeOutput = fmt.Sprintf("Error Compose up: %v\n%s", err, out)
				} else {
					m.composeOutput = out
				}
				return m, nil
			}
			if key == "d" && m.selectedProj != nil {
				out, err := compose.ExecuteCommand(context.Background(), m.selectedProj.FilePath, "down")
				if err != nil {
					m.composeOutput = fmt.Sprintf("Error Compose down: %v\n%s", err, out)
				} else {
					m.composeOutput = out
				}
				return m, nil
			}
			if (key == "up" || key == "k") && m.cursor > 0 {
				m.cursor--
				return m, nil
			}
			if (key == "down" || key == "j") && m.cursor < len(m.composeEntries)-1 {
				m.cursor++
				return m, nil
			}
			return m, nil
		}

		// --- MAIN CONTAINERS DASHBOARD KEY HANDLERS ---

		// [F1] Ayuda
		if msg.Type == tea.KeyF1 || key == "f1" || key == "?" || key == "1" {
			m.showHelp = true
			return m, nil
		}

		// [F2] Listar / Filtrar (Solo activos vs Todos)
		if msg.Type == tea.KeyF2 || key == "f2" || key == "f" || key == "2" {
			m.filterRunningOnly = !m.filterRunningOnly
			m.refreshContainers()
			if m.filterRunningOnly {
				m.statusMsg = "Filtro: Mostrando únicamente contenedores en ejecución."
			} else {
				m.statusMsg = "Filtro: Mostrando todos los contenedores (activos y detenidos)."
			}
			if m.cursor >= len(m.containers) {
				m.cursor = max(0, len(m.containers)-1)
			}
			return m, m.fetchSelectedMetricsCmd()
		}

		// [F3] Logs en tiempo real
		if msg.Type == tea.KeyF3 || key == "f3" || key == "l" || key == "3" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				c := m.containers[m.cursor]
				name := getContainerName(c)
				m.activeLogName = name
				m.activeLogID = c.ID
				logs, _ := fetchLogs(m.cli, c.ID, "100")
				m.activeLogText = logs
				m.state = viewLogs
			}
			return m, nil
		}

		// [F4] Entrar como exec interactivo (bash o sh)
		if msg.Type == tea.KeyF4 || key == "f4" || key == "e" || key == "4" {
			if len(m.containers) == 0 || m.cursor >= len(m.containers) {
				m.statusMsg = "⚠️ Ningún contenedor seleccionado."
				return m, nil
			}
			target := m.containers[m.cursor]
			name := getContainerName(target)
			if strings.ToLower(target.State) != "running" {
				m.statusMsg = fmt.Sprintf("⚠️ El contenedor '%s' está detenido. Inícialo con [F6] primero.", name)
				return m, nil
			}

			// Launch interactive shell with fallback: bash -> sh
			shellCmd := fmt.Sprintf(
				"if docker exec %s which bash >/dev/null 2>&1; then docker exec -it %s /bin/bash; else docker exec -it %s /bin/sh; fi",
				target.ID, target.ID, target.ID,
			)
			c := exec.Command("sh", "-c", shellCmd)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execFinishedMsg{containerName: name, err: err}
			})
		}

		// [F5] Reiniciar contenedor
		if msg.Type == tea.KeyF5 || key == "f5" || key == "r" || key == "5" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor]
				name := getContainerName(target)
				m.statusMsg = fmt.Sprintf("⚡ Reiniciando '%s'...", name)
				return m, func() tea.Msg {
					err := m.cli.RestartContainer(context.Background(), target.ID)
					if err != nil {
						return actionResultMsg{err: fmt.Errorf("fallo al reiniciar '%s': %w", name, err)}
					}
					return actionResultMsg{message: fmt.Sprintf("✔ Contenedor '%s' reiniciado con éxito.", name)}
				}
			}
			return m, nil
		}

		// [F6] Stop / Start
		if msg.Type == tea.KeyF6 || key == "f6" || key == "s" || key == "a" || key == "6" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor]
				name := getContainerName(target)
				if strings.ToLower(target.State) == "running" {
					m.statusMsg = fmt.Sprintf("⚡ Deteniendo '%s'...", name)
					return m, func() tea.Msg {
						err := m.cli.StopContainer(context.Background(), target.ID)
						if err != nil {
							return actionResultMsg{err: fmt.Errorf("fallo al detener '%s': %w", name, err)}
						}
						return actionResultMsg{message: fmt.Sprintf("✔ Contenedor '%s' detenido.", name)}
					}
				} else {
					m.statusMsg = fmt.Sprintf("⚡ Iniciando '%s'...", name)
					return m, func() tea.Msg {
						err := m.cli.StartContainer(context.Background(), target.ID)
						if err != nil {
							return actionResultMsg{err: fmt.Errorf("fallo al iniciar '%s': %w", name, err)}
						}
						return actionResultMsg{message: fmt.Sprintf("✔ Contenedor '%s' iniciado.", name)}
					}
				}
			}
			return m, nil
		}

		// [F7] Pausar / Reanudar
		if msg.Type == tea.KeyF7 || key == "f7" || key == "p" || key == "7" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor]
				name := getContainerName(target)
				if strings.ToLower(target.State) == "paused" {
					m.statusMsg = fmt.Sprintf("⚡ Reanudando '%s'...", name)
					return m, func() tea.Msg {
						err := m.cli.UnpauseContainer(context.Background(), target.ID)
						if err != nil {
							return actionResultMsg{err: fmt.Errorf("fallo al reanudar '%s': %w", name, err)}
						}
						return actionResultMsg{message: fmt.Sprintf("✔ Contenedor '%s' reanudado.", name)}
					}
				} else {
					m.statusMsg = fmt.Sprintf("⚡ Pausando '%s'...", name)
					return m, func() tea.Msg {
						err := m.cli.PauseContainer(context.Background(), target.ID)
						if err != nil {
							return actionResultMsg{err: fmt.Errorf("fallo al pausar '%s': %w", name, err)}
						}
						return actionResultMsg{message: fmt.Sprintf("✔ Contenedor '%s' pausado.", name)}
					}
				}
			}
			return m, nil
		}

		// [F8] Borrar contenedor (con confirmación)
		if msg.Type == tea.KeyF8 || key == "f8" || key == "x" || key == "8" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				m.confirmDelete = true
			}
			return m, nil
		}

		// [F9] Inspeccionar JSON
		if msg.Type == tea.KeyF9 || key == "f9" || key == "i" || key == "9" {
			if len(m.containers) > 0 && m.cursor < len(m.containers) {
				target := m.containers[m.cursor]
				data, err := m.cli.InspectContainer(context.Background(), target.ID)
				if err == nil {
					m.inspectingJSON = formatInspectJSON(data)
					m.state = viewInspect
				} else {
					m.statusMsg = fmt.Sprintf("Error inspeccionando: %v", err)
				}
			}
			return m, nil
		}

		// [F10] Salir
		if msg.Type == tea.KeyF10 || key == "f10" || key == "q" || key == "0" {
			m.fullExitRequested = true
			return m, tea.Quit
		}

		// Docker Compose view
		if key == "c" {
			m.refreshComposeEntries()
			m.state = viewCompose
			m.cursor = 0
			return m, nil
		}

		// Navegación con cursores
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.statusMsg = ""
				return m, m.fetchSelectedMetricsCmd()
			}
		case "down", "j":
			if m.cursor < len(m.containers)-1 {
				m.cursor++
				m.statusMsg = ""
				return m, m.fetchSelectedMetricsCmd()
			}
		}
	}

	return m, nil
}

func (m *AppModel) refreshContainers() {
	if m.cli != nil {
		list, err := m.cli.ListContainers(context.Background(), !m.filterRunningOnly)
		m.containers = list
		m.err = err
	}
}

func (m *AppModel) refreshComposeEntries() {
	entries, err := compose.ScanDirectory(m.currentDir)
	if err == nil {
		m.composeEntries = entries
	}
}

func (m AppModel) View() string {
	if m.err != nil {
		return HeaderTitleStyle.Render(" DOCKERETIOR ") + "\n\n" +
			fmt.Sprintf("⚠️  Error conectando con Docker Daemon: %v\n\nPresiona [F10] o [q] para salir.", m.err)
	}

	switch m.state {
	case viewContainers:
		return renderContainersDashboard(
			m.containers,
			m.cursor,
			m.currentMetrics,
			m.filterRunningOnly,
			m.statusMsg,
			m.confirmDelete,
			m.showHelp,
			m.width,
			m.height,
		)
	case viewInspect:
		return renderInspectView(m.inspectingJSON, m.width, m.height)
	case viewLogs:
		return renderLogsView(m.activeLogName, m.activeLogText)
	case viewCompose:
		return renderComposeView(m.currentDir, m.composeEntries, m.selectedProj, m.cursor, m.statusMsg, m.composeOutput)
	case viewSystemInfo:
		return renderSystemInfoView(m.sysInfo)
	}

	return ""
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
	s += "\n" + HelpBarStyle.Render("[q / Esc: Volver a la lista de contenedores]")
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

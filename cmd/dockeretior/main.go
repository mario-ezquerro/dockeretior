package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/mario-ezquerro/dockeretior/internal/alerts"
	"github.com/mario-ezquerro/dockeretior/internal/autodoctor"
	"github.com/mario-ezquerro/dockeretior/internal/compose"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mario-ezquerro/dockeretior/internal/tui"
	"golang.org/x/term"
)

// Supported hotkey trigger sequences
var (
	// Ctrl + Alt/Option + Space variations (ESC + NUL, ESC + Space, ESC + NBSP)
	seqCtrlAltSpace1 = []byte{0x1b, 0x00}
	seqCtrlAltSpace2 = []byte{0x1b, 0x20}
	seqCtrlAltSpace3 = []byte{0x1b, 0xc2, 0xa0}

	// Ctrl + Space (ASCII NUL)
	seqCtrlSpace = []byte{0x00}

	// Ctrl + \ (ASCII FS 0x1c - Universal Unix/macOS escape key)
	seqCtrlBackslash = []byte{0x1c}

	// Ctrl + ] (ASCII GS 0x1d)
	seqCtrlBracket = []byte{0x1d}

	// F12 key escape sequences across various terminal emulators
	seqF12Standard = []byte{0x1b, '[', '2', '4', '~'}
	seqF12VT       = []byte{0x1b, '[', '1', '1', '~'}
	seqF12SS3      = []byte{0x1b, 'O', 'S'}
)

func isTriggerSequence(b []byte) bool {
	if len(b) == 0 {
		return false
	}

	// Ctrl + \ (Universal & recommended for macOS)
	if bytes.Equal(b, seqCtrlBackslash) {
		return true
	}

	// Ctrl + Space
	if bytes.Equal(b, seqCtrlSpace) {
		return true
	}

	// Ctrl + ]
	if bytes.Equal(b, seqCtrlBracket) {
		return true
	}

	// Ctrl + Alt + Space sequences
	if len(b) >= 2 && bytes.Equal(b[:2], seqCtrlAltSpace1) {
		return true
	}
	if len(b) >= 2 && bytes.Equal(b[:2], seqCtrlAltSpace2) {
		return true
	}
	if len(b) >= 3 && bytes.Equal(b[:3], seqCtrlAltSpace3) {
		return true
	}

	// F12 sequences
	if bytes.Equal(b, seqF12Standard) || bytes.Equal(b, seqF12VT) || bytes.Equal(b, seqF12SS3) {
		return true
	}

	return false
}

type Supervisor struct {
	ptmx       *os.File
	inMenu     bool
	mu         sync.Mutex
	tuiProgram *tea.Program
	tuiStdinR  *io.PipeReader
	tuiStdinW  *io.PipeWriter
	dockerCli  *docker.Client
	cmd        *exec.Cmd
}

func main() {
	supervisorFlag := flag.Bool("supervisor", false, "Inicia como supervisor PTY latente en segundo plano (Hot-Toggle)")
	flag.BoolVar(supervisorFlag, "s", false, "Alias de --supervisor")
	_ = flag.Bool("tui", true, "Lanza directamente la interfaz TUI (activo por defecto)")
	debugKeys := flag.Bool("debug-keys", false, "Modo diagnóstico: muestra los bytes exactos enviados por el teclado")
	doctorFlag := flag.Bool("doctor", false, "Ejecuta AutoDoctor y muestra el informe de salud del servidor en consola")
	composeFlag := flag.Bool("compose", false, "Analiza y muestra el mapa topológico del archivo Compose local")
	testAlertFlag := flag.Bool("test-alert", false, "Envía una notificación de prueba a los webhooks/Telegram configurados")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dockeretior")
	flag.Parse()

	if *versionFlag {
		fmt.Println("dockeretior v1.3.0 (Interactive Docker Dashboard, AutoDoctor & Compose Topology)")
		return
	}

	// Modo diagnóstico de teclas para terminales
	if *debugKeys {
		runDebugKeys()
		return
	}

	// Prueba de envío de alertas
	if *testAlertFlag {
		cfg := alerts.LoadAlertConfig("")
		fmt.Printf("Configuración de alertas (~/.dockeretior/alerts.json):\n")
		fmt.Printf(" • Servidor : %s\n", cfg.ServerName)
		fmt.Printf(" • Webhook  : %s\n", cfg.WebhookURL)
		fmt.Printf(" • Telegram : Bot=%t, ChatID=%s\n", cfg.TelegramBotToken != "", cfg.TelegramChatID)
		if cfg.WebhookURL == "" && cfg.TelegramBotToken == "" {
			fmt.Println("⚠️  No hay ningún Webhook ni bot de Telegram configurado en ~/.dockeretior/alerts.json.")
			fmt.Println("   Edita el archivo y define \"webhook_url\" o \"telegram_bot_token\" / \"telegram_chat_id\".")
			return
		}
		payload := alerts.AlertPayload{
			ServerName:     cfg.ServerName,
			Title:          "Prueba de Notificación Dockeretior",
			Target:         "Docker Host",
			Severity:       autodoctor.SeverityInfo,
			RootCause:      "Comprobación manual de conectividad de alertas.",
			Recommendation: "Las notificaciones automáticas están operativas.",
			Timestamp:      time.Now(),
		}
		testCfg := cfg
		testCfg.Enabled = true
		if err := alerts.DispatchAlert(context.Background(), testCfg, payload); err != nil {
			fmt.Printf("❌ Error enviando alerta: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✔ Alerta de prueba enviada con éxito.")
		return
	}

	// Modo Compose Topológico directo en CLI
	if *composeFlag {
		cwd, _ := os.Getwd()
		entries, err := compose.ScanDirectory(cwd)
		if err != nil {
			fmt.Printf("❌ Error explorando directorio: %v\n", err)
			os.Exit(1)
		}
		var foundPath string
		for _, e := range entries {
			if !e.IsDir && e.IsCompose {
				foundPath = e.Path
				break
			}
		}
		if foundPath == "" {
			fmt.Println("⚠️  No se encontró ningún archivo Compose (.yml / .yaml) en el directorio actual.")
			os.Exit(1)
		}
		stack, err := compose.ParseComposeStack(foundPath)
		if err != nil {
			fmt.Printf("❌ Error al procesar archivo compose: %v\n", err)
			os.Exit(1)
		}
		graph := compose.BuildTopologyGraph(stack)
		fmt.Print(compose.RenderTopologyASCII(stack, graph, 90))
		return
	}

	dockerCli, err := docker.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Aviso: Docker socket no detectado (%v). Ejecutando en modo desconectado.\n", err)
	}
	defer func() {
		if dockerCli != nil {
			_ = dockerCli.Close()
		}
	}()

	// Modo AutoDoctor directo en CLI (sin entrar en TUI interactiva)
	if *doctorFlag {
		if dockerCli == nil {
			fmt.Println("❌ Error: No se puede conectar al socket de Docker.")
			os.Exit(1)
		}
		rep, err := autodoctor.RunAudit(context.Background(), dockerCli)
		if err != nil {
			fmt.Printf("❌ Error ejecutando diagnóstico: %v\n", err)
			os.Exit(1)
		}
		printDoctorCLIReport(rep)
		return
	}

	// Por defecto, ejecuta la interfaz TUI interactiva a pantalla completa
	if !*supervisorFlag {
		cwd, _ := os.Getwd()
		p := tea.NewProgram(tui.NewApp(dockerCli, cwd), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error ejecutando TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Supervisor PTY latente (si se especifica con --supervisor o -s)
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al iniciar pseudo-terminal (PTY): %v\n", err)
		os.Exit(1)
	}
	defer ptmx.Close()

	// Mantener tamaño del terminal sincronizado (SIGWINCH)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go func() {
		for range sigChan {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigChan <- syscall.SIGWINCH

	// Poner stdin del host en modo RAW
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error configurando modo raw en terminal: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	sup := &Supervisor{
		ptmx:      ptmx,
		dockerCli: dockerCli,
		cmd:       cmd,
	}

	// Mensaje inicial discreto
	welcomeBanner := "\r\n\x1b[38;5;99m⚓ Dockeretior Supervisor activo\x1b[0m \x1b[90m(Toggle: [Ctrl+\\] o [Cmd+Opt+Espacio] | Salir: 'exit' o Ctrl+Q en TUI)\x1b[0m\r\n"
	os.Stdout.WriteString(welcomeBanner)

	// Hilo 1: Salida de la Shell -> Consola (solo cuando la TUI no está activa)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			sup.mu.Lock()
			if !sup.inMenu {
				_, _ = os.Stdout.Write(buf[:n])
			}
			sup.mu.Unlock()
		}
	}()

	// Hilo 2: Entrada del Teclado -> Hot-Toggle o Shell
	inputBuf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(inputBuf)
		if err != nil {
			break
		}

		if isTriggerSequence(inputBuf[:n]) {
			sup.toggle()
			continue
		}

		sup.mu.Lock()
		if sup.inMenu {
			if sup.tuiStdinW != nil {
				_, _ = sup.tuiStdinW.Write(inputBuf[:n])
			}
		} else {
			_, _ = ptmx.Write(inputBuf[:n])
		}
		sup.mu.Unlock()
	}

	_ = cmd.Wait()
}

func (s *Supervisor) toggle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.inMenu {
		s.inMenu = true

		// Detectar dinámicamente la ruta en la que está el usuario en la shell
		pid := 0
		if s.cmd != nil && s.cmd.Process != nil {
			pid = s.cmd.Process.Pid
		}
		currentDir := compose.GetProcessCwd(pid)

		// Cambiar al buffer de pantalla alternativo ANSI
		os.Stdout.WriteString("\x1b[?1049h\x1b[H")

		s.tuiStdinR, s.tuiStdinW = io.Pipe()
		s.tuiProgram = tea.NewProgram(
			tui.NewApp(s.dockerCli, currentDir),
			tea.WithInput(s.tuiStdinR),
			tea.WithOutput(os.Stdout),
		)

		// Enviar dimensiones iniciales del terminal para renderizado perfecto
		if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			go func() {
				time.Sleep(20 * time.Millisecond)
				s.tuiProgram.Send(tea.WindowSizeMsg{Width: w, Height: h})
			}()
		}

		go func(p *tea.Program) {
			finalModel, _ := p.Run()
			s.mu.Lock()
			if s.inMenu {
				s.inMenu = false
				os.Stdout.WriteString("\x1b[?1049l")
			}

			// Si el usuario seleccionó "Salir completamente" (opción 5 o Ctrl+Q / Q)
			if appModel, ok := finalModel.(tui.AppModel); ok && appModel.ShouldExitSupervisor() {
				s.mu.Unlock()
				_ = s.ptmx.Close()
				if s.cmd != nil && s.cmd.Process != nil {
					_ = s.cmd.Process.Kill()
				}
				os.Stdout.WriteString("\r\n\x1b[32m✔ Dockeretior finalizado con éxito.\x1b[0m\r\n")
				os.Exit(0)
			}
			s.mu.Unlock()
		}(s.tuiProgram)
	} else {
		s.inMenu = false
		if s.tuiProgram != nil {
			s.tuiProgram.Quit()
		}
		if s.tuiStdinW != nil {
			s.tuiStdinW.Close()
		}
		os.Stdout.WriteString("\x1b[?1049l")
	}
}

func runDebugKeys() {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Error modo raw: %v\n", err)
		return
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	fmt.Print("\r\n=== MODO DIAGNÓSTICO DE TECLADO DOCKERETIOR ===\r\n")
	fmt.Print("Pulsa cualquier tecla o combinación para ver los bytes que recibe el terminal.\r\n")
	fmt.Print("Pulsa 'Ctrl+C' o 'q' para salir.\r\n\r\n")

	buf := make([]byte, 128)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		chunk := buf[:n]
		if len(chunk) == 1 && (chunk[0] == 0x03 || chunk[0] == 'q') {
			fmt.Print("\r\nSaliendo del modo diagnóstico...\r\n")
			break
		}

		hexStr := ""
		for _, b := range chunk {
			hexStr += fmt.Sprintf("0x%02x ", b)
		}

		matched := "No coincide con trigger"
		if isTriggerSequence(chunk) {
			matched = "\x1b[32m¡COINCIDE CON ACTIVACIÓN DOCKERETIOR!\x1b[0m"
		}

		fmt.Printf("\rBytes: %d | Hex: [ %s] | %s\r\n", n, hexStr, matched)
	}
}

func printDoctorCLIReport(rep *autodoctor.HealthReport) {
	fmt.Println("================================================================================")
	fmt.Printf("🩺 AUTODOCTOR - INFORME DE SALUD DEL SERVIDOR\n")
	fmt.Println("================================================================================")
	fmt.Printf("Salud Global : %d/100 %s %s\n", rep.HealthScore, rep.HealthBadge, rep.HealthLabel)
	fmt.Printf("Contenedores : %d en ejecución, %d detenidos (%d en total)\n", rep.ContainersRunning, rep.ContainersStopped, rep.ContainersTotal)
	fmt.Printf("Generado     : %s\n", rep.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("ESTADO DE SUBSISTEMAS:")
	for _, sub := range rep.Subsystems {
		icon := "🟢"
		if sub.Status == autodoctor.SeverityCritical {
			icon = "🔴"
		} else if sub.Status == autodoctor.SeverityWarning {
			icon = "🟡"
		}
		fmt.Printf("  %s %-15s : %s\n", icon, sub.Name, sub.Summary)
	}
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("ACCIONES PRIORITARIAS RECOMENDADAS (TOP 3):")
	if len(rep.TopActions) == 0 {
		fmt.Println("  🎉 ¡Sin problemas detectados! El entorno Docker funciona de forma óptima.")
	} else {
		for i, act := range rep.TopActions {
			badge := "🔴 CRÍTICO"
			if act.Severity == autodoctor.SeverityWarning {
				badge = "🟡 ALERTA"
			} else if act.Severity == autodoctor.SeverityInfo {
				badge = "ℹ️ INFO"
			}
			fmt.Printf("  %d. [%s] %s (%s)\n", i+1, badge, act.Title, act.Target)
			fmt.Printf("     ↳ Por qué : %s\n", act.RootCause)
			fmt.Printf("     ↳ Solución: %s\n", act.Recommendation)
			if act.RemediationCommand != "" {
				fmt.Printf("     ↳ Comando : %s\n", act.RemediationCommand)
			}
		}
	}
	if len(rep.Trends) > 0 {
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println("DERIVA Y TENDENCIAS HISTÓRICAS:")
		for _, tr := range rep.Trends {
			icon := "📈"
			if tr.IsWarning {
				icon = "⚠️"
			}
			fmt.Printf("  %s %s (%s): %s\n", icon, tr.Target, tr.Metric, tr.ChangeText)
		}
	}
	if rep.Storage.TotalReclaimableBytes > 0 {
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Printf("💡 ESPACIO RECUPERABLE: %s acumulados en imágenes sin usar y caché de Docker.\n",
			docker.FormatBytes(uint64(rep.Storage.TotalReclaimableBytes)))
		fmt.Printf("   Para liberar espacio ejecuta: dockeretior (pulsa 'a' y luego 'c')\n")
		fmt.Printf("   o por consola: docker system prune -a --volumes\n")
	}
	fmt.Println("================================================================================")
}

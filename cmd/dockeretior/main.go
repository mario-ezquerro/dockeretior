package main

import (
	"bytes"
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
}

func main() {
	onlyTUI := flag.Bool("tui", false, "Lanza directamente la interfaz TUI a pantalla completa")
	debugKeys := flag.Bool("debug-keys", false, "Modo diagnóstico: muestra los bytes exactos enviados por el teclado")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dockeretior")
	flag.Parse()

	if *versionFlag {
		fmt.Println("dockeretior v1.0.0 (Latent PTY Docker Supervisor)")
		return
	}

	// Modo diagnóstico de teclas para terminales
	if *debugKeys {
		runDebugKeys()
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

	// Modo TUI directo si se solicita con --tui
	if *onlyTUI {
		p := tea.NewProgram(tui.NewApp(dockerCli), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error ejecutando TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Supervisor PTY
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
	}

	// Mensaje inicial discreto
	welcomeBanner := "\r\n\x1b[38;5;99m⚓ Dockeretior Supervisor activo\x1b[0m \x1b[90m(Atajos de activación: [Ctrl+\\] o [Ctrl+Alt+Espacio] o [Ctrl+Space])\x1b[0m\r\n"
	os.Stdout.WriteString(welcomeBanner)

	// Hilo 1: Salida de la Shell -> Consola (solo cuando la TUI está oculta)
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

		// Cambiar al buffer de pantalla alternativo ANSI
		os.Stdout.WriteString("\x1b[?1049h\x1b[H")

		s.tuiStdinR, s.tuiStdinW = io.Pipe()
		s.tuiProgram = tea.NewProgram(
			tui.NewApp(s.dockerCli),
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
			_, _ = p.Run()
			s.mu.Lock()
			if s.inMenu {
				s.inMenu = false
				os.Stdout.WriteString("\x1b[?1049l")
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

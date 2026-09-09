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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/mario-ezquerro/dockeretior/internal/docker"
	"github.com/mario-ezquerro/dockeretior/internal/tui"
	"golang.org/x/term"
)

// Trigger sequence for Ctrl + Alt/Option + Space: ESC (\x1b) followed by NUL (\x00)
var triggerSequence = []byte{0x1b, 0x00}

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
	onlyTUI := flag.Bool("tui", false, "Lanza directamente la interfaz TUI sin el proxy PTY")
	versionFlag := flag.Bool("version", false, "Muestra la versión de dockeretior")
	flag.Parse()

	if *versionFlag {
		fmt.Println("dockeretior v1.0.0 (Latent PTY Docker Supervisor)")
		return
	}

	dockerCli, err := docker.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Aviso: No se pudo conectar al socket Docker (%v). La TUI funcionará en modo degradado.\n", err)
	}
	defer func() {
		if dockerCli != nil {
			_ = dockerCli.Close()
		}
	}()

	// Modo TUI directo si se pide explícitamente
	if *onlyTUI {
		p := tea.NewProgram(tui.NewApp(dockerCli), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error ejecutando TUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Iniciar PTY proxy transparente
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

	// Sincronizar tamaño del terminal dinámicamente con SIGWINCH
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go func() {
		for range sigChan {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigChan <- syscall.SIGWINCH

	// Poner el terminal del host en modo RAW
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error estableciendo modo raw en terminal: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	sup := &Supervisor{
		ptmx:      ptmx,
		dockerCli: dockerCli,
	}

	// Goroutine 1: Salida de la Shell -> Consola real (solo cuando la TUI no está activa)
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

	// Goroutine 2: Entrada del Teclado -> Detección de Hot-Toggle o Shell
	inputBuf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(inputBuf)
		if err != nil {
			break
		}

		// Detectar Ctrl + Alt/Option + Espacio (\x1b\x00)
		if n >= 2 && bytes.Equal(inputBuf[:2], triggerSequence) {
			sup.toggle()
			continue
		}

		sup.mu.Lock()
		if sup.inMenu {
			// Redirigir pulsaciones a la TUI de Bubble Tea
			if sup.tuiStdinW != nil {
				_, _ = sup.tuiStdinW.Write(inputBuf[:n])
			}
		} else {
			// Redirigir pulsaciones a la Shell habitual
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
		// Activar TUI y cambiar al buffer de pantalla alternativo
		s.inMenu = true
		os.Stdout.WriteString("\x1b[?1049h\x1b[H")

		s.tuiStdinR, s.tuiStdinW = io.Pipe()
		s.tuiProgram = tea.NewProgram(
			tui.NewApp(s.dockerCli),
			tea.WithInput(s.tuiStdinR),
			tea.WithOutput(os.Stdout),
		)

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
		// Ocultar TUI y volver a la Shell intacta
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

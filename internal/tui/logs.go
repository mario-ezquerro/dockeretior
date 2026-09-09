package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mario-ezquerro/dockeretior/internal/docker"
)

func fetchLogs(cli *docker.Client, containerID string, tail string) (string, error) {
	if cli == nil {
		return "", fmt.Errorf("docker client not available")
	}

	reader, err := cli.GetContainerLogs(context.Background(), containerID, tail)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Strip docker log headers (8-byte prefix per frame) or read as string
	buf := new(bytes.Buffer)
	rawBuf := make([]byte, 4096)
	for {
		n, err := reader.Read(rawBuf)
		if n > 0 {
			// Clean docker multiplexed header if present
			chunk := rawBuf[:n]
			for len(chunk) > 8 && (chunk[0] == 1 || chunk[0] == 2) && chunk[1] == 0 && chunk[2] == 0 && chunk[3] == 0 {
				chunk = chunk[8:]
			}
			buf.Write(chunk)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

	lines := strings.Split(buf.String(), "\n")
	if len(lines) > 50 {
		return strings.Join(lines[len(lines)-50:], "\n"), nil
	}
	return buf.String(), nil
}

func renderLogsView(containerName string, logContent string) string {
	var s string
	s += SelectedRowStyle.Render(fmt.Sprintf("─── LOGS EN VIVO :: %s ───", containerName)) + "\n\n"

	if logContent == "" {
		s += DimStyle.Render("(No se generaron logs aún)\n")
	} else {
		s += logContent + "\n"
	}

	s += "\n" + HelpBarStyle.Render("[r: Refrescar] [q / Esc: Volver a la lista de contenedores]")
	return s
}

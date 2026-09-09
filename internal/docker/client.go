package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

// Client wraps the official Docker engine client.
type Client struct {
	cli *client.Client
}

// detectDockerSocket looks for standard Docker sockets across Linux, macOS (Intel & Apple Silicon), Docker Desktop, OrbStack, and Colima.
func detectDockerSocket() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}

	homeDir, _ := os.UserHomeDir()
	candidateSockets := []string{
		"/var/run/docker.sock",
	}

	if homeDir != "" {
		// macOS Docker Desktop socket
		candidateSockets = append(candidateSockets, filepath.Join(homeDir, ".docker", "run", "docker.sock"))
		// macOS OrbStack socket
		candidateSockets = append(candidateSockets, filepath.Join(homeDir, ".orbstack", "run", "docker.sock"))
		// macOS Colima socket
		candidateSockets = append(candidateSockets, filepath.Join(homeDir, ".colima", "default", "docker.sock"))
		// Linux rootless socket
		if runtime.GOOS == "linux" {
			uid := os.Getuid()
			candidateSockets = append(candidateSockets, fmt.Sprintf("/run/user/%d/docker.sock", uid))
		}
	}

	for _, sock := range candidateSockets {
		if fi, err := os.Stat(sock); err == nil && (fi.Mode()&os.ModeSocket != 0 || fi.Mode().IsRegular()) {
			// Test connectivity with a fast dial
			c, err := net.DialTimeout("unix", sock, 300*time.Millisecond)
			if err == nil {
				_ = c.Close()
				return "unix://" + sock
			}
		}
	}

	return ""
}

// NewClient initializes a new Docker client with multi-platform socket detection.
func NewClient() (*Client, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	if customHost := detectDockerSocket(); customHost != "" && os.Getenv("DOCKER_HOST") == "" {
		opts = append(opts, client.WithHost(customHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Quick health check / ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ping docker daemon: %w", err)
	}

	return &Client{cli: cli}, nil
}

// ListContainers returns all containers (both running and stopped).
func (c *Client) ListContainers(ctx context.Context) ([]types.Container, error) {
	return c.cli.ContainerList(ctx, container.ListOptions{All: true})
}

// RestartContainer restarts a container by its ID or name.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	stopTimeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &stopTimeout})
}

// StopContainer stops a running container.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	stopTimeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &stopTimeout})
}

// StartContainer starts a stopped container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// PauseContainer pauses processes in a running container.
func (c *Client) PauseContainer(ctx context.Context, id string) error {
	return c.cli.ContainerPause(ctx, id)
}

// UnpauseContainer resumes processes in a paused container.
func (c *Client) UnpauseContainer(ctx context.Context, id string) error {
	return c.cli.ContainerUnpause(ctx, id)
}

// RemoveContainer removes a container.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

// InspectContainer returns full detailed JSON inspection struct.
func (c *Client) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return c.cli.ContainerInspect(ctx, id)
}

// GetContainerLogs returns a stream of stdout/stderr logs for a given container.
func (c *Client) GetContainerLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       tail,
		Timestamps: true,
	}
	return c.cli.ContainerLogs(ctx, id, opts)
}

// ServerInfo returns system information from the Docker daemon.
func (c *Client) ServerInfo(ctx context.Context) (system.Info, error) {
	return c.cli.Info(ctx)
}

// Close closes the underlying docker client transport.
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

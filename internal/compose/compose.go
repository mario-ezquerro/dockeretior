package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeProject represents a detected Docker Compose project.
type ComposeProject struct {
	FilePath string            `json:"file_path"`
	FileName string            `json:"file_name"`
	Services map[string]Service `json:"services"`
}

// Service represents a parsed service definition inside compose.
type Service struct {
	Image       string            `yaml:"image"`
	ContainerName string          `yaml:"container_name"`
	Ports       []string          `yaml:"ports"`
	Environment yaml.Node         `yaml:"environment"`
	Restart     string            `yaml:"restart"`
}

type rawCompose struct {
	Services map[string]Service `yaml:"services"`
}

// FindComposeFiles looks for compose YAML files in the given directory.
func FindComposeFiles(dir string) ([]string, error) {
	patterns := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
		"*compose*.yml",
		"*compose*.yaml",
	}

	foundMap := make(map[string]bool)
	var results []string

	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, p))
		if err == nil {
			for _, m := range matches {
				if !foundMap[m] {
					foundMap[m] = true
					results = append(results, m)
				}
			}
		}
	}

	return results, nil
}

// ParseComposeFile reads and parses a compose file.
func ParseComposeFile(path string) (*ComposeProject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read compose file: %w", err)
	}

	var raw rawCompose
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compose YAML: %w", err)
	}

	return &ComposeProject{
		FilePath: path,
		FileName: filepath.Base(path),
		Services: raw.Services,
	}, nil
}

// ExecuteCommand runs a docker compose subcommand against a specific compose file.
func ExecuteCommand(ctx context.Context, composeFile string, args ...string) (string, error) {
	cmdArgs := []string{"compose", "-f", composeFile}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("compose command failed (%v): %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

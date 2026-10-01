package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeProject represents a detected Docker Compose project.
type ComposeProject struct {
	FilePath string             `json:"file_path"`
	FileName string             `json:"file_name"`
	Services map[string]Service `json:"services"`
	Stack    *ComposeStack      `json:"stack,omitempty"`
	Graph    *TopologyGraph     `json:"graph,omitempty"`
}

// Service represents a parsed service definition inside compose.
type Service struct {
	Image         string    `yaml:"image"`
	ContainerName string    `yaml:"container_name"`
	Ports         []string  `yaml:"ports"`
	Environment   yaml.Node `yaml:"environment"`
	Restart       string    `yaml:"restart"`
}

type rawCompose struct {
	Services map[string]Service `yaml:"services"`
}

// FileEntry represents an item in the directory explorer.
type FileEntry struct {
	Name        string
	Path        string
	IsDir       bool
	IsCompose   bool
	ServiceCount int
}

// GetProcessCwd discovers the live current working directory of a child process (macOS / Linux).
func GetProcessCwd(pid int) string {
	if pid > 0 {
		if runtime.GOOS == "linux" {
			if target, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
				return target
			}
		}
		if runtime.GOOS == "darwin" {
			out, err := exec.Command("lsof", "-a", "-p", fmt.Sprintf("%d", pid), "-d", "cwd", "-Fn").Output()
			if err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "n") && len(line) > 1 {
						return strings.TrimSpace(line[1:])
					}
				}
			}
		}
	}
	cwd, _ := os.Getwd()
	return cwd
}

// ScanDirectory lists files, subfolders and any YAML compose files in the target directory.
func ScanDirectory(dir string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var results []FileEntry

	// Add parent directory option
	parentDir := filepath.Dir(dir)
	if parentDir != dir {
		results = append(results, FileEntry{
			Name:  "..",
			Path:  parentDir,
			IsDir: true,
		})
	}

	// 1. First add subdirectories
	for _, entry := range entries {
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				continue // Skip hidden directories like .git
			}
			results = append(results, FileEntry{
				Name:  entry.Name() + "/",
				Path:  filepath.Join(dir, entry.Name()),
				IsDir: true,
			})
		}
	}

	// 2. Add YAML and Compose files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yml" || ext == ".yaml" {
			fullPath := filepath.Join(dir, name)
			proj, parseErr := ParseComposeFile(fullPath)
			svcCount := 0
			isComp := false
			if parseErr == nil && proj != nil && len(proj.Services) > 0 {
				isComp = true
				svcCount = len(proj.Services)
			}
			results = append(results, FileEntry{
				Name:         name,
				Path:         fullPath,
				IsDir:        false,
				IsCompose:    isComp,
				ServiceCount: svcCount,
			})
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

	proj := &ComposeProject{
		FilePath: path,
		FileName: filepath.Base(path),
		Services: raw.Services,
	}

	if stack, err := ParseComposeStack(path); err == nil {
		proj.Stack = stack
		proj.Graph = BuildTopologyGraph(stack)
	}

	return proj, nil
}

// ExecuteCommand runs a docker compose subcommand against a specific compose file.
func ExecuteCommand(ctx context.Context, composeFile string, args ...string) (string, error) {
	cmdArgs := []string{"compose", "-f", composeFile}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	// Run command inside the compose file directory
	cmd.Dir = filepath.Dir(composeFile)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("compose command failed (%v): %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

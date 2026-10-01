package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RawComposeFile is an intermediate struct for parsing Compose YAML.
type RawComposeFile struct {
	Version  string               `yaml:"version,omitempty"`
	Services map[string]yaml.Node `yaml:"services"`
	Networks yaml.Node            `yaml:"networks"`
	Volumes  yaml.Node            `yaml:"volumes"`
}

// ParseComposeStack reads and parses a compose file into a structured ComposeStack.
func ParseComposeStack(path string) (*ComposeStack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", path, err)
	}

	var raw RawComposeFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("error unmarshaling compose file: %w", err)
	}

	stack := &ComposeStack{
		FilePath: path,
		FileName: filepath.Base(path),
		Services: make(map[string]ServiceInfo),
		Networks: extractKeysOrList(&raw.Networks),
		Volumes:  extractKeysOrList(&raw.Volumes),
	}

	for svcName, node := range raw.Services {
		nodeCopy := node
		svc := parseServiceNode(svcName, &nodeCopy)
		stack.Services[svcName] = svc
	}

	return stack, nil
}

func parseServiceNode(name string, node *yaml.Node) ServiceInfo {
	svc := ServiceInfo{
		Name:        name,
		Environment: make(map[string]string),
	}

	if node == nil || node.Kind != yaml.MappingNode {
		return svc
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valNode := node.Content[i+1]

		switch key {
		case "image":
			svc.Image = valNode.Value
		case "container_name":
			svc.ContainerName = valNode.Value
		case "command":
			if valNode.Kind == yaml.ScalarNode {
				svc.Command = valNode.Value
			} else if valNode.Kind == yaml.SequenceNode {
				var parts []string
				for _, p := range valNode.Content {
					parts = append(parts, p.Value)
				}
				svc.Command = strings.Join(parts, " ")
			}
		case "restart":
			svc.Restart = valNode.Value
		case "ports":
			svc.Ports = extractStringSequenceOrObjects(valNode)
		case "volumes":
			svc.Volumes = extractStringSequenceOrObjects(valNode)
		case "networks":
			svc.Networks = extractKeysOrList(valNode)
		case "depends_on":
			svc.DependsOn = parseDependsOn(valNode)
		case "environment":
			svc.Environment = parseEnvironment(valNode)
		case "healthcheck":
			svc.Healthcheck = parseHealthcheck(valNode)
		}
	}

	return svc
}

func parseDependsOn(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	var deps []string
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				deps = append(deps, item.Value)
			}
		}
	} else if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			depName := node.Content[i].Value
			deps = append(deps, depName)
		}
	}
	return deps
}

func parseEnvironment(node *yaml.Node) map[string]string {
	env := make(map[string]string)
	if node == nil {
		return env
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			k := node.Content[i].Value
			v := node.Content[i+1].Value
			env[k] = v
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			val := item.Value
			parts := strings.SplitN(val, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			} else if len(parts) == 1 {
				env[parts[0]] = ""
			}
		}
	}
	return env
}

func parseHealthcheck(node *yaml.Node) *HealthcheckInfo {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	hc := &HealthcheckInfo{}
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]
		switch key {
		case "test":
			if val.Kind == yaml.ScalarNode {
				hc.Test = val.Value
			} else if val.Kind == yaml.SequenceNode {
				var parts []string
				for _, p := range val.Content {
					parts = append(parts, p.Value)
				}
				hc.Test = strings.Join(parts, " ")
			}
		case "interval":
			hc.Interval = val.Value
		case "timeout":
			hc.Timeout = val.Value
		case "start_period":
			hc.StartPeriod = val.Value
		case "retries":
			var r int
			if fmt.Sscanf(val.Value, "%d", &r); r > 0 {
				hc.Retries = r
			}
		}
	}
	return hc
}

func extractStringSequenceOrObjects(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	var results []string
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				results = append(results, item.Value)
			} else if item.Kind == yaml.MappingNode {
				// E.g. long syntax for ports or volumes
				var target, published string
				for i := 0; i < len(item.Content); i += 2 {
					k := item.Content[i].Value
					v := item.Content[i+1].Value
					if k == "target" || k == "container" {
						target = v
					} else if k == "published" || k == "host" {
						published = v
					} else if k == "source" {
						published = v
					}
				}
				if published != "" && target != "" {
					results = append(results, fmt.Sprintf("%s:%s", published, target))
				} else if target != "" {
					results = append(results, target)
				}
			}
		}
	}
	return results
}

func extractKeysOrList(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	var list []string
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			list = append(list, node.Content[i].Value)
		}
	} else if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				list = append(list, item.Value)
			}
		}
	}
	return list
}

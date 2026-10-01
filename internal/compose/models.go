package compose

// ServiceInfo holds parsed details of a Docker Compose service.
type ServiceInfo struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	ContainerName string            `json:"container_name,omitempty"`
	Command       string            `json:"command,omitempty"`
	Restart       string            `json:"restart,omitempty"`
	Ports         []string          `json:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	Networks      []string          `json:"networks,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	Healthcheck   *HealthcheckInfo  `json:"healthcheck,omitempty"`
}

// HealthcheckInfo represents container healthcheck configuration in Compose.
type HealthcheckInfo struct {
	Test        string `json:"test,omitempty"`
	Interval    string `json:"interval,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	Retries     int    `json:"retries,omitempty"`
	StartPeriod string `json:"start_period,omitempty"`
}

// ComposeStack represents a parsed compose file with high-level infrastructure details.
type ComposeStack struct {
	FilePath string                 `json:"file_path"`
	FileName string                 `json:"file_name"`
	Services map[string]ServiceInfo `json:"services"`
	Networks []string               `json:"networks"`
	Volumes  []string               `json:"volumes"`
}

// DependencyTier represents a tier/layer in the topological dependency graph.
type DependencyTier struct {
	Level    int           `json:"level"`
	Label    string        `json:"label"` // e.g. "Ingress / Frontend", "Lógica / API", "Datos / Almacenamiento"
	Services []ServiceInfo `json:"services"`
}

// TopologyGraph represents the directed dependency structure of a stack.
type TopologyGraph struct {
	Tiers          []DependencyTier `json:"tiers"`
	RootServices   []string         `json:"root_services"`
	MiddleServices []string         `json:"middle_services"`
	LeafServices   []string         `json:"leaf_services"`
	AllDependencies map[string][]string `json:"dependencies"` // svc -> list of services it depends on
	Dependents      map[string][]string `json:"dependents"`   // svc -> list of services that depend on it
}

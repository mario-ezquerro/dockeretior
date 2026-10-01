package compose

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
)

// BuildTopologyGraph analyzes dependencies and constructs layered tiers.
func BuildTopologyGraph(stack *ComposeStack) *TopologyGraph {
	if stack == nil || len(stack.Services) == 0 {
		return &TopologyGraph{}
	}

	graph := &TopologyGraph{
		AllDependencies: make(map[string][]string),
		Dependents:      make(map[string][]string),
	}

	inDegree := make(map[string]int)  // how many depend on this service
	outDegree := make(map[string]int) // how many this service depends on

	for name, svc := range stack.Services {
		graph.AllDependencies[name] = svc.DependsOn
		outDegree[name] = len(svc.DependsOn)
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, dep := range svc.DependsOn {
			graph.Dependents[dep] = append(graph.Dependents[dep], name)
			inDegree[dep]++
		}
	}

	// Classify into tiers
	var rootSvcs, middleSvcs, leafSvcs []string

	for name, svc := range stack.Services {
		// Is it a database/store?
		isDB := isDatabaseOrStore(name, svc.Image)

		if outDegree[name] == 0 && (inDegree[name] > 0 || isDB) {
			// Leaf / Data store (other services depend on it, or it's a known DB)
			leafSvcs = append(leafSvcs, name)
		} else if inDegree[name] == 0 {
			// Root / Ingress / Frontend
			rootSvcs = append(rootSvcs, name)
		} else {
			// Middle-tier API / Worker
			middleSvcs = append(middleSvcs, name)
		}
	}

	// Sort alphabetically for deterministic ordering
	sort.Strings(rootSvcs)
	sort.Strings(middleSvcs)
	sort.Strings(leafSvcs)

	// If everything was classified in one tier, distribute reasonably
	if len(rootSvcs) == 0 && len(middleSvcs) > 0 {
		rootSvcs = append(rootSvcs, middleSvcs[0])
		middleSvcs = middleSvcs[1:]
	}

	graph.RootServices = rootSvcs
	graph.MiddleServices = middleSvcs
	graph.LeafServices = leafSvcs

	var tiers []DependencyTier

	if len(rootSvcs) > 0 {
		var tierSvcs []ServiceInfo
		for _, n := range rootSvcs {
			tierSvcs = append(tierSvcs, stack.Services[n])
		}
		tiers = append(tiers, DependencyTier{
			Level:    1,
			Label:    "Ingress / Frontend / Entrypoint",
			Services: tierSvcs,
		})
	}

	if len(middleSvcs) > 0 {
		var tierSvcs []ServiceInfo
		for _, n := range middleSvcs {
			tierSvcs = append(tierSvcs, stack.Services[n])
		}
		tiers = append(tiers, DependencyTier{
			Level:    2,
			Label:    "Lógica de Negocio / APIs / Workers",
			Services: tierSvcs,
		})
	}

	if len(leafSvcs) > 0 {
		var tierSvcs []ServiceInfo
		for _, n := range leafSvcs {
			tierSvcs = append(tierSvcs, stack.Services[n])
		}
		tiers = append(tiers, DependencyTier{
			Level:    3,
			Label:    "Bases de Datos / Caché / Almacenamiento",
			Services: tierSvcs,
		})
	}

	graph.Tiers = tiers
	return graph
}

// RenderTopologyASCII generates an ASCII visual map of the compose services and connections.
func RenderTopologyASCII(stack *ComposeStack, graph *TopologyGraph, maxWidth int) string {
	if stack == nil || len(stack.Services) == 0 {
		return "  (No hay servicios definidos en este archivo Compose)\n"
	}

	var sb strings.Builder

	// 1. Render summary header
	sb.WriteString(fmt.Sprintf(" 📦 STACK: %s (%d servicios, %d redes, %d volúmenes)\n\n",
		stack.FileName, len(stack.Services), len(stack.Networks), len(stack.Volumes)))

	// 2. Render each tier with connection arrows
	for tierIdx, tier := range graph.Tiers {
		// Tier Label
		sb.WriteString(fmt.Sprintf(" ── Nivel %d: %s ──\n", tier.Level, tier.Label))

		// Render service boxes in this tier
		var boxes [][]string
		for _, svc := range tier.Services {
			box := formatServiceBox(svc, 32)
			boxes = append(boxes, box)
		}

		// Arrange boxes side-by-side if terminal width allows, otherwise stacked
		combinedBoxes := renderSideBySideBoxes(boxes, maxWidth-4)
		for _, line := range combinedBoxes {
			sb.WriteString("  " + line + "\n")
		}

		// If there is a next tier, draw connection arrows
		if tierIdx < len(graph.Tiers)-1 {
			sb.WriteString("            │\n")
			sb.WriteString("            ▼ (depends_on)\n")
		}
	}

	// 3. Render textual dependency matrix
	sb.WriteString("\n ── Mapa de Dependencias y Enlaces ────────────────────────\n")
	hasDeps := false
	for svcName, svc := range stack.Services {
		if len(svc.DependsOn) > 0 {
			hasDeps = true
			sb.WriteString(fmt.Sprintf("  • %-16s ──depends_on──▶ [%s]\n",
				svcName, strings.Join(svc.DependsOn, ", ")))
		}
	}
	if !hasDeps {
		sb.WriteString("  (Los servicios no tienen directivas 'depends_on' explícitas; operan de forma independiente)\n")
	}

	return sb.String()
}

func formatServiceBox(svc ServiceInfo, boxWidth int) []string {
	if boxWidth < 28 {
		boxWidth = 28
	}

	icon := "📦"
	if isDatabaseOrStore(svc.Name, svc.Image) {
		icon = "🗄️ "
	} else if strings.Contains(strings.ToLower(svc.Name), "api") || strings.Contains(strings.ToLower(svc.Name), "app") {
		icon = "🚀"
	} else if strings.Contains(strings.ToLower(svc.Name), "web") || strings.Contains(strings.ToLower(svc.Name), "proxy") || strings.Contains(strings.ToLower(svc.Name), "traefik") || strings.Contains(strings.ToLower(svc.Name), "nginx") {
		icon = "🌐"
	}

	title := fmt.Sprintf("%s %s", icon, svc.Name)
	if runewidth.StringWidth(title) > boxWidth-4 {
		title = title[:min(len(title), boxWidth-7)] + "..."
	}

	innerW := boxWidth - 4
	topBorder := "┌" + strings.Repeat("─", boxWidth-2) + "┐"
	botBorder := "└" + strings.Repeat("─", boxWidth-2) + "┘"

	line1 := fmt.Sprintf("│ %-*s │", innerW, title)

	imgStr := "img: " + svc.Image
	if len(imgStr) > innerW {
		imgStr = imgStr[:innerW-3] + "..."
	}
	line2 := fmt.Sprintf("│ %-*s │", innerW, imgStr)

	extraStr := ""
	if len(svc.Ports) > 0 {
		extraStr = "ports: " + strings.Join(svc.Ports, ",")
	} else if len(svc.Volumes) > 0 {
		extraStr = fmt.Sprintf("vols: %d montados", len(svc.Volumes))
	} else if len(svc.Networks) > 0 {
		extraStr = "net: " + strings.Join(svc.Networks, ",")
	}
	if len(extraStr) > innerW {
		extraStr = extraStr[:innerW-3] + "..."
	}
	line3 := fmt.Sprintf("│ %-*s │", innerW, extraStr)

	return []string{topBorder, line1, line2, line3, botBorder}
}

func renderSideBySideBoxes(boxes [][]string, maxW int) []string {
	if len(boxes) == 0 {
		return nil
	}

	boxW := 32
	boxesPerRow := maxW / (boxW + 2)
	if boxesPerRow < 1 {
		boxesPerRow = 1
	}

	var finalLines []string

	for i := 0; i < len(boxes); i += boxesPerRow {
		end := i + boxesPerRow
		if end > len(boxes) {
			end = len(boxes)
		}

		chunk := boxes[i:end]
		boxHeight := len(chunk[0])

		for row := 0; row < boxHeight; row++ {
			var rowParts []string
			for _, b := range chunk {
				if row < len(b) {
					rowParts = append(rowParts, b[row])
				}
			}
			finalLines = append(finalLines, strings.Join(rowParts, "  "))
		}
		if end < len(boxes) {
			finalLines = append(finalLines, "")
		}
	}

	return finalLines
}

func isDatabaseOrStore(name, image string) bool {
	combined := strings.ToLower(name + " " + image)
	dbKeywords := []string{
		"postgres", "mysql", "mariadb", "redis", "mongo",
		"cockroach", "elasticsearch", "influxdb", "clickhouse",
		"memcached", "rabbitmq", "kafka", "scylla", "cassand",
	}
	for _, kw := range dbKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

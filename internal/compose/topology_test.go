package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseComposeStackAndTopology(t *testing.T) {
	yamlContent := `
version: '3.8'

services:
  traefik:
    image: traefik:v2.10
    ports:
      - "80:80"
      - "443:443"
    networks:
      - web

  api:
    image: mycompany/api:latest
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      - DB_HOST=postgres
      - CACHE_HOST=redis
    networks:
      - web
      - internal

  postgres:
    image: postgres:16-alpine
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: admin
    networks:
      - internal
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U admin"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    networks:
      - internal

networks:
  web:
  internal:

volumes:
  db_data:
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test compose file: %v", err)
	}

	stack, err := ParseComposeStack(filePath)
	if err != nil {
		t.Fatalf("ParseComposeStack failed: %v", err)
	}

	if len(stack.Services) != 4 {
		t.Errorf("expected 4 services, got %d", len(stack.Services))
	}
	if len(stack.Networks) != 2 {
		t.Errorf("expected 2 networks, got %d", len(stack.Networks))
	}
	if len(stack.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(stack.Volumes))
	}

	apiSvc := stack.Services["api"]
	if len(apiSvc.DependsOn) != 2 {
		t.Errorf("expected api to have 2 depends_on, got %d", len(apiSvc.DependsOn))
	}

	pgSvc := stack.Services["postgres"]
	if pgSvc.Healthcheck == nil {
		t.Errorf("expected postgres to have a healthcheck")
	} else if !strings.Contains(pgSvc.Healthcheck.Test, "pg_isready") {
		t.Errorf("expected postgres healthcheck to contain pg_isready, got %s", pgSvc.Healthcheck.Test)
	}

	// Test Topology Graph
	graph := BuildTopologyGraph(stack)
	if len(graph.Tiers) == 0 {
		t.Fatalf("expected non-empty tiers in topology graph")
	}

	// Render ASCII topology
	rendered := RenderTopologyASCII(stack, graph, 100)
	if !strings.Contains(rendered, "traefik") {
		t.Errorf("expected rendered topology to mention traefik")
	}
	if !strings.Contains(rendered, "depends_on") {
		t.Errorf("expected rendered topology to mention depends_on")
	}
}

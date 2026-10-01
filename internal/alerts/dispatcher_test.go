package alerts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mario-ezquerro/dockeretior/internal/autodoctor"
)

func TestFormatAlertMessage(t *testing.T) {
	payload := AlertPayload{
		ServerName:     "production-01",
		Title:          "Contenedor 'api' terminado por OOM Killer (Exit 137)",
		Target:         "api",
		Severity:       autodoctor.SeverityCritical,
		RootCause:      "Presión de memoria. Límite alcanzado.",
		Recommendation: "Incrementar memory en compose.yml.",
		Timestamp:      time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC),
	}

	msg := FormatAlertMessage("production-01", payload)
	if !strings.Contains(msg, "🚨 Dockeretior Alert") {
		t.Errorf("expected alert header")
	}
	if !strings.Contains(msg, "production-01") {
		t.Errorf("expected host name in alert")
	}
	if !strings.Contains(msg, "CRITICAL") {
		t.Errorf("expected severity in alert")
	}
}

func TestWebhookDispatch(t *testing.T) {
	var receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := AlertConfig{
		Enabled:    true,
		ServerName: "test-node",
		WebhookURL: ts.URL,
	}

	payload := AlertPayload{
		ServerName:     "test-node",
		Title:          "Disk Full Warning",
		Target:         "Docker Storage",
		Severity:       autodoctor.SeverityWarning,
		RootCause:      "Over 90% full",
		Recommendation: "Prune unused volumes",
		Timestamp:      time.Now(),
	}

	err := DispatchAlert(context.Background(), cfg, payload)
	if err != nil {
		t.Fatalf("unexpected error dispatching alert: %v", err)
	}

	if !strings.Contains(receivedBody, "Disk Full Warning") {
		t.Errorf("expected webhook body to contain alert title, got: %s", receivedBody)
	}
}

func TestConfigLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test_alerts.json")

	cfg := AlertConfig{
		Enabled:          true,
		ServerName:       "custom-server-01",
		WebhookURL:       "https://hooks.slack.com/services/test",
		TelegramBotToken: "123456:ABC-DEF",
		TelegramChatID:   "-100123456789",
	}

	if err := SaveAlertConfig(cfg, cfgPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded := LoadAlertConfig(cfgPath)
	if !loaded.Enabled || loaded.ServerName != "custom-server-01" {
		t.Errorf("loaded config does not match saved config")
	}
	if loaded.TelegramChatID != "-100123456789" {
		t.Errorf("telegram chat ID mismatch")
	}
}

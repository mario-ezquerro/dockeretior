package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mario-ezquerro/dockeretior/internal/autodoctor"
)

// AlertConfig stores notification endpoints and credentials.
type AlertConfig struct {
	Enabled          bool   `json:"enabled"`
	ServerName       string `json:"server_name"`
	WebhookURL       string `json:"webhook_url,omitempty"`
	TelegramBotToken string `json:"telegram_bot_token,omitempty"`
	TelegramChatID   string `json:"telegram_chat_id,omitempty"`
}

// AlertPayload defines the structured notification.
type AlertPayload struct {
	ServerName     string               `json:"server_name"`
	Title          string               `json:"title"`
	Target         string               `json:"target"`
	Severity       autodoctor.Severity  `json:"severity"`
	RootCause      string               `json:"root_cause"`
	Recommendation string               `json:"recommendation"`
	Timestamp      time.Time            `json:"timestamp"`
}

// DefaultAlertConfigPath returns the config file path.
func DefaultAlertConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".dockeretior")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "alerts.json")
}

// LoadAlertConfig loads notification settings or creates a template.
func LoadAlertConfig(customPath string) AlertConfig {
	path := customPath
	if path == "" {
		path = DefaultAlertConfigPath()
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "docker-host"
	}

	cfg := AlertConfig{
		Enabled:    false,
		ServerName: hostname,
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}

	return cfg
}

// SaveAlertConfig writes the configuration to disk.
func SaveAlertConfig(cfg AlertConfig, customPath string) error {
	path := customPath
	if path == "" {
		path = DefaultAlertConfigPath()
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// DispatchAlert sends an alert to configured Webhooks and Telegram.
func DispatchAlert(ctx context.Context, cfg AlertConfig, payload AlertPayload) error {
	if !cfg.Enabled {
		return nil
	}

	msg := FormatAlertMessage(cfg.ServerName, payload)

	var errs []string

	// 1. Send to Webhook (Slack / Discord / generic HTTP POST)
	if cfg.WebhookURL != "" {
		if err := sendWebhook(ctx, cfg.WebhookURL, msg, payload); err != nil {
			errs = append(errs, fmt.Sprintf("webhook: %v", err))
		}
	}

	// 2. Send to Telegram
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
		if err := sendTelegram(ctx, cfg.TelegramBotToken, cfg.TelegramChatID, msg); err != nil {
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("dispatch errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// FormatAlertMessage formats the alert into clean markdown text.
func FormatAlertMessage(serverName string, payload AlertPayload) string {
	icon := "🚨"
	if payload.Severity == autodoctor.SeverityWarning {
		icon = "⚠️"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s Dockeretior Alert\n", icon))
	sb.WriteString(fmt.Sprintf("Host       : %s\n", serverName))
	sb.WriteString(fmt.Sprintf("Target     : %s\n", payload.Target))
	sb.WriteString(fmt.Sprintf("Severidad  : %s\n", payload.Severity))
	sb.WriteString(fmt.Sprintf("Problema   : %s\n", payload.Title))
	sb.WriteString(fmt.Sprintf("Causa Raíz : %s\n", payload.RootCause))
	sb.WriteString(fmt.Sprintf("Solución   : %s\n", payload.Recommendation))
	sb.WriteString(fmt.Sprintf("Fecha      : %s\n", payload.Timestamp.Format("2006-01-02 15:04:05")))

	return sb.String()
}

func sendWebhook(ctx context.Context, webhookURL string, text string, payload AlertPayload) error {
	bodyMap := map[string]interface{}{
		"text":     text,
		"content":  text, // For Discord compatibility
		"server":   payload.ServerName,
		"severity": string(payload.Severity),
		"target":   payload.Target,
	}

	jsonData, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook responded with HTTP %d", resp.StatusCode)
	}

	return nil
}

func sendTelegram(ctx context.Context, token, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API responded with HTTP %d", resp.StatusCode)
	}

	return nil
}

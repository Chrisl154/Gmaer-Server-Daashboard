package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	WebhookURL    string
	WebhookFormat string // discord|slack|generic (default: discord)
	Events        []string
}

type Service struct {
	mu     sync.RWMutex
	cfg    Config
	client *http.Client
	logger *zap.Logger
}

func New(cfg Config, logger *zap.Logger) *Service {
	return &Service{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// UpdateConfig replaces the running config (thread-safe; called by the settings API).
func (s *Service) UpdateConfig(cfg Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

// GetConfig returns a copy of the current config.
func (s *Service) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Send fires a webhook for event if the event is in the enabled list and a URL is configured.
func (s *Service) Send(event, serverName, message string) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	if cfg.WebhookURL == "" {
		return
	}
	// Check if event is enabled (empty list = all events).
	if len(cfg.Events) > 0 {
		found := false
		for _, e := range cfg.Events {
			if e == event {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}
	go func() {
		if err := s.post(cfg, event, serverName, message); err != nil {
			s.logger.Warn("Webhook notification failed",
				zap.String("event", event),
				zap.String("server", serverName),
				zap.Error(err),
			)
		}
	}()
}

// Test sends a test notification using the current config and returns any error.
func (s *Service) Test() error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg.WebhookURL == "" {
		return fmt.Errorf("no webhook URL configured")
	}
	return s.post(cfg, "test", "Games Dashboard", "This is a test notification from Games Dashboard.")
}

func (s *Service) post(cfg Config, event, serverName, message string) error {
	var payload any

	text := fmt.Sprintf("[%s] %s — %s", event, serverName, message)

	format := cfg.WebhookFormat
	if format == "" {
		format = "discord"
	}

	switch format {
	case "slack":
		payload = map[string]string{"text": text}
	case "generic":
		payload = map[string]string{
			"event":   event,
			"server":  serverName,
			"message": message,
		}
	default: // discord
		payload = map[string]any{
			"embeds": []map[string]any{{
				"title":       fmt.Sprintf("[%s] %s", event, serverName),
				"description": message,
				"color": func() int {
					switch event {
					case "server.crash", "disk.warning", "backup.failed":
						return 0xef4444
					case "server.restart":
						return 0xf97316
					default:
						return 0x22c55e
					}
				}(),
			}},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := s.client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook POST failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

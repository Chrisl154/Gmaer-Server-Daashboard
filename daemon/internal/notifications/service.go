package notifications

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EmailConfig mirrors config.EmailConfig — duplicated here to avoid an import cycle.
type EmailConfig struct {
	Enabled  bool
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
}

type Config struct {
	WebhookURL    string
	WebhookFormat string // discord|slack|generic (default: discord)
	Events        []string
	Email         *EmailConfig
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

// Send fires a webhook and/or email for event if the event is in the enabled list.
func (s *Service) Send(event, serverName, message string) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

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

	if cfg.WebhookURL != "" {
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

	if cfg.Email != nil && cfg.Email.Enabled && len(cfg.Email.To) > 0 {
		go func() {
			subject := fmt.Sprintf("[Games Dashboard] [%s] %s", event, serverName)
			body := fmt.Sprintf("Event: %s\nServer: %s\n\n%s", event, serverName, message)
			if err := s.sendEmail(cfg.Email, subject, body); err != nil {
				s.logger.Warn("Email notification failed",
					zap.String("event", event),
					zap.String("server", serverName),
					zap.Error(err),
				)
			}
		}()
	}
}

// Test sends a test notification (webhook + email) and returns the first error encountered.
func (s *Service) Test() error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	var errs []string
	if cfg.WebhookURL != "" {
		if err := s.post(cfg, "test", "Games Dashboard", "This is a test notification from Games Dashboard."); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}
	if cfg.Email != nil && cfg.Email.Enabled && len(cfg.Email.To) > 0 {
		if err := s.sendEmail(cfg.Email,
			"[Games Dashboard] Test notification",
			"This is a test email from Games Dashboard.",
		); err != nil {
			errs = append(errs, "email: "+err.Error())
		}
	}
	if cfg.WebhookURL == "" && (cfg.Email == nil || !cfg.Email.Enabled) {
		return fmt.Errorf("no webhook URL or email configured")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// sendEmail delivers a plain-text email via SMTP.
func (s *Service) sendEmail(cfg *EmailConfig, subject, body string) error {
	if cfg.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	port := cfg.SMTPPort
	if port == 0 {
		if cfg.UseTLS {
			port = 465
		} else {
			port = 587
		}
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)

	msg := []byte("To: " + strings.Join(cfg.To, ", ") + "\r\n" +
		"From: " + cfg.From + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body + "\r\n")

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	if cfg.UseTLS {
		// Implicit TLS (port 465)
		tlsCfg := &tls.Config{ServerName: cfg.SMTPHost}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("TLS dial failed: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return fmt.Errorf("SMTP client failed: %w", err)
		}
		defer c.Quit()
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
		if err := c.Mail(cfg.From); err != nil {
			return err
		}
		for _, to := range cfg.To {
			if err := c.Rcpt(to); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = w.Write(msg)
		return err
	}

	// STARTTLS (port 587)
	return smtp.SendMail(addr, auth, cfg.From, cfg.To, msg)
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

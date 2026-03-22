// Package firewall provides a thin wrapper around the host's UFW firewall so
// the dashboard can read and manage rules without requiring the operator to
// SSH in for routine game-server port management.
package firewall

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Rule is a single UFW numbered rule as returned by "ufw status numbered"
type Rule struct {
	Num     int    `json:"num"`
	To      string `json:"to"`       // destination — e.g. "22/tcp", "8443/tcp"
	Action  string `json:"action"`   // ALLOW, DENY, REJECT
	From    string `json:"from"`     // source — "Anywhere" or a CIDR
	Comment string `json:"comment"`  // text after the '#' marker, may be empty
	V6      bool   `json:"v6"`       // true when the rule is the IPv6 variant
}

// Status is the full firewall state returned to callers
type Status struct {
	Available bool   `json:"available"` // false when ufw is not installed
	Enabled   bool   `json:"enabled"`
	Rules     []Rule `json:"rules"`
}

// AddRuleRequest describes a new firewall rule to create
type AddRuleRequest struct {
	Port    int    `json:"port"    binding:"required"`
	Proto   string `json:"proto"`    // "tcp" | "udp" | "" (both)
	From    string `json:"from"`     // CIDR or "" for Anywhere
	Comment string `json:"comment"`
}

// Service executes ufw commands and parses their output.
// All methods are safe to call even when ufw is not installed — they return
// an error that the API layer surfaces to the client.
type Service struct {
	logger  *zap.Logger
	ufwPath string // resolved once at startup
}

// NewService creates a Service. If ufw is not in PATH, Available() returns
// false on every Status call, but the service itself is non-nil.
func NewService(logger *zap.Logger) *Service {
	path, _ := exec.LookPath("ufw")
	if path == "" {
		logger.Warn("ufw not found in PATH — firewall management will be unavailable")
	}
	return &Service{logger: logger, ufwPath: path}
}

// Available reports whether ufw is installed on this host
func (s *Service) Available() bool { return s.ufwPath != "" }

// GetStatus returns whether UFW is enabled and the current numbered rule list.
func (s *Service) GetStatus(ctx context.Context) (Status, error) {
	if !s.Available() {
		return Status{Available: false}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, s.ufwPath, "status", "numbered").Output() //nolint:gosec
	if err != nil {
		return Status{}, fmt.Errorf("ufw status: %w", err)
	}

	enabled, rules := parseStatus(string(out))
	return Status{Available: true, Enabled: enabled, Rules: rules}, nil
}

// AddRule opens a port in UFW. from may be "" (Anywhere) or a CIDR.
func (s *Service) AddRule(ctx context.Context, req AddRuleRequest) error {
	if !s.Available() {
		return fmt.Errorf("ufw is not installed on this host")
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("port %d is out of range (1–65535)", req.Port)
	}
	if req.Proto == "" {
		req.Proto = "tcp"
	}
	if req.Proto != "tcp" && req.Proto != "udp" {
		return fmt.Errorf("protocol must be tcp or udp, got %q", req.Proto)
	}
	if req.From != "" && !cidrRe.MatchString(req.From) {
		return fmt.Errorf("from %q is not a valid IP or CIDR", req.From)
	}
	if req.Comment != "" && !commentSafeRe.MatchString(req.Comment) {
		return fmt.Errorf("comment contains disallowed characters")
	}

	// Build: ufw allow [from <cidr>] to any port <port> proto <proto> comment '<comment>'
	args := []string{"allow"}
	if req.From != "" {
		args = append(args, "from", req.From, "to", "any")
	}
	args = append(args, "port", strconv.Itoa(req.Port), "proto", req.Proto)
	if req.Comment != "" {
		args = append(args, "comment", req.Comment)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ufwPath, args...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw allow failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	s.logger.Info("UFW rule added",
		zap.Int("port", req.Port), zap.String("proto", req.Proto),
		zap.String("from", req.From), zap.String("comment", req.Comment))
	return nil
}

// DeleteRule removes a rule by its numbered position from "ufw status numbered".
func (s *Service) DeleteRule(ctx context.Context, num int) error {
	if !s.Available() {
		return fmt.Errorf("ufw is not installed on this host")
	}
	if num < 1 {
		return fmt.Errorf("rule number must be >= 1")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// "ufw --force delete N" — --force skips the y/n prompt
	cmd := exec.CommandContext(ctx, s.ufwPath, "--force", "delete", strconv.Itoa(num)) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw delete failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	s.logger.Info("UFW rule deleted", zap.Int("num", num))
	return nil
}

// SetEnabled enables or disables UFW.
func (s *Service) SetEnabled(ctx context.Context, enable bool) error {
	if !s.Available() {
		return fmt.Errorf("ufw is not installed on this host")
	}

	verb := "enable"
	if !enable {
		verb = "disable"
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ufwPath, "--force", verb) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw %s failed: %s (%w)", verb, strings.TrimSpace(string(out)), err)
	}
	s.logger.Info("UFW state changed", zap.String("state", verb))
	return nil
}

// ---------------------------------------------------------------------------
// Output parser
// ---------------------------------------------------------------------------

// ruleLineRe matches a numbered rule line from "ufw status numbered":
//
//	[ 1] 22/tcp                     ALLOW IN    Anywhere             # SSH
//	[ 2] 8443/tcp                   ALLOW IN    Anywhere
//	[ 3] 22/tcp (v6)                ALLOW IN    Anywhere (v6)        # SSH
// cidrRe validates the From field — must be empty (Anywhere), a bare IPv4/IPv6,
// or a CIDR block. Rejects anything that could confuse UFW's argument parser.
var cidrRe = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$|^[0-9a-fA-F:]+(:\/\d{1,3})?$`)

// commentSafeRe allows only printable ASCII minus shell-special chars.
var commentSafeRe = regexp.MustCompile(`^[a-zA-Z0-9 _\-\.,:/@#\(\)]{0,128}$`)

var ruleLineRe = regexp.MustCompile(
	`^\[\s*(\d+)\]\s+(\S[\S ]*?)\s{2,}(ALLOW|DENY|REJECT)(?:\s+(?:IN|OUT|FWD))?\s{1,}(\S[\S ]*?)(?:\s{2,}#\s+(.+))?$`,
)

func parseStatus(output string) (enabled bool, rules []Rule) {
	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")

		// Detect enabled/disabled from the first "Status:" line
		if strings.HasPrefix(line, "Status:") {
			enabled = strings.Contains(strings.ToLower(line), "active")
			continue
		}

		m := ruleLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		num, _ := strconv.Atoi(m[1])
		to := strings.TrimSpace(m[2])
		action := strings.TrimSpace(m[3])
		from := strings.TrimSpace(m[4])
		comment := strings.TrimSpace(m[5])

		// The IPv6 variants have "(v6)" appended to the 'to' or 'from' fields.
		v6 := strings.Contains(to, "(v6)") || strings.Contains(from, "(v6)")
		to = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(to, "(v6)")), "")
		from = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(from, "(v6)")), "")

		rules = append(rules, Rule{
			Num:     num,
			To:      strings.TrimSpace(to),
			Action:  action,
			From:    strings.TrimSpace(from),
			Comment: comment,
			V6:      v6,
		})
	}
	return enabled, rules
}

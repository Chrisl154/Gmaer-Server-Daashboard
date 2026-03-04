package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PortValidation is the result of checking a single port
type PortValidation struct {
	Internal  int    `json:"internal"`
	External  int    `json:"external"`
	Protocol  string `json:"protocol"`
	Available bool   `json:"available"`
	Reachable bool   `json:"reachable"`
	Conflict  string `json:"conflict,omitempty"`
	Latency   int64  `json:"latency_ms,omitempty"`
}

// ReachabilityProbeConfig configures the remote reachability validator
type ReachabilityProbeConfig struct {
	ValidatorURL string        `yaml:"validator_url"`
	Timeout      time.Duration `yaml:"timeout"`
}

// Service provides port availability checking and reachability probing
type Service struct {
	cfg    ReachabilityProbeConfig
	logger *zap.Logger
	mu     sync.RWMutex
	// reserved tracks ports already allocated to known servers
	reserved map[string]string // "proto:port" -> serverID
}

// NewService creates a new networking service
func NewService(cfg ReachabilityProbeConfig, logger *zap.Logger) *Service {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Service{
		cfg:      cfg,
		logger:   logger,
		reserved: make(map[string]string),
	}
}

// ReservePort marks a port as allocated to a server
func (s *Service) ReservePort(serverID, protocol string, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%d", protocol, port)
	s.reserved[key] = serverID
}

// ReleaseServer removes all port reservations for a server
func (s *Service) ReleaseServer(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, id := range s.reserved {
		if id == serverID {
			delete(s.reserved, key)
		}
	}
}

// ValidatePorts checks availability and optional reachability of a list of ports
func (s *Service) ValidatePorts(ctx context.Context, ports []struct {
	Internal int
	External int
	Protocol string
}) ([]PortValidation, error) {
	results := make([]PortValidation, 0, len(ports))

	for _, p := range ports {
		result := PortValidation{
			Internal: p.Internal,
			External: p.External,
			Protocol: p.Protocol,
		}

		// 1. Check for internal reservation conflicts
		key := fmt.Sprintf("%s:%d", p.Protocol, p.External)
		s.mu.RLock()
		conflictServer, conflicted := s.reserved[key]
		s.mu.RUnlock()

		if conflicted {
			result.Available = false
			result.Conflict = fmt.Sprintf("allocated to server %s", conflictServer)
			results = append(results, result)
			continue
		}

		// 2. Check OS-level availability
		available, err := s.isPortAvailable(p.Protocol, p.External)
		if err != nil {
			s.logger.Warn("Port check error",
				zap.Int("port", p.External),
				zap.String("proto", p.Protocol),
				zap.Error(err))
		}
		result.Available = available

		if !available {
			result.Conflict = "port in use by another process"
		}

		// 3. Remote reachability probe (best-effort)
		if s.cfg.ValidatorURL != "" && available {
			reachable, latency := s.probeReachability(ctx, p.Protocol, p.External)
			result.Reachable = reachable
			result.Latency = latency
		}

		results = append(results, result)
	}

	return results, nil
}

// isPortAvailable returns true if the port is not currently bound on the host
func (s *Service) isPortAvailable(protocol string, port int) (bool, error) {
	addr := fmt.Sprintf(":%d", port)

	switch protocol {
	case "tcp", "tcp4", "tcp6":
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return false, nil
		}
		ln.Close()
		return true, nil

	case "udp", "udp4", "udp6":
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return false, nil
		}
		pc.Close()
		return true, nil

	default:
		return false, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// probeReachability checks if the port is reachable from the internet via
// the configured remote validator endpoint.
// The validator is expected to respond to:
//
//	GET {ValidatorURL}/probe?proto=<tcp|udp>&port=<port>
//
// with a JSON body:  {"reachable": true|false, "latency_ms": <n>}
//
// When no ValidatorURL is configured (empty string), the function returns
// (false, 0) so callers treat it as "not probed".
func (s *Service) probeReachability(ctx context.Context, protocol string, port int) (reachable bool, latencyMS int64) {
	if s.cfg.ValidatorURL == "" {
		return false, 0
	}

	reqURL := fmt.Sprintf("%s/probe?proto=%s&port=%d", s.cfg.ValidatorURL, protocol, port)

	start := time.Now()
	client := &http.Client{Timeout: s.cfg.Timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		s.logger.Warn("Failed to build reachability probe request",
			zap.String("url", reqURL), zap.Error(err))
		return false, 0
	}

	resp, err := client.Do(req)
	latencyMS = time.Since(start).Milliseconds()
	if err != nil {
		s.logger.Warn("Reachability probe request failed",
			zap.String("url", reqURL), zap.Error(err))
		return false, latencyMS
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("Reachability probe returned non-200",
			zap.String("url", reqURL), zap.Int("status", resp.StatusCode))
		return false, latencyMS
	}

	var body struct {
		Reachable bool  `json:"reachable"`
		LatencyMS int64 `json:"latency_ms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		s.logger.Warn("Failed to decode reachability probe response",
			zap.String("url", reqURL), zap.Error(err))
		return false, latencyMS
	}

	// Prefer the latency reported by the remote validator if available.
	if body.LatencyMS > 0 {
		latencyMS = body.LatencyMS
	}
	return body.Reachable, latencyMS
}

// FindFreePort finds the next available port starting from start
func FindFreePort(protocol string, start int) (int, error) {
	for port := start; port < 65535; port++ {
		addr := fmt.Sprintf(":%d", port)
		switch protocol {
		case "tcp":
			ln, err := net.Listen("tcp", addr)
			if err == nil {
				ln.Close()
				return port, nil
			}
		case "udp":
			pc, err := net.ListenPacket("udp", addr)
			if err == nil {
				pc.Close()
				return port, nil
			}
		}
	}
	return 0, fmt.Errorf("no free %s port found starting at %d", protocol, start)
}

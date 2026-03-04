package networking

import (
	"context"
	"fmt"
	"net"
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

// probeReachability checks if the port is reachable from the internet
// using the configured remote validator endpoint (best-effort).
func (s *Service) probeReachability(ctx context.Context, protocol string, port int) (reachable bool, latencyMS int64) {
	// In production this would call the remote validator endpoint
	// e.g. GET https://validator.example.com/probe?proto=udp&port=7777
	// For now we check localhost connectivity as a proxy
	start := time.Now()
	timeout := s.cfg.Timeout

	dialer := &net.Dialer{Timeout: timeout}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := dialer.DialContext(ctx, protocol, addr)
	latencyMS = time.Since(start).Milliseconds()
	if err != nil {
		return false, latencyMS
	}
	conn.Close()
	return true, latencyMS
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

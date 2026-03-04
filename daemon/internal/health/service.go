package health

import (
	"context"
	"sync"
	"time"
)

// Status represents system health
type Status struct {
	Healthy    bool              `json:"healthy"`
	Timestamp  time.Time         `json:"timestamp"`
	Components map[string]Check  `json:"components"`
}

// Check is a single component health check result
type Check struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// SystemStatus includes broader metrics
type SystemStatus struct {
	Status
	Version   string    `json:"version"`
	Uptime    int64     `json:"uptime_seconds"`
	StartTime time.Time `json:"start_time"`
}

// Service manages health checks
type Service struct {
	mu        sync.RWMutex
	checks    map[string]func() Check
	startTime time.Time
}

// NewService creates a new health service
func NewService() *Service {
	return &Service{
		checks:    make(map[string]func() Check),
		startTime: time.Now(),
	}
}

// Start runs periodic health checks
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runChecks()
		case <-ctx.Done():
			return
		}
	}
}

// RegisterCheck adds a named health check
func (s *Service) RegisterCheck(name string, fn func() Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = fn
}

// Check runs all health checks and returns overall status
func (s *Service) Check() *Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &Status{
		Healthy:    true,
		Timestamp:  time.Now(),
		Components: make(map[string]Check),
	}

	for name, fn := range s.checks {
		result := fn()
		status.Components[name] = result
		if !result.Healthy {
			status.Healthy = false
		}
	}

	return status
}

// SystemStatus returns full system status
func (s *Service) SystemStatus() *SystemStatus {
	return &SystemStatus{
		Status:    *s.Check(),
		Version:   "1.0.0",
		Uptime:    int64(time.Since(s.startTime).Seconds()),
		StartTime: s.startTime,
	}
}

func (s *Service) runChecks() {
	s.Check() // just run to warm cache
}

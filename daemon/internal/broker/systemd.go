package broker

// systemd.go — systemd user-service based process management and re-adoption.
//
// Every non-Docker game server gets its own systemd user unit:
//   ~/.config/systemd/user/gdash-<id>.service
//
// Benefits over the previous goroutine-based approach:
//   - Server processes survive daemon restarts (systemd manages them independently)
//   - On daemon restart, running servers are re-adopted: state set back to Running
//   - Crash recovery delegated to systemd (Restart=on-failure)
//   - Log history persists in journald; streamed to the dashboard console via journalctl
//
// For Docker-deployed servers, re-adoption works differently: the daemon checks
// whether the container is still running via "docker inspect" and re-attaches.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ── Availability check ────────────────────────────────────────────────────────

// systemdAvailable returns true if systemd --user is accessible on this host.
// The result is memoised after the first call.
var (
	systemdChecked bool
	systemdOK      bool
)

func systemdAvailable() bool {
	if systemdChecked {
		return systemdOK
	}
	// "systemctl --user status" exits 0 (running) or 3 (some units failed/inactive)
	// when the user session bus is reachable.  Any other error means not available.
	cmd := exec.Command("systemctl", "--user", "--no-pager", "status") //nolint:gosec
	err := cmd.Run()
	if err == nil {
		systemdOK = true
	} else if ee, ok := err.(*exec.ExitError); ok {
		code := ee.ExitCode()
		// 1 = degraded, 3 = not-found/inactive — all mean the bus is reachable.
		systemdOK = code == 1 || code == 3
	}
	systemdChecked = true
	return systemdOK
}

// ── Unit file helpers ─────────────────────────────────────────────────────────

func systemdUnitName(serverID string) string { return "gdash-" + serverID + ".service" }

func systemdUnitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	return dir, os.MkdirAll(dir, 0o755)
}

func systemdUnitPath(serverID string) (string, error) {
	dir, err := systemdUnitDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, systemdUnitName(serverID)), nil
}

// scriptDir returns the directory where per-server wrapper scripts live.
// Using DataDir keeps them together with other server data and avoids
// embedding complex shell commands directly in the unit file.
func (b *Broker) scriptDir(serverID string) string {
	return filepath.Join(b.cfg.Storage.DataDir, "server-scripts", serverID)
}

// writeSystemdUnit generates:
//  1. A start.sh wrapper script (and optionally stop.sh) in the script dir.
//  2. A systemd user unit file that invokes the wrapper.
//
// Returns (true, nil) when the unit was written, (false, nil) when systemd is
// unavailable (caller should fall back to direct process management).
func (b *Broker) writeSystemdUnit(s *Server, startCmd, stopCmd, installDir string) (bool, error) {
	if !systemdAvailable() {
		return false, nil
	}

	dir := b.scriptDir(s.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create script dir: %w", err)
	}

	// start.sh — cd to install dir first so relative paths in the command work.
	startScript := filepath.Join(dir, "start.sh")
	startContent := "#!/bin/bash\n"
	if installDir != "" && installDir != "." {
		startContent += "cd " + shellescape(installDir) + " || exit 1\n"
	}
	startContent += startCmd + "\n"
	if err := os.WriteFile(startScript, []byte(startContent), 0o755); err != nil {
		return false, fmt.Errorf("write start script: %w", err)
	}

	// stop.sh (optional)
	execStop := ""
	if stopCmd != "" {
		stopScript := filepath.Join(dir, "stop.sh")
		if err := os.WriteFile(stopScript, []byte("#!/bin/bash\n"+stopCmd+"\n"), 0o755); err != nil {
			b.logger.Warn("Could not write stop script", zap.String("server", s.ID), zap.Error(err))
		} else {
			execStop = "ExecStop=" + stopScript + "\n"
		}
	}

	maxRestarts := s.MaxRestarts
	if maxRestarts <= 0 {
		maxRestarts = 5
	}
	restartSec := s.RestartDelaySecs
	if restartSec <= 0 {
		restartSec = 10
	}

	unit := fmt.Sprintf(`[Unit]
Description=GDash Game Server: %s
After=network.target

[Service]
Type=simple
ExecStart=%s
%sRestart=on-failure
RestartSec=%d
StartLimitIntervalSec=600
StartLimitBurst=%d
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gdash-%s

[Install]
WantedBy=default.target
`, s.Name, startScript, execStop, restartSec, maxRestarts, s.ID)

	unitPath, err := systemdUnitPath(s.ID)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return false, fmt.Errorf("write unit file: %w", err)
	}
	return true, nil
}

// removeSystemdUnit stops and removes the systemd unit and wrapper scripts.
func (b *Broker) removeSystemdUnit(serverID string) {
	if !systemdAvailable() {
		return
	}
	name := systemdUnitName(serverID)
	_ = b.systemctlUser("stop", name)
	_ = b.systemctlUser("disable", name)
	if path, err := systemdUnitPath(serverID); err == nil {
		_ = os.Remove(path)
	}
	_ = b.systemctlUser("daemon-reload")
	_ = os.RemoveAll(b.scriptDir(serverID))
}

// ── systemctl helpers ─────────────────────────────────────────────────────────

func (b *Broker) systemctlUser(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.logger.Debug("systemctl --user",
			zap.Strings("args", args),
			zap.String("output", strings.TrimSpace(string(out))),
			zap.Error(err))
		return fmt.Errorf("systemctl --user %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (b *Broker) systemdIsActive(serverID string) bool {
	cmd := exec.Command("systemctl", "--user", "is-active", systemdUnitName(serverID)) //nolint:gosec
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out)) == "active"
}

func (b *Broker) systemdMainPID(serverID string) int {
	cmd := exec.Command("systemctl", "--user", "show", //nolint:gosec
		"--property=MainPID", "--value", systemdUnitName(serverID))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

// ── Systemd-based server start ────────────────────────────────────────────────

// startViaSystemd writes a unit file and starts the server through systemd.
// Returns true if systemd was used (caller should return), false to fall back
// to the goroutine-based direct process management.
func (b *Broker) startViaSystemd(ctx context.Context, serverID string, s *Server, startCmd, stopCmd, installDir string) bool {
	written, err := b.writeSystemdUnit(s, startCmd, stopCmd, installDir)
	if !written {
		return false // systemd not available
	}
	if err != nil {
		b.logger.Warn("Could not write systemd unit — falling back to direct process management",
			zap.String("server", serverID), zap.Error(err))
		return false
	}

	if err := b.systemctlUser("daemon-reload"); err != nil {
		b.logger.Warn("systemctl daemon-reload failed — falling back",
			zap.String("server", serverID), zap.Error(err))
		return false
	}

	if err := b.systemctlUser("start", systemdUnitName(serverID)); err != nil {
		b.logger.Warn("systemctl start failed — falling back to direct process management",
			zap.String("server", serverID), zap.Error(err))
		return false
	}

	// Wait up to 5 s for the unit to become active.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b.systemdIsActive(serverID) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	pid := b.systemdMainPID(serverID)

	b.mu.Lock()
	if sv, ok := b.servers[serverID]; ok {
		sv.State = StateRunning
		now := time.Now()
		sv.LastStarted = &now
		sv.LastError = ""
		sv.PID = pid
		b.saveServersLocked()
	}
	b.mu.Unlock()

	monCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.serverCancels[serverID] = cancel
	b.mu.Unlock()

	go b.streamJournalLogs(monCtx, serverID)
	go b.monitorSystemdState(monCtx, serverID)

	b.sendConsoleMessage(serverID, fmt.Sprintf(
		`{"type":"system","msg":"[GDash] Server started via systemd (unit: gdash-%s, PID: %d)","ts":%d}`,
		serverID, pid, time.Now().Unix()))
	return true
}

// stopViaSystemd stops a systemd-managed server. Returns true if the server
// was managed by systemd and the stop was issued, false if caller should use
// the direct signal approach.
func (b *Broker) stopViaSystemd(serverID string) bool {
	if !systemdAvailable() || !b.systemdIsActive(serverID) {
		return false
	}
	b.logger.Info("Stopping server via systemd", zap.String("server", serverID))
	if err := b.systemctlUser("stop", systemdUnitName(serverID)); err != nil {
		b.logger.Warn("systemctl stop failed — falling back to signal",
			zap.String("server", serverID), zap.Error(err))
		return false
	}
	return true
}

// ── Startup re-adoption ───────────────────────────────────────────────────────

// reAdoptRunningServers is called during broker.Start() to re-attach to game
// servers that survived the daemon restart.  It checks both Docker containers
// and systemd-managed native processes.
func (b *Broker) reAdoptRunningServers(ctx context.Context) {
	b.mu.RLock()
	type snap struct {
		deployMethod string
		containerID  string
	}
	servers := make(map[string]snap, len(b.servers))
	for id, s := range b.servers {
		servers[id] = snap{deployMethod: s.DeployMethod, containerID: s.ContainerID}
	}
	b.mu.RUnlock()

	for id, sn := range servers {
		switch {
		case sn.deployMethod == "docker" && sn.containerID != "":
			b.tryAdoptDockerContainer(ctx, id, sn.containerID)
		default:
			b.tryAdoptSystemdService(ctx, id)
		}
	}
}

// tryAdoptDockerContainer re-adopts a running Docker container.
func (b *Broker) tryAdoptDockerContainer(ctx context.Context, serverID, containerID string) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return
	}
	out, err := exec.Command(dockerPath, "inspect", //nolint:gosec
		"--format={{.State.Running}}", containerID).Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return
	}

	b.logger.Info("Re-adopting Docker container after daemon restart",
		zap.String("server", serverID), zap.String("container", containerID))

	now := time.Now()
	b.mu.Lock()
	if s, ok := b.servers[serverID]; ok {
		s.State = StateRunning
		s.LastStarted = &now
	}
	b.mu.Unlock()

	adoptCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.serverCancels[serverID] = cancel
	b.mu.Unlock()

	// Re-attach log stream (last 50 lines + follow).
	go func() {
		logsCmd := exec.CommandContext(adoptCtx, dockerPath, "logs", "-f", "--tail", "50", containerID) //nolint:gosec
		stdout, _ := logsCmd.StdoutPipe()
		stderr, _ := logsCmd.StderrPipe()
		if err := logsCmd.Start(); err != nil {
			return
		}
		pipe := func(r interface{ Read([]byte) (int, error) }, typ string) {
			buf := make([]byte, 4096)
			for {
				n, rdErr := r.Read(buf)
				if n > 0 {
					b.sendConsoleMessage(serverID, fmt.Sprintf(`{"type":%q,"msg":%s,"ts":%d}`,
						typ, jsonStr(string(buf[:n])), time.Now().Unix()))
				}
				if rdErr != nil {
					return
				}
			}
		}
		go pipe(stdout, "stdout")
		go pipe(stderr, "stderr")
		_ = logsCmd.Wait()
	}()

	// Monitor container exit.
	go func() {
		defer cancel()
		waitCmd := exec.CommandContext(adoptCtx, dockerPath, "wait", containerID) //nolint:gosec
		_ = waitCmd.Run()
		if adoptCtx.Err() != nil {
			return // intentional stop
		}
		b.mu.Lock()
		if s, ok := b.servers[serverID]; ok && s.State == StateRunning {
			s.State = StateStopped
			t := time.Now()
			s.LastStopped = &t
		}
		b.mu.Unlock()
		b.sendConsoleMessage(serverID, fmt.Sprintf(
			`{"type":"system","msg":"Container exited","ts":%d}`, time.Now().Unix()))
		b.saveState()
	}()

	b.sendConsoleMessage(serverID, fmt.Sprintf(
		`{"type":"system","msg":"[GDash] Re-adopted container %s after daemon restart","ts":%d}`,
		containerID[:min(12, len(containerID))], time.Now().Unix()))
}

// tryAdoptSystemdService re-adopts a native server managed by systemd.
func (b *Broker) tryAdoptSystemdService(ctx context.Context, serverID string) {
	if !systemdAvailable() || !b.systemdIsActive(serverID) {
		return
	}

	pid := b.systemdMainPID(serverID)
	b.logger.Info("Re-adopting systemd service after daemon restart",
		zap.String("server", serverID), zap.Int("pid", pid))

	now := time.Now()
	b.mu.Lock()
	if s, ok := b.servers[serverID]; ok {
		s.State = StateRunning
		s.LastStarted = &now
		s.PID = pid
	}
	b.mu.Unlock()

	adoptCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.serverCancels[serverID] = cancel
	b.mu.Unlock()

	go b.streamJournalLogs(adoptCtx, serverID)
	go b.monitorSystemdState(adoptCtx, serverID)

	b.sendConsoleMessage(serverID, fmt.Sprintf(
		`{"type":"system","msg":"[GDash] Re-adopted systemd service gdash-%s after daemon restart (PID %d)","ts":%d}`,
		serverID, pid, time.Now().Unix()))
}

// ── State monitoring ──────────────────────────────────────────────────────────

// monitorSystemdState polls systemctl is-active every 5 s until the service
// becomes inactive (crashed or stopped), then marks the server accordingly.
func (b *Broker) monitorSystemdState(ctx context.Context, serverID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if b.systemdIsActive(serverID) {
				// Still running — refresh PID in case systemd restarted it after a crash.
				if pid := b.systemdMainPID(serverID); pid > 0 {
					b.mu.Lock()
					if s, ok := b.servers[serverID]; ok {
						s.PID = pid
					}
					b.mu.Unlock()
				}
				continue
			}
			// Service is no longer active.
			b.mu.Lock()
			if s, ok := b.servers[serverID]; ok && s.State == StateRunning {
				s.State = StateStopped
				t := time.Now()
				s.LastStopped = &t
				s.PID = 0
				b.saveServersLocked()
			}
			b.mu.Unlock()
			b.sendConsoleMessage(serverID, fmt.Sprintf(
				`{"type":"system","msg":"Server %s stopped (systemd reports inactive)","ts":%d}`,
				serverID, time.Now().Unix()))
			return
		}
	}
}

// ── Journal log streaming ─────────────────────────────────────────────────────

// streamJournalLogs follows journald output for the server's systemd unit and
// forwards each line to the console broadcast channel.
func (b *Broker) streamJournalLogs(ctx context.Context, serverID string) {
	cmd := exec.CommandContext(ctx, "journalctl", //nolint:gosec
		"--user-unit", systemdUnitName(serverID),
		"-f", "--output=cat", "--since=now",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		b.logger.Warn("journalctl pipe failed", zap.String("server", serverID), zap.Error(err))
		return
	}
	if err := cmd.Start(); err != nil {
		b.logger.Warn("journalctl start failed", zap.String("server", serverID), zap.Error(err))
		return
	}
	defer func() { _ = cmd.Wait() }()

	buf := make([]byte, 4096)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			line := strings.TrimRight(string(buf[:n]), "\n")
			b.sendConsoleMessage(serverID, fmt.Sprintf(`{"type":"stdout","msg":%s,"ts":%d}`,
				jsonStr(line), time.Now().Unix()))
		}
		if err != nil {
			return
		}
	}
}

// ── saveState helper (non-locked) ────────────────────────────────────────────

// saveState acquires the write lock and persists server state to disk.
func (b *Broker) saveState() {
	b.mu.Lock()
	b.saveServersLocked()
	b.mu.Unlock()
}

// ── shellescape ───────────────────────────────────────────────────────────────

// shellescape wraps s in single quotes, escaping any embedded single quotes.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

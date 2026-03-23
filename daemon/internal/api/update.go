package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	updateScriptPath   = "/opt/gdash/bin/gdash-update.sh"
	defaultRepoDirPath = "/opt/gdash/repo"
	updateLogPath      = "/opt/gdash/logs/gdash-update.log"
	updateStatePath    = "/opt/gdash/data/update-state.json"
	versionFilePath    = "/opt/gdash/VERSION"
)

// resolveRepoDirPath returns the git repo path for update checks.
// It prefers /opt/gdash/repo (production install), falling back to the
// current working directory's repo root (development mode).
func resolveRepoDirPath() string {
	if fi, err := os.Stat(defaultRepoDirPath); err == nil && fi.IsDir() {
		return defaultRepoDirPath
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output() //nolint:gosec
	if err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			return dir
		}
	}
	return defaultRepoDirPath
}

// readVersionFile reads the VERSION file and returns its contents, or "1.0.0".
func readVersionFile() string {
	data, err := os.ReadFile(versionFilePath) //nolint:gosec
	if err != nil {
		// Fall back to VERSION in the repo dir.
		data, err = os.ReadFile(filepath.Join(resolveRepoDirPath(), "VERSION")) //nolint:gosec
		if err != nil {
			return "1.0.0"
		}
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "1.0.0"
	}
	return v
}

// UpdateState represents the JSON state file written by the update script.
type UpdateState struct {
	Status    string `json:"status"`    // running, complete, failed
	Phase     string `json:"phase"`     // finding_go, building_daemon, etc.
	Progress  int    `json:"progress"`  // 0-100
	Branch    string `json:"branch"`    // main, dev
	Error     string `json:"error"`     // non-empty on failure
	UpdatedAt string `json:"updated_at"`
}

// readUpdateState reads the state file written by gdash-update.sh.
// Returns nil if the file doesn't exist or can't be parsed.
func readUpdateState() *UpdateState {
	data, err := os.ReadFile(updateStatePath) //nolint:gosec
	if err != nil {
		return nil
	}
	var s UpdateState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

// getUpdateStatus returns the current version, branch, commit, and the update
// state file contents.  This does NOT call git fetch — it only reads local
// state so it responds instantly.  Use POST /update/check to fetch from remote.
//
// GET /api/v1/admin/update/status
func (s *Server) getUpdateStatus(c *gin.Context) {
	repoDir := resolveRepoDirPath()

	hashOut, _ := exec.Command("git", "-C", repoDir, "rev-parse", "--short", "HEAD").Output() //nolint:gosec
	currentCommit := strings.TrimSpace(string(hashOut))
	if currentCommit == "" {
		currentCommit = "unknown"
	}

	branchOut, _ := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output() //nolint:gosec
	currentBranch := strings.TrimSpace(string(branchOut))
	if currentBranch == "" {
		currentBranch = "main"
	}

	version := readVersionFile()
	state := readUpdateState()

	resp := gin.H{
		"version":        version,
		"current_branch": currentBranch,
		"current_commit": currentCommit,
	}
	if state != nil {
		resp["update_state"] = state
	}
	c.JSON(http.StatusOK, resp)
}

// checkForUpdates fetches from the remote and compares HEAD against the target
// branch to determine if updates are available.  This is the only endpoint
// that hits the network — call it when the user clicks "Check for updates".
//
// POST /api/v1/admin/update/check
func (s *Server) checkForUpdates(c *gin.Context) {
	var req struct {
		Branch string `json:"branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Branch == "" {
		req.Branch = "main"
	}
	if req.Branch != "main" && req.Branch != "dev" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch must be 'main' or 'dev'"})
		return
	}

	repoDir := resolveRepoDirPath()

	// Fetch with a 30-second timeout so we don't hang forever.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	fetchCmd := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "origin", //nolint:gosec
		"+refs/heads/"+req.Branch+":refs/remotes/origin/"+req.Branch, "--quiet")
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		s.cfg.Logger.Warn("git fetch failed",
			zap.String("branch", req.Branch), zap.String("output", string(out)), zap.Error(err))
		c.JSON(http.StatusOK, gin.H{
			"update_available": false,
			"commits_behind":   0,
			"error":            "Could not reach GitHub — check network connectivity.",
		})
		return
	}

	behindOut, _ := exec.Command("git", "-C", repoDir, "rev-list", //nolint:gosec
		"HEAD..origin/"+req.Branch, "--count").Output()
	commitsBehind, _ := strconv.Atoi(strings.TrimSpace(string(behindOut)))

	latestOut, _ := exec.Command("git", "-C", repoDir, "log", //nolint:gosec
		"origin/"+req.Branch, "-1", "--pretty=format:%s").Output()
	latestMsg := strings.TrimSpace(string(latestOut))

	// Read the VERSION from the remote branch so we can show a before/after diff.
	remoteVersionOut, _ := exec.Command("git", "-C", repoDir, "show", //nolint:gosec
		"origin/"+req.Branch+":VERSION").Output()
	remoteVersion := strings.TrimSpace(string(remoteVersionOut))
	if remoteVersion == "" {
		remoteVersion = readVersionFile() // fallback: same as local
	}

	c.JSON(http.StatusOK, gin.H{
		"target_branch":    req.Branch,
		"commits_behind":   commitsBehind,
		"update_available": commitsBehind > 0,
		"latest_message":   latestMsg,
		"available_version": remoteVersion,
	})
}

// getUpdateLog returns the last N lines of the update log.
// GET /api/v1/admin/update/log
func (s *Server) getUpdateLog(c *gin.Context) {
	n := 80
	if nStr := c.Query("lines"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 && parsed <= 500 {
			n = parsed
		}
	}

	f, err := os.Open(updateLogPath) //nolint:gosec
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"lines": []string{}, "note": "no update log yet — run an update first"})
		return
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	c.JSON(http.StatusOK, gin.H{"lines": all})
}

// applyUpdate kicks off the update script in a detached process and returns 202.
// POST /api/v1/admin/update/apply
func (s *Server) applyUpdate(c *gin.Context) {
	var req struct {
		Branch string `json:"branch"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Branch == "" {
		req.Branch = "main"
	}
	if req.Branch != "main" && req.Branch != "dev" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch must be 'main' or 'dev'"})
		return
	}

	repoDir := resolveRepoDirPath()

	// Fetch target branch first with explicit refspec.
	fetchCmd := exec.Command("git", "-C", repoDir, "fetch", "origin", //nolint:gosec
		"+refs/heads/"+req.Branch+":refs/remotes/origin/"+req.Branch, "--quiet")
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		s.cfg.Logger.Warn("git fetch failed (continuing anyway)",
			zap.String("branch", req.Branch), zap.String("output", string(out)), zap.Error(err))
	}

	// Refresh the on-disk update script from the repo.
	repoScript := filepath.Join(repoDir, "scripts", "gdash-update.sh")
	if data, readErr := os.ReadFile(repoScript); readErr == nil { //nolint:gosec
		if writeErr := os.WriteFile(updateScriptPath, data, 0o755); writeErr != nil {
			s.cfg.Logger.Warn("Could not refresh update script", zap.Error(writeErr))
		} else {
			s.cfg.Logger.Info("Update script refreshed from repo")
		}
	}

	if _, err := os.Stat(updateScriptPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "update script not found at " + updateScriptPath + " — re-run the installer to restore it",
		})
		return
	}

	// GPG signature verification (if enabled).
	requireSigned := s.cfg.DaemonCfg == nil || s.cfg.DaemonCfg.Updates.RequireSignedCommits
	if requireSigned {
		tipOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "origin/"+req.Branch).Output() //nolint:gosec
		if err != nil {
			s.recordEvent(c, "update.apply", "daemon", false, map[string]any{"branch": req.Branch, "error": "could not resolve tip commit"})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve tip commit for branch " + req.Branch})
			return
		}
		tipCommit := strings.TrimSpace(string(tipOut))

		verifyCmd := exec.Command("git", "-C", repoDir, "verify-commit", tipCommit) //nolint:gosec
		if out, err := verifyCmd.CombinedOutput(); err != nil {
			s.cfg.Logger.Warn("Update blocked — tip commit is not GPG-signed",
				zap.String("branch", req.Branch),
				zap.String("commit", tipCommit),
				zap.String("gpg_output", string(out)),
			)
			s.recordEvent(c, "update.apply", "daemon", false, map[string]any{
				"branch": req.Branch,
				"commit": tipCommit,
				"reason": "unsigned commit",
			})
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Update blocked: the tip commit on " + req.Branch + " is not GPG-signed. " +
					"Set updates.require_signed_commits: false in daemon.yaml to allow unsigned updates.",
			})
			return
		}
	}

	// Backup current daemon binary.
	if self, exErr := os.Executable(); exErr == nil {
		_ = func() error {
			src, err := os.Open(self) //nolint:gosec
			if err != nil {
				return err
			}
			defer src.Close()
			dst, err := os.OpenFile(self+".bak", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			defer dst.Close()
			_, err = io.Copy(dst, src)
			return err
		}()
	}

	s.cfg.Logger.Info("Starting self-update", zap.String("branch", req.Branch))
	s.recordEvent(c, "update.apply", "daemon", true, map[string]any{"branch": req.Branch})

	// Setsid detaches the child so it survives daemon restart.
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	cmd := exec.CommandContext(updateCtx, "bash", updateScriptPath, req.Branch) //nolint:gosec
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		updateCancel()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to launch update script: " + err.Error()})
		return
	}
	go func() {
		defer updateCancel()
		_ = cmd.Wait()
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status": "update_started",
		"branch": req.Branch,
		"msg":    "Update is running in the background. The dashboard will restart automatically when complete.",
	})
}

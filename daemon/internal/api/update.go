package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
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
)

// resolveRepoDirPath returns the git repo path for update checks.
// It prefers /opt/gdash/repo (production install), falling back to the
// current working directory's repo root (development mode).
func resolveRepoDirPath() string {
	// Production path exists — use it.
	if fi, err := os.Stat(defaultRepoDirPath); err == nil && fi.IsDir() {
		return defaultRepoDirPath
	}
	// Fall back to the git repo root of the daemon's working directory.
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output() //nolint:gosec
	if err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			return dir
		}
	}
	return defaultRepoDirPath
}

// getUpdateStatus returns current branch, commit, and whether updates are available
// on the requested target branch (query param ?branch=dev|main; defaults to current branch).
func (s *Server) getUpdateStatus(c *gin.Context) {
	repoDir := resolveRepoDirPath()

	hashOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "--short", "HEAD").Output() //nolint:gosec
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":            "could not read git state — is the repo at " + repoDir + "?",
			"update_available": false,
		})
		return
	}
	currentCommit := strings.TrimSpace(string(hashOut))

	branchOut, _ := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output() //nolint:gosec
	currentBranch := strings.TrimSpace(string(branchOut))
	if currentBranch == "" {
		currentBranch = "main"
	}

	// The target branch is what the user selected in the UI (may differ from current).
	targetBranch := c.DefaultQuery("branch", currentBranch)
	if targetBranch != "main" && targetBranch != "dev" {
		targetBranch = currentBranch
	}

	// Fetch so behind-count is accurate (best-effort; ignore errors — no internet, etc.)
	_ = exec.Command("git", "-C", repoDir, "fetch", "origin", "--quiet").Run() //nolint:gosec

	// Compare HEAD against origin/<targetBranch> so switching to dev shows dev's commits.
	behindOut, _ := exec.Command("git", "-C", repoDir, "rev-list", //nolint:gosec
		"HEAD..origin/"+targetBranch, "--count").Output()
	commitsBehind, _ := strconv.Atoi(strings.TrimSpace(string(behindOut)))

	latestOut, _ := exec.Command("git", "-C", repoDir, "log", //nolint:gosec
		"origin/"+targetBranch, "-1", "--pretty=format:%s").Output()
	latestMsg := strings.TrimSpace(string(latestOut))

	c.JSON(http.StatusOK, gin.H{
		"current_branch":   currentBranch,
		"target_branch":    targetBranch,
		"current_commit":   currentCommit,
		"commits_behind":   commitsBehind,
		"update_available": commitsBehind > 0,
		"latest_message":   latestMsg,
	})
}

// getUpdateLog returns the last N lines of the update log so the UI can show
// what the background update script actually did (or why it failed).
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

	// Collect all lines then return the last n.
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

// applyUpdate kicks off the update script in a detached process and returns 202 immediately.
// The script rebuilds the daemon/UI binaries and restarts the systemd service.
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

	if _, err := os.Stat(updateScriptPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "update script not found at " + updateScriptPath + " — re-run the installer to restore it",
		})
		return
	}

	// Verify the tip commit is GPG-signed before applying unless the operator
	// has explicitly opted out via updates.require_signed_commits: false.
	requireSigned := s.cfg.DaemonCfg == nil || s.cfg.DaemonCfg.Updates.RequireSignedCommits
	if requireSigned {
		// Fetch so we have the latest remote refs.
		_ = exec.Command("git", "-C", resolveRepoDirPath(), "fetch", "origin", "--quiet").Run() //nolint:gosec

		// Resolve the tip commit of origin/<branch>.
		tipOut, err := exec.Command("git", "-C", resolveRepoDirPath(), "rev-parse", "origin/"+req.Branch).Output() //nolint:gosec
		if err != nil {
			s.recordEvent(c, "update.apply", "daemon", false, map[string]any{"branch": req.Branch, "error": "could not resolve tip commit"})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve tip commit for branch " + req.Branch})
			return
		}
		tipCommit := strings.TrimSpace(string(tipOut))

		// Verify the commit carries a valid GPG signature.
		verifyCmd := exec.Command("git", "-C", resolveRepoDirPath(), "verify-commit", tipCommit) //nolint:gosec
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

	// P52: copy the current daemon binary before overwriting so a failed update
	// can be manually recovered by restoring from the .bak file.
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

	// Setsid detaches the child into its own session so it survives when systemd
	// kills the current daemon process during the restart step.
	// 10-minute timeout guards against a hung update script.
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
		"msg":    "Update is running in the background. The dashboard will restart automatically in ~60 seconds.",
	})
}

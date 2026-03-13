package api

import (
	"bufio"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	updateScriptPath = "/opt/gdash/bin/gdash-update.sh"
	repoDirPath      = "/opt/gdash/repo"
	updateLogPath    = "/var/log/gdash-update.log"
)

// getUpdateStatus returns current branch, commit, and whether updates are available
// on the requested target branch (query param ?branch=dev|main; defaults to current branch).
func (s *Server) getUpdateStatus(c *gin.Context) {
	hashOut, err := exec.Command("git", "-C", repoDirPath, "rev-parse", "--short", "HEAD").Output() //nolint:gosec
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":            "could not read git state — is the repo at " + repoDirPath + "?",
			"update_available": false,
		})
		return
	}
	currentCommit := strings.TrimSpace(string(hashOut))

	branchOut, _ := exec.Command("git", "-C", repoDirPath, "rev-parse", "--abbrev-ref", "HEAD").Output() //nolint:gosec
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
	_ = exec.Command("git", "-C", repoDirPath, "fetch", "origin", "--quiet").Run() //nolint:gosec

	// Compare HEAD against origin/<targetBranch> so switching to dev shows dev's commits.
	behindOut, _ := exec.Command("git", "-C", repoDirPath, "rev-list", //nolint:gosec
		"HEAD..origin/"+targetBranch, "--count").Output()
	commitsBehind, _ := strconv.Atoi(strings.TrimSpace(string(behindOut)))

	latestOut, _ := exec.Command("git", "-C", repoDirPath, "log", //nolint:gosec
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

	s.cfg.Logger.Info("Starting self-update", zap.String("branch", req.Branch))

	// Setsid detaches the child into its own session so it survives when systemd
	// kills the current daemon process during the restart step.
	cmd := exec.Command("bash", updateScriptPath, req.Branch) //nolint:gosec
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to launch update script: " + err.Error()})
		return
	}
	go func() { _ = cmd.Wait() }()

	c.JSON(http.StatusAccepted, gin.H{
		"status": "update_started",
		"branch": req.Branch,
		"msg":    "Update is running in the background. The dashboard will restart automatically in ~60 seconds.",
	})
}

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// readACFBuildID reads the installed buildid from the SteamCMD app manifest
// at <installDir>/steamapps/appmanifest_<appID>.acf.
// Returns "" if the file doesn't exist or the field isn't found.
func readACFBuildID(installDir, appID string) string {
	acfPath := filepath.Join(installDir, "steamapps", "appmanifest_"+appID+".acf")
	data, err := os.ReadFile(acfPath) //nolint:gosec
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`"buildid"\s+"(\d+)"`)
	m := re.FindSubmatch(data)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

// steamUpdateCheckResult is the outcome of checkSteamUpdate.
type steamUpdateCheckResult struct {
	LocalBuildID  string
	RemoteBuildID string // newest required build ID from Steam; may be ""
	UpToDate      bool
	Checked       bool   // false if the API call failed or was skipped
	Note          string // human-readable summary for the console
}

// checkSteamUpdate queries the Steam Web API UpToDateCheck endpoint to
// determine whether a newer build is available for appID.
//
// If localBuildID is "" (no manifest on disk yet), it returns Checked=false
// so the caller proceeds with a normal deploy.
// If the network call fails, it also returns Checked=false so updates are
// never blocked by a connectivity issue.
func checkSteamUpdate(ctx context.Context, appID, localBuildID string) steamUpdateCheckResult {
	if localBuildID == "" {
		return steamUpdateCheckResult{
			Checked: false,
			Note:    fmt.Sprintf("no local manifest for app %s — assuming update needed", appID),
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	url := fmt.Sprintf(
		"https://api.steampowered.com/ISteamApps/UpToDateCheck/v1/?appid=%s&version=%s",
		appID, localBuildID,
	)
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, url, nil)
	if err != nil {
		return steamUpdateCheckResult{LocalBuildID: localBuildID, Checked: false,
			Note: "Steam update check failed (could not build request): " + err.Error()}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return steamUpdateCheckResult{LocalBuildID: localBuildID, Checked: false,
			Note: "Steam update check failed (network): " + err.Error()}
	}
	defer resp.Body.Close() //nolint:errcheck

	var payload struct {
		Response struct {
			Success         bool `json:"success"`
			UpToDate        bool `json:"up_to_date"`
			RequiredVersion int  `json:"required_version"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return steamUpdateCheckResult{LocalBuildID: localBuildID, Checked: false,
			Note: "Steam update check failed (parse error): " + err.Error()}
	}
	if !payload.Response.Success {
		return steamUpdateCheckResult{LocalBuildID: localBuildID, Checked: false,
			Note: fmt.Sprintf("Steam update check returned success=false for app %s", appID)}
	}

	remoteBuildID := ""
	if payload.Response.RequiredVersion > 0 {
		remoteBuildID = strconv.Itoa(payload.Response.RequiredVersion)
	}

	var note string
	if payload.Response.UpToDate {
		note = fmt.Sprintf("app %s is up to date (build %s)", appID, localBuildID)
	} else {
		if remoteBuildID != "" {
			note = fmt.Sprintf("app %s update available: build %s → %s", appID, localBuildID, remoteBuildID)
		} else {
			note = fmt.Sprintf("app %s update available (current build: %s)", appID, localBuildID)
		}
	}

	return steamUpdateCheckResult{
		LocalBuildID:  localBuildID,
		RemoteBuildID: remoteBuildID,
		UpToDate:      payload.Response.UpToDate,
		Checked:       true,
		Note:          note,
	}
}

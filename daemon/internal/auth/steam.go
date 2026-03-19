package auth

// Steam OpenID 2.0 authentication helpers.
//
// Steam uses OpenID 2.0 (not OIDC). The flow is:
//  1. Redirect the browser to SteamLoginURL(...).
//  2. Steam authenticates the user and redirects back to the return_to URL with
//     a set of openid.* query parameters.
//  3. Call VerifySteamCallback to confirm the assertion is genuine by
//     re-posting it to Steam's check_authentication endpoint.
//  4. Extract the 64-bit Steam ID from openid.claimed_id.
//  5. Optionally call GetSteamDisplayName to fetch the user's profile name.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	steamOpenIDEndpoint = "https://steamcommunity.com/openid/login"
	steamOpenIDNS       = "http://specs.openid.net/auth/2.0"
	steamIdentitySelect = "http://specs.openid.net/auth/2.0/identifier_select"
	steamProfileAPI     = "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/"
)

var steamIDRegexp = regexp.MustCompile(`https://steamcommunity\.com/openid/id/(\d+)$`)

// SteamLoginURL builds the URL to redirect the browser to for Steam authentication.
func SteamLoginURL(returnTo, realm string) string {
	params := url.Values{
		"openid.ns":         {steamOpenIDNS},
		"openid.mode":       {"checkid_setup"},
		"openid.return_to":  {returnTo},
		"openid.realm":      {realm},
		"openid.identity":   {steamIdentitySelect},
		"openid.claimed_id": {steamIdentitySelect},
	}
	return steamOpenIDEndpoint + "?" + params.Encode()
}

// VerifySteamCallback validates a Steam OpenID 2.0 callback by re-posting the
// received parameters to Steam's check_authentication endpoint.
// Returns the 64-bit Steam ID string on success.
func VerifySteamCallback(ctx context.Context, rawQuery string) (steamID string, err error) {
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("steam: bad query string: %w", err)
	}

	if params.Get("openid.mode") != "id_res" {
		return "", fmt.Errorf("steam: unexpected openid.mode %q", params.Get("openid.mode"))
	}

	claimedID := params.Get("openid.claimed_id")
	m := steamIDRegexp.FindStringSubmatch(claimedID)
	if m == nil {
		return "", fmt.Errorf("steam: claimed_id %q is not a valid Steam profile URL", claimedID)
	}
	steamID = m[1]

	// Re-post to Steam for check_authentication verification
	checkParams := url.Values{}
	for k, v := range params {
		checkParams[k] = v
	}
	checkParams.Set("openid.mode", "check_authentication")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, steamOpenIDEndpoint,
		strings.NewReader(checkParams.Encode()))
	if err != nil {
		return "", fmt.Errorf("steam: building check_authentication request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("steam: check_authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("steam: reading check_authentication response: %w", err)
	}

	if !strings.Contains(string(body), "is_valid:true") {
		return "", fmt.Errorf("steam: check_authentication returned is_valid:false")
	}

	return steamID, nil
}

// GetSteamDisplayName fetches the persona name for steamID from the Steam Web API.
// Returns the steamID itself if the API key is empty or the request fails.
func GetSteamDisplayName(ctx context.Context, apiKey, steamID string) string {
	if apiKey == "" {
		return steamID
	}

	u := fmt.Sprintf("%s?key=%s&steamids=%s", steamProfileAPI, url.QueryEscape(apiKey), url.QueryEscape(steamID))
	httpClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return steamID
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return steamID
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			Players []struct {
				PersonaName string `json:"personaname"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32768)).Decode(&result); err != nil {
		return steamID
	}
	if len(result.Response.Players) > 0 && result.Response.Players[0].PersonaName != "" {
		return result.Response.Players[0].PersonaName
	}
	return steamID
}

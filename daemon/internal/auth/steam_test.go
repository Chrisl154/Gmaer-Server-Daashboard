package auth

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// newSteamService creates an auth service with Steam enabled.
func newSteamService(t *testing.T, returnURL, realm, frontendURL string) *Service {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	cfg := Config{
		JWTSecret: "test-secret-key-at-least-32-bytes!",
		TokenTTL:  24 * time.Hour,
		Steam: &SteamConfig{
			Enabled:     true,
			APIKey:      "",
			ReturnURL:   returnURL,
			Realm:       realm,
			FrontendURL: frontendURL,
		},
	}
	svc, err := NewService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// ---------- SteamLoginURL (pure helper) ----------

func TestSteamLoginURL_ContainsRequiredParams(t *testing.T) {
	u := SteamLoginURL("https://example.com/cb", "https://example.com")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("SteamLoginURL returned invalid URL: %v", err)
	}
	q := parsed.Query()
	if q.Get("openid.ns") != steamOpenIDNS {
		t.Errorf("missing openid.ns")
	}
	if q.Get("openid.mode") != "checkid_setup" {
		t.Errorf("missing openid.mode")
	}
	if q.Get("openid.return_to") != "https://example.com/cb" {
		t.Errorf("return_to mismatch: %s", q.Get("openid.return_to"))
	}
	if q.Get("openid.realm") != "https://example.com" {
		t.Errorf("realm mismatch")
	}
	if !strings.HasPrefix(u, steamOpenIDEndpoint) {
		t.Errorf("URL should start with Steam endpoint, got %s", u)
	}
}

// ---------- GetSteamLoginURL ----------

func TestGetSteamLoginURL_Disabled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc, _ := NewService(Config{JWTSecret: "x"}, nil, logger)
	_, _, err := svc.GetSteamLoginURL()
	if err == nil {
		t.Error("expected error when Steam is not configured")
	}
}

func TestGetSteamLoginURL_Success(t *testing.T) {
	svc := newSteamService(t, "https://example.com/api/v1/auth/steam/callback", "", "")
	loginURL, state, err := svc.GetSteamLoginURL()
	if err != nil {
		t.Fatalf("GetSteamLoginURL: %v", err)
	}
	if state == "" {
		t.Error("expected non-empty state nonce")
	}
	if !strings.Contains(loginURL, "openid.return_to") {
		t.Error("loginURL missing openid.return_to")
	}
	if !strings.Contains(loginURL, state) {
		t.Error("loginURL should include state nonce in return_to")
	}
}

func TestGetSteamLoginURL_DerivesRealmFromReturnURL(t *testing.T) {
	svc := newSteamService(t, "https://games.example.com/api/v1/auth/steam/callback", "", "")
	loginURL, _, err := svc.GetSteamLoginURL()
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(loginURL)
	realm := parsed.Query().Get("openid.realm")
	if realm != "https://games.example.com" {
		t.Errorf("derived realm incorrect: %s", realm)
	}
}

func TestGetSteamLoginURL_ExplicitRealm(t *testing.T) {
	svc := newSteamService(t, "https://games.example.com/api/v1/auth/steam/callback", "https://custom.realm", "")
	loginURL, _, err := svc.GetSteamLoginURL()
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(loginURL)
	realm := parsed.Query().Get("openid.realm")
	if realm != "https://custom.realm" {
		t.Errorf("expected explicit realm, got %s", realm)
	}
}

// ---------- SteamCallback error paths ----------

func TestSteamCallback_Disabled(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc, _ := NewService(Config{JWTSecret: "x"}, nil, logger)
	_, err := svc.SteamCallback(context.Background(), "openid.mode=id_res")
	if err == nil {
		t.Error("expected error when Steam is not configured")
	}
}

func TestSteamCallback_MissingState(t *testing.T) {
	svc := newSteamService(t, "https://example.com/cb", "", "")
	// No state param — should fail
	_, err := svc.SteamCallback(context.Background(), "openid.mode=id_res")
	if err == nil {
		t.Error("expected error for missing state")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("error should mention state, got: %v", err)
	}
}

func TestSteamCallback_ExpiredState(t *testing.T) {
	svc := newSteamService(t, "https://example.com/cb", "", "")
	// Plant an already-expired nonce
	svc.steamStates.Store("expiredNonce", time.Now().Add(-1*time.Second))
	_, err := svc.SteamCallback(context.Background(), "state=expiredNonce&openid.mode=id_res")
	if err == nil {
		t.Error("expected error for expired state nonce")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired, got: %v", err)
	}
}

func TestSteamCallback_InvalidState(t *testing.T) {
	svc := newSteamService(t, "https://example.com/cb", "", "")
	_, err := svc.SteamCallback(context.Background(), "state=unknownNonce&openid.mode=id_res")
	if err == nil {
		t.Error("expected error for unknown state nonce")
	}
}

// ---------- SteamFrontendURL ----------

func TestSteamFrontendURL_WithConfig(t *testing.T) {
	svc := newSteamService(t, "https://example.com/cb", "", "https://frontend.example.com")
	if got := svc.SteamFrontendURL(); got != "https://frontend.example.com" {
		t.Errorf("SteamFrontendURL: expected configured URL, got %q", got)
	}
}

func TestSteamFrontendURL_NoConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	svc, _ := NewService(Config{JWTSecret: "x"}, nil, logger)
	if got := svc.SteamFrontendURL(); got != "" {
		t.Errorf("SteamFrontendURL should be empty when Steam not configured, got %q", got)
	}
}

// ---------- VerifySteamCallback validation (no network) ----------

func TestVerifySteamCallback_BadMode(t *testing.T) {
	_, err := VerifySteamCallback(context.Background(), "openid.mode=cancel")
	if err == nil {
		t.Error("expected error for non id_res mode")
	}
}

func TestVerifySteamCallback_BadClaimedID(t *testing.T) {
	q := url.Values{
		"openid.mode":       {"id_res"},
		"openid.claimed_id": {"https://not-steam.com/openid/id/12345"},
	}
	_, err := VerifySteamCallback(context.Background(), q.Encode())
	if err == nil {
		t.Error("expected error for non-Steam claimed_id")
	}
}

// ---------- Helper functions ----------

func TestParseURLBase(t *testing.T) {
	base, err := parseURLBase("https://example.com/some/path?query=1")
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://example.com" {
		t.Errorf("got %s", base)
	}
}

func TestDropParam(t *testing.T) {
	raw := "state=abc&openid.mode=id_res&openid.ns=http%3A%2F%2Fspecs.openid.net%2Fauth%2F2.0"
	result := dropParam(raw, "state")
	vals, _ := url.ParseQuery(result)
	if vals.Get("state") != "" {
		t.Error("state should have been dropped")
	}
	if vals.Get("openid.mode") != "id_res" {
		t.Error("openid.mode should remain")
	}
}

func TestParseQuery(t *testing.T) {
	m, err := parseQuery("foo=bar&baz=qux")
	if err != nil {
		t.Fatal(err)
	}
	if m["foo"] != "bar" || m["baz"] != "qux" {
		t.Errorf("unexpected map: %v", m)
	}
}

func TestParseQuery_Invalid(t *testing.T) {
	// url.ParseQuery is lenient, but exercise the function at least
	m, err := parseQuery("")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map for empty query, got %v", m)
	}
}

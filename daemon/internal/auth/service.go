package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/games-dashboard/daemon/internal/notifications"
	"github.com/games-dashboard/daemon/internal/secrets"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// Config holds auth configuration
type Config struct {
	Local    LocalAuthConfig  `yaml:"local" json:"local"`
	OIDC     *OIDCConfig      `yaml:"oidc,omitempty" json:"oidc,omitempty"`
	SAML     *SAMLConfig      `yaml:"saml,omitempty" json:"saml,omitempty"`
	Steam    *SteamConfig     `yaml:"steam,omitempty" json:"steam,omitempty"`
	JWTSecret   string        `yaml:"jwt_secret" json:"jwt_secret"`
	TokenTTL    time.Duration `yaml:"token_ttl" json:"token_ttl"`
	MFARequired bool          `yaml:"mfa_required" json:"mfa_required"`
	// DataDir is the directory where users.json and audit.log are persisted.
	DataDir string `yaml:"data_dir" json:"-"`
}

type LocalAuthConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Admin   User   `yaml:"admin" json:"admin"`
}

type OIDCConfig struct {
	Issuer       string `yaml:"issuer" json:"issuer"`
	ClientID     string `yaml:"client_id" json:"client_id"`
	ClientSecret string `yaml:"client_secret" json:"client_secret"`
	RedirectURL  string `yaml:"redirect_url" json:"redirect_url"`
}

type SAMLConfig struct {
	MetadataURL string `yaml:"metadata_url" json:"metadata_url"`
	EntityID    string `yaml:"entity_id" json:"entity_id"`
}

// SteamConfig holds Steam OpenID 2.0 settings.
type SteamConfig struct {
	// Enabled turns on the Steam login option.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// APIKey is a Steam Web API key used to fetch player display names.
	// Obtain one at https://steamcommunity.com/dev/apikey
	// Optional — if omitted the Steam64 ID is used as the username.
	APIKey string `yaml:"api_key" json:"api_key"`
	// ReturnURL is the full URL Steam will redirect the browser back to after
	// authentication — must be the public /api/v1/auth/steam/callback URL,
	// e.g. "https://your-server.com/api/v1/auth/steam/callback".
	ReturnURL string `yaml:"return_url" json:"return_url"`
	// Realm is the OpenID trust root (typically your dashboard's base URL,
	// e.g. "https://your-server.com"). Defaults to the scheme+host of ReturnURL.
	Realm string `yaml:"realm" json:"realm"`
	// FrontendURL is the base URL of the frontend SPA. After the callback the
	// browser is redirected to {FrontendURL}/login?token=... so the app can
	// store the issued JWT. Leave empty to use a relative redirect (works when
	// the daemon serves the frontend from the same origin).
	FrontendURL string `yaml:"frontend_url" json:"frontend_url"`
}

// APIKey is a long-lived personal access token for scripting / external automation.
// The raw token (gdash_<base64url>) is shown once at creation and never stored —
// only its SHA-256 hash is kept.
type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`               // first 12 chars of raw token (for display)
	Hash      string     `json:"-"`                     // hex-encoded SHA-256 of raw token
	Roles     []string   `json:"roles"`                 // scoped roles (subset of user roles)
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = never expires
}

// PushSubscription holds a Web Push endpoint and its encryption keys.
type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// User represents an authenticated user
type User struct {
	ID                string             `json:"id"`
	Username          string             `json:"username"`
	PasswordHash      string             `json:"-"`
	Roles             []string           `json:"roles"`
	AllowedServers    []string           `json:"allowed_servers,omitempty"` // nil/empty = all servers
	TOTPEnabled       bool               `json:"totp_enabled"`
	TOTPSecret        string             `json:"-"`
	RecoveryCodes     []string           `json:"-"` // single-use backup codes, stored plaintext (in-memory)
	PushSubscriptions []PushSubscription `json:"push_subscriptions,omitempty"`
	APIKeys           []APIKey           `json:"api_keys,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	LastLogin         time.Time          `json:"last_login"`
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	MFADone  bool     `json:"mfa_done"`
	jwt.RegisteredClaims
}

func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role || r == "admin" {
			return true
		}
	}
	return false
}

// LoginRequest is a login payload
type LoginRequest struct {
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password" binding:"required"`
	TOTPCode     string `json:"totp_code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

// LoginResponse contains the JWT token
type LoginResponse struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	MFARequired bool     `json:"mfa_required"`
	User       *User     `json:"user"`
}

// TOTPSetupResponse contains TOTP setup info
type TOTPSetupResponse struct {
	Secret  string `json:"secret"`
	QRCode  string `json:"qr_code_url"`
	Issuer  string `json:"issuer"`
}

type TOTPVerifyRequest struct {
	Code string `json:"code" binding:"required"`
}

// TOTPVerifyResponse is returned on successful TOTP enrollment.
// RecoveryCodes must be shown to the user once — they are not retrievable later.
type TOTPVerifyResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// CreateUserRequest is a user creation payload
type CreateUserRequest struct {
	Username       string   `json:"username" binding:"required"`
	Password       string   `json:"password" binding:"required"`
	Roles          []string `json:"roles"`
	AllowedServers []string `json:"allowed_servers,omitempty"`
}

// UpdateUserRequest is a user update payload
type UpdateUserRequest struct {
	Password       string    `json:"password,omitempty"`
	Roles          []string  `json:"roles,omitempty"`
	AllowedServers *[]string `json:"allowed_servers,omitempty"` // pointer so nil = don't change
}

// AuditEntry is an audit log record
type AuditEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Details   any       `json:"details,omitempty"`
}

// Service handles authentication
type Service struct {
	cfg      Config
	secrets  *secrets.Manager
	logger   *zap.Logger
	users    map[string]*User
	usersMu  sync.RWMutex // protects users map and all User field mutations
	auditLog   []AuditEntry
	auditLogMu sync.RWMutex

	// blocklist holds revoked JWT tokens until their natural expiry.
	// Only logged-out tokens are stored here — keeps memory footprint minimal.
	blocklist   map[string]time.Time // raw token -> expiry time
	blocklistMu sync.Mutex

	// OIDC — initialized once on first use
	oidcOnce     sync.Once
	oidcProvider *gooidc.Provider
	oauth2Cfg    *oauth2.Config
	oidcInitErr  error
	oidcStates   sync.Map // state nonce -> expiry time.Time

	// Steam — nonce map prevents replay attacks on the callback
	steamStates sync.Map // state nonce -> expiry time.Time
}

// NewService creates a new auth service
func NewService(cfg Config, secretsMgr *secrets.Manager, logger *zap.Logger) (*Service, error) {
	svc := &Service{
		cfg:       cfg,
		secrets:   secretsMgr,
		logger:    logger,
		users:     make(map[string]*User),
		auditLog:  []AuditEntry{},
		blocklist: make(map[string]time.Time),
	}

	// Load persisted users from disk first, then merge the admin from config
	// so daemon.yaml remains authoritative for the admin password.
	svc.loadUsers()
	svc.mergeAdminFromConfig()

	// Load the persisted audit log into memory.
	svc.loadAuditLog()

	// P39: periodically evict expired tokens from the blocklist so it stays bounded.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			svc.purgeBlocklist()
		}
	}()

	return svc, nil
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	s.usersMu.RLock()
	user, exists := s.users[req.Username]
	s.usersMu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.audit(user.ID, user.Username, "login", "auth", "", false, nil)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Enforce global MFA requirement before issuing a token.
	if s.cfg.MFARequired && !user.TOTPEnabled {
		return nil, fmt.Errorf("MFA is required — please set up two-factor authentication before logging in")
	}

	mfaDone := false

	if user.TOTPEnabled {
		if req.TOTPCode == "" && req.RecoveryCode == "" {
			return &LoginResponse{
				MFARequired: true,
				User:        sanitizeUser(user),
			}, nil
		}
		if req.RecoveryCode != "" {
			// Try to consume a single-use recovery code
			if !consumeRecoveryCode(user, req.RecoveryCode) {
				return nil, fmt.Errorf("invalid recovery code")
			}
		} else if !totp.Validate(req.TOTPCode, user.TOTPSecret) {
			return nil, fmt.Errorf("invalid TOTP code")
		}
		mfaDone = true
	}

	ttl := s.cfg.TokenTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	expiresAt := time.Now().Add(ttl)
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		MFADone:  mfaDone,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	s.usersMu.Lock()
	user.LastLogin = time.Now()
	s.usersMu.Unlock()
	go s.saveUsers()

	s.audit(user.ID, user.Username, "login", "auth", "", true, nil)

	return &LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User:      sanitizeUser(user),
	}, nil
}

// Logout immediately revokes a token by adding it to the blocklist.
// The token remains in the blocklist until its natural JWT expiry so that
// ValidateToken can reject any reuse attempt within the original TTL window.
func (s *Service) Logout(ctx context.Context, rawHeader string) error {
	tokenStr := strings.TrimPrefix(rawHeader, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil
	}

	// Parse without full validation so we can read the expiry even if the
	// clock has already passed it (shouldn't happen at logout but be safe).
	var expiresAt time.Time
	parsed, _ := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if parsed != nil {
		if c, ok := parsed.Claims.(*Claims); ok && c.ExpiresAt != nil {
			expiresAt = c.ExpiresAt.Time
		}
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(s.cfg.TokenTTL)
	}

	s.blocklistMu.Lock()
	s.blocklist[tokenStr] = expiresAt
	s.blocklistMu.Unlock()

	s.purgeBlocklist()
	return nil
}

// purgeBlocklist removes expired entries so the blocklist stays bounded.
func (s *Service) purgeBlocklist() {
	now := time.Now()
	s.blocklistMu.Lock()
	defer s.blocklistMu.Unlock()
	for tok, exp := range s.blocklist {
		if now.After(exp) {
			delete(s.blocklist, tok)
		}
	}
}

// ValidateToken validates and parses a JWT token, and rejects revoked tokens.
func (s *Service) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	// Strip "Bearer " prefix
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	tokenStr = strings.TrimSpace(tokenStr)

	// API key path — tokens start with "gdash_"
	if strings.HasPrefix(tokenStr, "gdash_") {
		return s.validateAPIKey(tokenStr)
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Reject tokens that have been explicitly revoked via Logout.
	s.blocklistMu.Lock()
	_, revoked := s.blocklist[tokenStr]
	s.blocklistMu.Unlock()
	if revoked {
		return nil, fmt.Errorf("token has been revoked")
	}

	return claims, nil
}

// validateAPIKey looks up a gdash_ API key by its SHA-256 hash across all users.
func (s *Service) validateAPIKey(raw string) (*Claims, error) {
	hash := hashAPIKey(raw)
	now := time.Now()

	s.usersMu.Lock()
	defer s.usersMu.Unlock()

	for _, u := range s.users {
		for i, key := range u.APIKeys {
			if key.Hash != hash {
				continue
			}
			if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
				return nil, fmt.Errorf("API key expired")
			}
			// Update last-used timestamp and persist asynchronously.
			u.APIKeys[i].LastUsed = &now
			go s.saveUsers()
			return &Claims{
				UserID:   u.ID,
				Username: u.Username,
				Roles:    key.Roles,
				MFADone:  true,
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid API key")
}

// SetupTOTP initializes TOTP for a user. If TOTP is already enabled,
// currentCode must be a valid TOTP code to prevent account takeover.
func (s *Service) SetupTOTP(ctx context.Context, claims *Claims, currentCode string) (*TOTPSetupResponse, error) {
	if claims == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if user.TOTPEnabled {
		if currentCode == "" || !totp.Validate(currentCode, user.TOTPSecret) {
			return nil, fmt.Errorf("current TOTP code required to re-enroll 2FA")
		}
	}

	secret := generateTOTPSecret()
	user.TOTPSecret = secret

	qrURL := fmt.Sprintf("otpauth://totp/GamesDashboard:%s?secret=%s&issuer=GamesDashboard", user.Username, secret)

	return &TOTPSetupResponse{
		Secret: secret,
		QRCode: qrURL,
		Issuer: "GamesDashboard",
	}, nil
}

// VerifyTOTP verifies a TOTP code, enables TOTP for the user, and returns
// fresh recovery codes. The codes are single-use backup codes that can be
// used in place of a TOTP code when the user loses their device.
func (s *Service) VerifyTOTP(ctx context.Context, claims *Claims, code string) (*TOTPVerifyResponse, error) {
	if claims == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if !totp.Validate(code, user.TOTPSecret) {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	codes := generateRecoveryCodes(10)
	hashes, err := hashRecoveryCodes(codes)
	if err != nil {
		return nil, fmt.Errorf("could not hash recovery codes: %w", err)
	}
	user.TOTPEnabled = true
	user.RecoveryCodes = hashes // store bcrypt hashes, not plaintext
	go s.saveUsers()

	return &TOTPVerifyResponse{RecoveryCodes: codes}, nil // return plaintext once
}

// GetRecoveryCodesCount returns how many unused recovery codes the user has left.
func (s *Service) GetRecoveryCodesCount(ctx context.Context, claims *Claims) (int, error) {
	if claims == nil {
		return 0, fmt.Errorf("not authenticated")
	}
	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return 0, fmt.Errorf("user not found")
	}
	if !user.TOTPEnabled {
		return 0, fmt.Errorf("TOTP not enabled")
	}
	return len(user.RecoveryCodes), nil
}

// RegenerateRecoveryCodes burns all existing recovery codes and issues 10 new ones.
// Requires a valid TOTP code to authorize the regeneration.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, claims *Claims, totpCode string) (*TOTPVerifyResponse, error) {
	if claims == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	if !user.TOTPEnabled {
		return nil, fmt.Errorf("TOTP not enabled")
	}
	if !totp.Validate(totpCode, user.TOTPSecret) {
		return nil, fmt.Errorf("invalid TOTP code")
	}
	codes := generateRecoveryCodes(10)
	hashes, err := hashRecoveryCodes(codes)
	if err != nil {
		return nil, fmt.Errorf("could not hash recovery codes: %w", err)
	}
	user.RecoveryCodes = hashes // store bcrypt hashes, not plaintext
	go s.saveUsers()
	return &TOTPVerifyResponse{RecoveryCodes: codes}, nil // return plaintext once
}

// initOIDC initializes the OIDC provider and OAuth2 config on first use.
// Uses sync.Once so the provider discovery call happens at most once.
func (s *Service) initOIDC() error {
	s.oidcOnce.Do(func() {
		if s.cfg.OIDC == nil || s.cfg.OIDC.Issuer == "" {
			s.oidcInitErr = fmt.Errorf("OIDC not configured")
			return
		}
		provider, err := gooidc.NewProvider(context.Background(), s.cfg.OIDC.Issuer)
		if err != nil {
			s.oidcInitErr = fmt.Errorf("OIDC provider discovery for %s failed: %w", s.cfg.OIDC.Issuer, err)
			s.logger.Warn("OIDC provider init failed", zap.Error(err))
			return
		}
		s.oidcProvider = provider
		s.oauth2Cfg = &oauth2.Config{
			ClientID:     s.cfg.OIDC.ClientID,
			ClientSecret: s.cfg.OIDC.ClientSecret,
			RedirectURL:  s.cfg.OIDC.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		}
		s.logger.Info("OIDC provider initialized", zap.String("issuer", s.cfg.OIDC.Issuer))
	})
	return s.oidcInitErr
}

// GetOIDCAuthURL returns the authorization URL to redirect the browser to,
// along with the state nonce (store it in the session for validation on callback).
func (s *Service) GetOIDCAuthURL(ctx context.Context) (authURL, state string, err error) {
	if initErr := s.initOIDC(); initErr != nil {
		return "", "", initErr
	}
	state = generateID()
	s.oidcStates.Store(state, time.Now().Add(5*time.Minute))
	authURL = s.oauth2Cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return authURL, state, nil
}

// OIDCCallback exchanges the authorization code for tokens, verifies the ID token,
// and issues a Games Dashboard JWT for the authenticated user.
func (s *Service) OIDCCallback(ctx context.Context, code, state string) (*LoginResponse, error) {
	if initErr := s.initOIDC(); initErr != nil {
		return nil, initErr
	}

	// Validate state nonce (prevents CSRF)
	expiry, ok := s.oidcStates.LoadAndDelete(state)
	if !ok || time.Now().After(expiry.(time.Time)) {
		return nil, fmt.Errorf("invalid or expired OAuth state")
	}

	// Exchange authorization code for OAuth2 tokens
	oauth2Token, err := s.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("authorization code exchange failed: %w", err)
	}

	// Extract and verify the ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	verifier := s.oidcProvider.Verifier(&gooidc.Config{ClientID: s.cfg.OIDC.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID token verification failed: %w", err)
	}

	// Extract user claims from the ID token
	var oidcClaims struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
	}
	if err := idToken.Claims(&oidcClaims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	// Derive a local username: prefer email, then preferred_username, then sub
	username := oidcClaims.Email
	if username == "" {
		username = oidcClaims.PreferredUsername
	}
	if username == "" {
		username = oidcClaims.Sub
	}

	// Find or create a local user record for this OIDC identity
	s.usersMu.Lock()
	user, exists := s.users[username]
	if !exists {
		user = &User{
			ID:        generateID(),
			Username:  username,
			Roles:     []string{"viewer"},
			CreatedAt: time.Now(),
		}
		s.users[username] = user
		s.logger.Info("Created user from OIDC", zap.String("username", username))
	}
	user.LastLogin = time.Now()
	s.usersMu.Unlock()
	go s.saveUsers()

	// Issue a Games Dashboard JWT
	ttl := s.cfg.TokenTTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	expiresAt := time.Now().Add(ttl)
	jwtClaims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		MFADone:  true, // OIDC flow is treated as MFA-complete
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	tokenStr, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign JWT: %w", err)
	}

	s.audit(user.ID, user.Username, "oidc-login", "auth", "", true,
		map[string]string{"sub": oidcClaims.Sub, "issuer": s.cfg.OIDC.Issuer})

	return &LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User:      sanitizeUser(user),
	}, nil
}

// ListUsers returns all users
func (s *Service) ListUsers(ctx context.Context) ([]*User, error) {
	s.usersMu.RLock()
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, sanitizeUser(u))
	}
	s.usersMu.RUnlock()
	return users, nil
}

// IsInitialized returns true when at least one user account exists.
func (s *Service) IsInitialized() bool {
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()
	return len(s.users) > 0
}

// BootstrapAdmin creates the very first admin user. It returns the sanitised
// user and the raw bcrypt password hash so the caller can persist it to
// daemon.yaml. Returns an error if the system is already initialised.
func (s *Service) BootstrapAdmin(ctx context.Context, req CreateUserRequest) (*User, string, error) {
	if s.IsInitialized() {
		return nil, "", fmt.Errorf("system already initialized")
	}
	req.Roles = []string{"admin"}
	user, err := s.CreateUser(ctx, req)
	if err != nil {
		return nil, "", err
	}
	s.usersMu.RLock()
	hash := s.users[req.Username].PasswordHash
	s.usersMu.RUnlock()
	return user, hash, nil
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	s.usersMu.Lock()
	_, exists := s.users[req.Username]
	s.usersMu.Unlock()
	if exists {
		return nil, fmt.Errorf("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:             generateID(),
		Username:       req.Username,
		PasswordHash:   string(hash),
		Roles:          req.Roles,
		AllowedServers: req.AllowedServers,
		CreatedAt:      time.Now(),
	}

	s.usersMu.Lock()
	s.users[req.Username] = user
	s.usersMu.Unlock()
	go s.saveUsers()
	return sanitizeUser(user), nil
}

// UpdateUser updates user fields
func (s *Service) UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*User, error) {
	s.usersMu.Lock()
	user, exists := s.getUserByID(userID)
	if !exists {
		s.usersMu.Unlock()
		return nil, fmt.Errorf("user not found")
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(hash)
	}

	if len(req.Roles) > 0 {
		user.Roles = req.Roles
	}

	if req.AllowedServers != nil {
		user.AllowedServers = *req.AllowedServers
	}
	s.usersMu.Unlock()
	go s.saveUsers()
	return sanitizeUser(user), nil
}

// DeleteUser removes a user
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	s.usersMu.Lock()
	for username, u := range s.users {
		if u.ID == userID {
			delete(s.users, username)
			s.usersMu.Unlock()
			go s.saveUsers()
			return nil
		}
	}
	s.usersMu.Unlock()
	return fmt.Errorf("user not found")
}

// GetAuditLog returns a paginated slice of the audit log (newest first).
// offset and limit default to 0 and 100; max limit is 1000.
func (s *Service) GetAuditLog(ctx context.Context, offset, limit int) ([]AuditEntry, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	s.auditLogMu.RLock()
	all := make([]AuditEntry, len(s.auditLog))
	copy(all, s.auditLog)
	total := len(all)
	s.auditLogMu.RUnlock()

	// Return newest first.
	reversed := make([]AuditEntry, total)
	for i, e := range all {
		reversed[total-1-i] = e
	}
	if offset >= total {
		return []AuditEntry{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return reversed[offset:end], total, nil
}

// RecordEvent appends a system event to the audit log (called by API handlers for
// server lifecycle, backup, and mod operations).
func (s *Service) RecordEvent(userID, username, action, resource, ip string, success bool, details any) {
	s.audit(userID, username, action, resource, ip, success, details)
}

// getUserByID finds a user by ID. Caller must hold s.usersMu (read or write).
func (s *Service) getUserByID(id string) (*User, bool) {
	for _, u := range s.users {
		if u.ID == id {
			return u, true
		}
	}
	return nil, false
}

func (s *Service) audit(userID, username, action, resource, ip string, success bool, details any) {
	entry := AuditEntry{
		ID:        generateID(),
		UserID:    userID,
		Username:  username,
		Action:    action,
		Resource:  resource,
		IP:        ip,
		Timestamp: time.Now(),
		Success:   success,
		Details:   details,
	}
	s.auditLogMu.Lock()
	s.auditLog = append(s.auditLog, entry)
	s.auditLogMu.Unlock()
	// Persist to disk so audit trail survives restarts.
	go s.appendAuditEntry(entry)
}

// CanAccessServer returns true when the user is allowed to access the given server.
// An empty AllowedServers list means the user can access all servers.
func (s *Service) CanAccessServer(userID, serverID string) bool {
	u, ok := s.getUserByID(userID)
	if !ok {
		return false
	}
	if len(u.AllowedServers) == 0 {
		return true
	}
	for _, id := range u.AllowedServers {
		if id == serverID {
			return true
		}
	}
	return false
}

// GetAllowedServers returns the list of server IDs the user is restricted to.
// An empty slice means no restriction (access to all).
func (s *Service) GetAllowedServers(userID string) []string {
	u, ok := s.getUserByID(userID)
	if !ok {
		return nil
	}
	return u.AllowedServers
}

func sanitizeUser(u *User) *User {
	return &User{
		ID:             u.ID,
		Username:       u.Username,
		Roles:          u.Roles,
		AllowedServers: u.AllowedServers,
		TOTPEnabled:    u.TOTPEnabled,
		CreatedAt:      u.CreatedAt,
		LastLogin:      u.LastLogin,
	}
}

func generateTOTPSecret() string {
	b := make([]byte, 20)
	rand.Read(b)
	return base32.StdEncoding.EncodeToString(b)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// generateRecoveryCodes returns n single-use backup codes in the format
// "xxxxxxxx-xxxxxxxx" (8 random hex bytes split into two groups of 4).
func generateRecoveryCodes(n int) []string {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 8)
		rand.Read(b)
		codes[i] = fmt.Sprintf("%x-%x", b[:4], b[4:])
	}
	return codes
}

// hashRecoveryCodes returns a bcrypt hash for each plaintext recovery code.
// P37: codes are stored as hashes so the raw values are never persisted.
func hashRecoveryCodes(codes []string) ([]string, error) {
	hashes := make([]string, len(codes))
	for i, c := range codes {
		h, err := bcrypt.GenerateFromPassword([]byte(c), 12)
		if err != nil {
			return nil, err
		}
		hashes[i] = string(h)
	}
	return hashes, nil
}

// consumeRecoveryCode attempts to find and remove the given code from the
// user's recovery code list. P38: uses bcrypt comparison (timing-safe).
// Returns true if the code matched any stored hash and was consumed.
func consumeRecoveryCode(user *User, code string) bool {
	for i, h := range user.RecoveryCodes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
			user.RecoveryCodes = append(user.RecoveryCodes[:i], user.RecoveryCodes[i+1:]...)
			return true
		}
	}
	return false
}

// GetSteamLoginURL returns the URL to redirect the browser to for Steam auth.
// state is a random nonce stored server-side to prevent CSRF on the callback.
func (s *Service) GetSteamLoginURL() (loginURL, state string, err error) {
	if s.cfg.Steam == nil || !s.cfg.Steam.Enabled {
		return "", "", fmt.Errorf("steam auth not configured")
	}
	state = generateID()
	s.steamStates.Store(state, time.Now().Add(10*time.Minute))

	returnTo := s.cfg.Steam.ReturnURL + "?state=" + state
	realm := s.cfg.Steam.Realm
	if realm == "" {
		// Derive realm from ReturnURL (scheme + host)
		if u, parseErr := parseURLBase(s.cfg.Steam.ReturnURL); parseErr == nil {
			realm = u
		} else {
			realm = s.cfg.Steam.ReturnURL
		}
	}
	return SteamLoginURL(returnTo, realm), state, nil
}

// SteamCallback verifies the OpenID 2.0 assertion from Steam, finds or creates
// a local user, and issues a Games Dashboard JWT.
func (s *Service) SteamCallback(ctx context.Context, rawQuery string) (*LoginResponse, error) {
	if s.cfg.Steam == nil || !s.cfg.Steam.Enabled {
		return nil, fmt.Errorf("steam auth not configured")
	}

	// Validate state nonce
	params, parseErr := parseQuery(rawQuery)
	if parseErr != nil {
		return nil, fmt.Errorf("steam: bad callback query: %w", parseErr)
	}
	state := params["state"]
	if state == "" {
		return nil, fmt.Errorf("steam: missing state nonce")
	}
	expiry, ok := s.steamStates.LoadAndDelete(state)
	if !ok || time.Now().After(expiry.(time.Time)) {
		return nil, fmt.Errorf("steam: invalid or expired state nonce")
	}

	// Remove our state param before passing to Steam verifier
	openIDQuery := dropParam(rawQuery, "state")

	steamID, err := VerifySteamCallback(ctx, openIDQuery)
	if err != nil {
		s.audit("", "unknown", "steam-login", "auth", "", false, map[string]string{"error": err.Error()})
		return nil, err
	}

	displayName := GetSteamDisplayName(ctx, s.cfg.Steam.APIKey, steamID)

	// Find or create local user keyed on "steam:<steamID>"
	userKey := "steam:" + steamID
	s.usersMu.Lock()
	user, exists := s.users[userKey]
	if !exists {
		user = &User{
			ID:        generateID(),
			Username:  displayName,
			Roles:     []string{"viewer"},
			CreatedAt: time.Now(),
		}
		s.users[userKey] = user
		s.logger.Info("Created user from Steam", zap.String("steam_id", steamID), zap.String("username", displayName))
	}
	user.LastLogin = time.Now()
	s.usersMu.Unlock()
	go s.saveUsers()

	expiresAt := time.Now().Add(s.cfg.TokenTTL)
	jwtClaims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		MFADone:  true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims).
		SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("steam: failed to sign JWT: %w", err)
	}
	s.audit(user.ID, user.Username, "steam-login", "auth", "", true, map[string]string{"steam_id": steamID})

	return &LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User:      sanitizeUser(user),
	}, nil
}

// SteamFrontendURL returns the configured frontend base URL, or empty string
// (which the API layer will treat as same-origin / relative).
func (s *Service) SteamFrontendURL() string {
	if s.cfg.Steam != nil {
		return s.cfg.Steam.FrontendURL
	}
	return ""
}

// parseURLBase returns scheme://host from a full URL string.
func parseURLBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

// dropParam removes a single key from a raw query string.
func dropParam(rawQuery, key string) string {
	vals, _ := url.ParseQuery(rawQuery)
	vals.Del(key)
	return vals.Encode()
}

// parseQuery returns the query string as a flat map.
func parseQuery(rawQuery string) (map[string]string, error) {
	vals, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(vals))
	for k := range vals {
		out[k] = vals.Get(k)
	}
	return out, nil
}

// --- Web Push subscription management ---

// AddPushSubscription saves a Web Push subscription for the given user.
// If the endpoint is already registered it is replaced (idempotent).
func (s *Service) AddPushSubscription(userID string, sub PushSubscription) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			// Replace existing entry with same endpoint, or append.
			for i, existing := range u.PushSubscriptions {
				if existing.Endpoint == sub.Endpoint {
					u.PushSubscriptions[i] = sub
					return nil
				}
			}
			u.PushSubscriptions = append(u.PushSubscriptions, sub)
			go s.saveUsers()
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

// RemovePushSubscription deletes the subscription with the given endpoint for a user.
func (s *Service) RemovePushSubscription(userID, endpoint string) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			subs := u.PushSubscriptions[:0]
			for _, sub := range u.PushSubscriptions {
				if sub.Endpoint != endpoint {
					subs = append(subs, sub)
				}
			}
			u.PushSubscriptions = subs
			go s.saveUsers()
			return
		}
	}
}

// RemovePushSubscriptionByEndpoint removes the subscription with the given endpoint
// across all users. Called by the push service when a 410 Gone is received. P50.
func (s *Service) RemovePushSubscriptionByEndpoint(endpoint string) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	for _, u := range s.users {
		n := u.PushSubscriptions[:0]
		for _, sub := range u.PushSubscriptions {
			if sub.Endpoint != endpoint {
				n = append(n, sub)
			}
		}
		u.PushSubscriptions = n
	}
	go s.saveUsers()
}

// GetAllWebPushSubs returns a flat list of all push subscriptions across all users.
// This is used by the notifications service to fan out to every subscriber.
func (s *Service) GetAllWebPushSubs() []notifications.WebPushSub {
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()
	var all []notifications.WebPushSub
	for _, u := range s.users {
		for _, sub := range u.PushSubscriptions {
			all = append(all, notifications.WebPushSub{
				Endpoint: sub.Endpoint,
				P256DH:   sub.Keys.P256DH,
				Auth:     sub.Keys.Auth,
			})
		}
	}
	return all
}

// --- API key management ---

// hashAPIKey returns a hex-encoded SHA-256 hash of the raw token.
func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// generateAPIKeyToken creates a new random gdash_ prefixed token.
func generateAPIKeyToken() (string, error) {
	b := make([]byte, 24) // 24 bytes → 32-char base64url
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gdash_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateAPIKeyRequest is the request body for creating an API key.
type CreateAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required"`
	Roles     []string   `json:"roles"` // if empty, inherits caller's roles
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyResponse includes the key metadata and the raw token (shown once).
type CreateAPIKeyResponse struct {
	Key   APIKey `json:"key"`
	Token string `json:"token"` // raw token — show to the user once and discard
}

// CreateAPIKey mints a new personal access token for the given user.
func (s *Service) CreateAPIKey(ctx context.Context, callerClaims *Claims, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	raw, err := generateAPIKeyToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	// Scope roles to caller's roles (can't escalate).
	roles := req.Roles
	if len(roles) == 0 {
		roles = callerClaims.Roles
	} else {
		// Filter to only roles the caller actually has.
		allowed := make(map[string]bool)
		for _, r := range callerClaims.Roles {
			allowed[r] = true
		}
		var scoped []string
		for _, r := range roles {
			if allowed[r] || allowed["admin"] {
				scoped = append(scoped, r)
			}
		}
		if len(scoped) == 0 {
			return nil, fmt.Errorf("no valid roles — caller has %v", callerClaims.Roles)
		}
		roles = scoped
	}

	key := APIKey{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Prefix:    raw[:12],
		Hash:      hashAPIKey(raw),
		Roles:     roles,
		CreatedAt: time.Now(),
		ExpiresAt: req.ExpiresAt,
	}

	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	for _, u := range s.users {
		if u.ID == callerClaims.UserID {
			u.APIKeys = append(u.APIKeys, key)
			go s.saveUsers()
			s.audit(u.ID, u.Username, "api_key.create", "auth", "", true, map[string]any{"key_name": key.Name})
			return &CreateAPIKeyResponse{Key: key, Token: raw}, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// ListAPIKeys returns the API keys for the given user (hashes omitted).
func (s *Service) ListAPIKeys(userID string) ([]APIKey, error) {
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()
	for _, u := range s.users {
		if u.ID == userID {
			out := make([]APIKey, len(u.APIKeys))
			copy(out, u.APIKeys)
			return out, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// RevokeAPIKey removes an API key by ID for the given user.
func (s *Service) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			orig := len(u.APIKeys)
			keys := u.APIKeys[:0]
			for _, k := range u.APIKeys {
				if k.ID != keyID {
					keys = append(keys, k)
				}
			}
			if len(keys) == orig {
				return fmt.Errorf("API key not found")
			}
			u.APIKeys = keys
			go s.saveUsers()
			s.audit(u.ID, u.Username, "api_key.revoke", "auth", "", true, map[string]any{"key_id": keyID})
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

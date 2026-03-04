package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/games-dashboard/daemon/internal/secrets"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Config holds auth configuration
type Config struct {
	Local    LocalAuthConfig  `yaml:"local" json:"local"`
	OIDC     *OIDCConfig      `yaml:"oidc,omitempty" json:"oidc,omitempty"`
	SAML     *SAMLConfig      `yaml:"saml,omitempty" json:"saml,omitempty"`
	JWTSecret string          `yaml:"jwt_secret" json:"jwt_secret"`
	TokenTTL time.Duration    `yaml:"token_ttl" json:"token_ttl"`
	MFARequired bool          `yaml:"mfa_required" json:"mfa_required"`
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

// User represents an authenticated user
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Roles        []string  `json:"roles"`
	TOTPEnabled  bool      `json:"totp_enabled"`
	TOTPSecret   string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login"`
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
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code,omitempty"`
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

// CreateUserRequest is a user creation payload
type CreateUserRequest struct {
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required"`
	Roles    []string `json:"roles"`
}

// UpdateUserRequest is a user update payload
type UpdateUserRequest struct {
	Password string   `json:"password,omitempty"`
	Roles    []string `json:"roles,omitempty"`
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
	cfg        Config
	secrets    *secrets.Manager
	logger     *zap.Logger
	users      map[string]*User
	auditLog   []AuditEntry
	tokenCache map[string]*Claims
}

// NewService creates a new auth service
func NewService(cfg Config, secretsMgr *secrets.Manager, logger *zap.Logger) (*Service, error) {
	svc := &Service{
		cfg:        cfg,
		secrets:    secretsMgr,
		logger:     logger,
		users:      make(map[string]*User),
		auditLog:   []AuditEntry{},
		tokenCache: make(map[string]*Claims),
	}

	// Seed admin user
	if cfg.Local.Enabled && cfg.Local.Admin.Username != "" {
		svc.users[cfg.Local.Admin.Username] = &User{
			ID:           "admin-0",
			Username:     cfg.Local.Admin.Username,
			PasswordHash: cfg.Local.Admin.PasswordHash,
			Roles:        []string{"admin"},
			CreatedAt:    time.Now(),
		}
	}

	return svc, nil
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, exists := s.users[req.Username]
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.audit(user.ID, user.Username, "login", "auth", "", false, nil)
		return nil, fmt.Errorf("invalid credentials")
	}

	mfaRequired := user.TOTPEnabled && s.cfg.MFARequired
	mfaDone := false

	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			return &LoginResponse{
				MFARequired: true,
				User:        sanitizeUser(user),
			}, nil
		}
		if !totp.Validate(req.TOTPCode, user.TOTPSecret) {
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

	s.tokenCache[tokenStr] = claims
	user.LastLogin = time.Now()

	s.audit(user.ID, user.Username, "login", "auth", "", true, nil)
	_ = mfaRequired

	return &LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User:      sanitizeUser(user),
	}, nil
}

// Logout invalidates a token
func (s *Service) Logout(ctx context.Context, token string) error {
	delete(s.tokenCache, token)
	return nil
}

// ValidateToken validates and parses a JWT token
func (s *Service) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	// Strip "Bearer " prefix
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
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

	return claims, nil
}

// SetupTOTP initializes TOTP for a user
func (s *Service) SetupTOTP(ctx context.Context, claims *Claims) (*TOTPSetupResponse, error) {
	if claims == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return nil, fmt.Errorf("user not found")
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

// VerifyTOTP verifies a TOTP code and enables TOTP for the user
func (s *Service) VerifyTOTP(ctx context.Context, claims *Claims, code string) error {
	if claims == nil {
		return fmt.Errorf("not authenticated")
	}

	user, exists := s.getUserByID(claims.UserID)
	if !exists {
		return fmt.Errorf("user not found")
	}

	if !totp.Validate(code, user.TOTPSecret) {
		return fmt.Errorf("invalid TOTP code")
	}

	user.TOTPEnabled = true
	return nil
}

// OIDCCallback handles OIDC callback
func (s *Service) OIDCCallback(ctx context.Context, code, state string) (*LoginResponse, error) {
	// TODO: implement full OIDC flow with coreos/go-oidc
	return nil, fmt.Errorf("OIDC not configured")
}

// ListUsers returns all users
func (s *Service) ListUsers(ctx context.Context) ([]*User, error) {
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, sanitizeUser(u))
	}
	return users, nil
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	if _, exists := s.users[req.Username]; exists {
		return nil, fmt.Errorf("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           generateID(),
		Username:     req.Username,
		PasswordHash: string(hash),
		Roles:        req.Roles,
		CreatedAt:    time.Now(),
	}

	s.users[req.Username] = user
	return sanitizeUser(user), nil
}

// UpdateUser updates user fields
func (s *Service) UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*User, error) {
	user, exists := s.getUserByID(userID)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(hash)
	}

	if len(req.Roles) > 0 {
		user.Roles = req.Roles
	}

	return sanitizeUser(user), nil
}

// DeleteUser removes a user
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	for username, u := range s.users {
		if u.ID == userID {
			delete(s.users, username)
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

// GetAuditLog returns the audit log
func (s *Service) GetAuditLog(ctx context.Context) ([]AuditEntry, error) {
	return s.auditLog, nil
}

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
	s.auditLog = append(s.auditLog, entry)
}

func sanitizeUser(u *User) *User {
	return &User{
		ID:          u.ID,
		Username:    u.Username,
		Roles:       u.Roles,
		TOTPEnabled: u.TOTPEnabled,
		CreatedAt:   u.CreatedAt,
		LastLogin:   u.LastLogin,
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

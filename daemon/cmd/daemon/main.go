package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/games-dashboard/daemon/internal/api"
	"github.com/games-dashboard/daemon/internal/auth"
	"github.com/games-dashboard/daemon/internal/broker"
	"github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/firewall"
	"github.com/games-dashboard/daemon/internal/health"
	"github.com/games-dashboard/daemon/internal/metrics"
	"github.com/games-dashboard/daemon/internal/notifications"
	"github.com/games-dashboard/daemon/internal/secrets"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var (
	cfgFile  string
	logLevel string
	tlsCert  string
	tlsKey   string
	bindAddr string
	noTLS    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "games-daemon",
		Short: "Gaming Server Dashboard Daemon",
		Long: `Games Dashboard Daemon - manages game server lifecycle, backups,
mods, networking, and exposes a secure REST/WebSocket API.`,
		RunE: runDaemon,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/games-dashboard/daemon.yaml", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	rootCmd.PersistentFlags().StringVar(&tlsCert, "tls-cert", "", "path to TLS certificate PEM")
	rootCmd.PersistentFlags().StringVar(&tlsKey, "tls-key", "", "path to TLS private key PEM")
	rootCmd.PersistentFlags().StringVar(&bindAddr, "bind", ":8443", "daemon bind address")
	rootCmd.PersistentFlags().BoolVar(&noTLS, "no-tls", false, "run plain HTTP (testing only — do not use in production)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logger, err := initLogger(logLevel)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("Games Dashboard Daemon starting", zap.String("version", version()))

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override with flags
	if tlsCert != "" {
		cfg.TLS.CertFile = tlsCert
	}
	if tlsKey != "" {
		cfg.TLS.KeyFile = tlsKey
	}

	// Initialize secrets manager
	secretsMgr, err := secrets.NewManager(secrets.Config{
		Backend:    cfg.Secrets.Backend,
		KeyFile:    cfg.Secrets.KeyFile,
		VaultAddr:  cfg.Secrets.VaultAddr,
		VaultToken: cfg.Secrets.VaultToken,
		VaultPath:  cfg.Secrets.VaultPath,
	}, logger)
	if err != nil {
		return fmt.Errorf("failed to init secrets manager: %w", err)
	}

	// Resolve JWT secret — loads the encrypted secret from disk or generates a
	// new 64-byte (128 hex char) secret, encrypts it via the secrets manager,
	// and persists the ciphertext. Must run after secretsMgr is ready.
	jwtSecret, err := resolveJWTSecret(cfg.Storage.DataDir, secretsMgr, logger)
	if err != nil {
		return fmt.Errorf("failed to resolve JWT secret: %w", err)
	}
	cfg.Auth.JWTSecret = jwtSecret

	// Map config.AuthConfig → auth.Config (the two packages define parallel types)
	authCfg := auth.Config{
		Local: auth.LocalAuthConfig{
			Enabled: cfg.Auth.Local.Enabled,
			Admin: auth.User{
				Username:     cfg.Auth.Local.AdminUser,
				PasswordHash: cfg.Auth.Local.AdminPassHash,
			},
		},
		JWTSecret:   cfg.Auth.JWTSecret,
		TokenTTL:    cfg.Auth.TokenTTL,
		MFARequired: cfg.Auth.MFARequired,
		DataDir:     cfg.Storage.DataDir,
	}
	if cfg.Auth.OIDC != nil {
		authCfg.OIDC = &auth.OIDCConfig{
			Issuer:       cfg.Auth.OIDC.Issuer,
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.Auth.OIDC.RedirectURL,
		}
	}
	if cfg.Auth.Steam != nil {
		authCfg.Steam = &auth.SteamConfig{
			Enabled:     cfg.Auth.Steam.Enabled,
			APIKey:      cfg.Auth.Steam.APIKey,
			ReturnURL:   cfg.Auth.Steam.ReturnURL,
			Realm:       cfg.Auth.Steam.Realm,
			FrontendURL: cfg.Auth.Steam.FrontendURL,
		}
	}

	// Initialize auth service
	authSvc, err := auth.NewService(authCfg, secretsMgr, logger)
	if err != nil {
		return fmt.Errorf("failed to init auth service: %w", err)
	}

	// Initialize metrics
	metricsSvc := metrics.NewService()

	// Initialize health service
	healthSvc := health.NewService()

	// Initialize game broker
	gameBroker, err := broker.NewBroker(cfg, secretsMgr, logger, metricsSvc)
	if err != nil {
		return fmt.Errorf("failed to init game broker: %w", err)
	}

	notifySvc := gameBroker.NotifyService()

	// Initialize Web Push (VAPID keys auto-generated on first run).
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = cfg.Storage.DataDir
	}
	if dataDir != "" {
		vapidPath := filepath.Join(dataDir, "vapid_keys.json")
		if vapidKeys, err := notifications.LoadOrGenerateVAPIDKeys(vapidPath, logger); err != nil {
			logger.Warn("Web Push disabled — could not init VAPID keys", zap.Error(err))
		} else {
			notifySvc.SetPush(*vapidKeys, authSvc.GetAllWebPushSubs, authSvc.RemovePushSubscriptionByEndpoint)
			logger.Info("Web Push initialized", zap.String("public_key", vapidKeys.Public[:16]+"..."))
		}
	}

	// Initialize firewall service (gracefully unavailable when ufw not installed)
	firewallSvc := firewall.NewService(logger)

	// TLS: set up autocert when AutoTLS is enabled, otherwise use static cert files.
	var autoTLSConfig *tls.Config
	if cfg.TLS.AutoTLS {
		if cfg.TLS.ACMEDomain == "" {
			return fmt.Errorf("auto_tls is enabled but acme_domain is not set in the config")
		}
		cacheDir := cfg.TLS.ACMECacheDir
		if cacheDir == "" {
			cacheDir = "/etc/games-dashboard/tls/acme"
		}
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			return fmt.Errorf("failed to create ACME cache dir %s: %w", cacheDir, err)
		}
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.TLS.ACMEDomain),
			Cache:      autocert.DirCache(cacheDir),
			Email:      cfg.TLS.ACMEEmail,
		}
		if cfg.TLS.ACMEStaging {
			// Override CA URL to Let's Encrypt staging for testing.
			m.Client = &acme.Client{DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory"}
		}
		// Start HTTP-01 challenge handler on port 80.
		go func() {
			logger.Info("Starting ACME HTTP-01 challenge server on :80")
			if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil { //nolint:gosec
				logger.Warn("ACME HTTP-01 server stopped", zap.Error(err))
			}
		}()
		autoTLSConfig = m.TLSConfig()
		logger.Info("AutoTLS enabled via Let's Encrypt", zap.String("domain", cfg.TLS.ACMEDomain))
	}

	// Initialize API server
	apiServer, err := api.NewServer(api.Config{
		BindAddr:        bindAddr,
		TLSCert:         cfg.TLS.CertFile,
		TLSKey:          cfg.TLS.KeyFile,
		AutoTLSConfig:   autoTLSConfig,
		Logger:          logger,
		AuthSvc:         authSvc,
		Broker:          gameBroker,
		HealthSvc:       healthSvc,
		MetricsSvc:      metricsSvc,
		FirewallSvc:     firewallSvc,
		NotificationSvc: notifySvc,
		DaemonCfg:       cfg,
		ConfigPath:      cfgFile,
	})
	if err != nil {
		return fmt.Errorf("failed to init API server: %w", err)
	}

	// Start services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gameBroker.Start(ctx)
	go healthSvc.Start(ctx)
	go gameBroker.BackupService().Start(ctx)
	if cm := gameBroker.ClusterManager(); cm != nil {
		go cm.Start(ctx)
		logger.Info("Cluster manager started")
	}

	// Start API server
	errCh := make(chan error, 1)
	go func() {
		logger.Info("API server listening", zap.String("addr", bindAddr), zap.Bool("tls", !noTLS))
		var serveErr error
		if noTLS {
			serveErr = apiServer.ListenAndServe()
		} else {
			serveErr = apiServer.ListenAndServeTLS()
		}
		if serveErr != nil {
			errCh <- serveErr
		}
	}()

	// Wait for signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received signal, shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		logger.Error("Server error", zap.Error(err))
		return err
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("Shutdown error", zap.Error(err))
	}

	logger.Info("Games Dashboard Daemon stopped")
	return nil
}

func initLogger(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Encoding = "json"
	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return cfg.Build()
}

func version() string {
	return "1.0.0"
}

// resolveJWTSecret loads the JWT secret from {dataDir}/jwt_secret.enc.
// The file contains the secret encrypted by the secrets manager (AES-GCM via
// the master key). On first boot the file does not exist: a fresh 64-byte
// (128 hex char) secret is generated via crypto/rand, encrypted, and persisted.
// The ciphertext file is created with mode 0600. Even if the file is exfiltrated,
// the plaintext secret is unrecoverable without the master key.
func resolveJWTSecret(dataDir string, mgr *secrets.Manager, logger *zap.Logger) (string, error) {
	secretFile := filepath.Join(dataDir, "jwt_secret.enc")

	// Try to load and decrypt an existing secret.
	if ciphertext, err := os.ReadFile(secretFile); err == nil {
		plaintext := strings.TrimSpace(string(ciphertext))
		if plaintext != "" {
			secret, decErr := mgr.Decrypt(plaintext)
			if decErr == nil && secret != "" {
				return secret, nil
			}
			// Decryption failed (e.g. master key rotated) — regenerate below.
			logger.Warn("JWT secret decryption failed, regenerating", zap.Error(decErr))
		}
	}

	// Generate a new 64-byte (128 hex char) random secret.
	raw := make([]byte, 64)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	secret := hex.EncodeToString(raw) // 128 hex characters

	// Encrypt and persist (0600 — owner read/write only).
	ciphertext, err := mgr.Encrypt(secret)
	if err != nil {
		return "", fmt.Errorf("could not encrypt JWT secret: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return "", fmt.Errorf("could not create data dir: %w", err)
	}
	if err := os.WriteFile(secretFile, []byte(ciphertext+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("could not write jwt_secret.enc: %w", err)
	}

	logger.Warn("JWT secret auto-generated and saved (encrypted)",
		zap.String("path", secretFile),
		zap.String("note", "set auth.jwt_secret in daemon.yaml to use a fixed secret instead"),
	)
	return secret, nil
}

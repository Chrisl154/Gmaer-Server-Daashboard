package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/games-dashboard/daemon/internal/api"
	"github.com/games-dashboard/daemon/internal/auth"
	"github.com/games-dashboard/daemon/internal/broker"
	"github.com/games-dashboard/daemon/internal/config"
	"github.com/games-dashboard/daemon/internal/health"
	"github.com/games-dashboard/daemon/internal/metrics"
	"github.com/games-dashboard/daemon/internal/secrets"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	cfgFile  string
	logLevel string
	tlsCert  string
	tlsKey   string
	bindAddr string
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
	}
	if cfg.Auth.OIDC != nil {
		authCfg.OIDC = &auth.OIDCConfig{
			Issuer:       cfg.Auth.OIDC.Issuer,
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RedirectURL:  cfg.Auth.OIDC.RedirectURL,
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

	// Initialize API server
	apiServer, err := api.NewServer(api.Config{
		BindAddr:   bindAddr,
		TLSCert:    cfg.TLS.CertFile,
		TLSKey:     cfg.TLS.KeyFile,
		Logger:     logger,
		AuthSvc:    authSvc,
		Broker:     gameBroker,
		HealthSvc:  healthSvc,
		MetricsSvc: metricsSvc,
		DaemonCfg:  cfg,
		ConfigPath: cfgFile,
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
		logger.Info("API server listening", zap.String("addr", bindAddr))
		if err := apiServer.ListenAndServeTLS(); err != nil {
			errCh <- err
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

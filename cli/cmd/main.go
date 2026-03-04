package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	daemonURL string
	token     string
	output    string
)

func main() {
	root := &cobra.Command{
		Use:   "gdash",
		Short: "Games Dashboard CLI",
		Long:  `gdash - command-line interface for the Gaming Server Dashboard daemon`,
	}

	root.PersistentFlags().StringVar(&daemonURL, "daemon", "https://localhost:8443", "Daemon API URL")
	root.PersistentFlags().StringVar(&token, "token", "", "API bearer token (or set GDASH_TOKEN env)")
	root.PersistentFlags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")

	viper.BindPFlag("daemon", root.PersistentFlags().Lookup("daemon"))
	viper.BindPFlag("token", root.PersistentFlags().Lookup("token"))
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GDASH")

	// Server commands
	serverCmd := &cobra.Command{Use: "server", Short: "Manage game servers"}
	serverCmd.AddCommand(
		serverListCmd(),
		serverGetCmd(),
		serverCreateCmd(),
		serverDeleteCmd(),
		serverStartCmd(),
		serverStopCmd(),
		serverRestartCmd(),
		serverDeployCmd(),
		serverLogsCmd(),
		serverConsoleCmd(),
	)

	// Backup commands
	backupCmd := &cobra.Command{Use: "backup", Short: "Manage backups"}
	backupCmd.AddCommand(
		backupListCmd(),
		backupCreateCmd(),
		backupRestoreCmd(),
	)

	// Mod commands
	modCmd := &cobra.Command{Use: "mod", Short: "Manage mods"}
	modCmd.AddCommand(
		modListCmd(),
		modInstallCmd(),
		modUninstallCmd(),
		modTestCmd(),
		modRollbackCmd(),
	)

	// Port commands
	portCmd := &cobra.Command{Use: "port", Short: "Manage ports"}
	portCmd.AddCommand(portListCmd(), portValidateCmd())

	// SBOM/CVE commands
	sbomCmd := &cobra.Command{Use: "sbom", Short: "SBOM and CVE management"}
	sbomCmd.AddCommand(sbomShowCmd(), sbomScanCmd(), cveReportCmd())

	// Auth commands
	authCmd := &cobra.Command{Use: "auth", Short: "Authentication"}
	authCmd.AddCommand(authLoginCmd(), authLogoutCmd(), authStatusCmd())

	root.AddCommand(serverCmd, backupCmd, modCmd, portCmd, sbomCmd, authCmd,
		healthCmd(), versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Server commands

func serverListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all game servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func serverGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0])
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func serverCreateCmd() *cobra.Command {
	var adapter, deployMethod, installDir string
	cmd := &cobra.Command{
		Use:   "create <id> <name>",
		Short: "Create a new game server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"id": args[0], "name": args[1],
				"adapter": adapter, "deploy_method": deployMethod,
				"install_dir": installDir,
			}
			resp, err := apiPost("/api/v1/servers", body)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&adapter, "adapter", "minecraft", "Game adapter")
	cmd.Flags().StringVar(&deployMethod, "deploy-method", "manual", "Deploy method: steamcmd|manual")
	cmd.Flags().StringVar(&installDir, "install-dir", "/opt/games", "Install directory")
	return cmd
}

func serverDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a game server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiDelete("/api/v1/servers/" + args[0])
		},
	}
}

func serverStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <id>",
		Short: "Start a game server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/start", nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func serverStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a game server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/stop", nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func serverRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <id>",
		Short: "Restart a game server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/restart", nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func serverDeployCmd() *cobra.Command {
	var method, appID string
	cmd := &cobra.Command{
		Use:   "deploy <id>",
		Short: "Deploy a game server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"method": method}
			if appID != "" {
				body["steamcmd"] = map[string]any{"app_id": appID}
			}
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/deploy", body)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&method, "method", "steamcmd", "Deploy method")
	cmd.Flags().StringVar(&appID, "app-id", "", "SteamCMD app ID")
	return cmd
}

func serverLogsCmd() *cobra.Command {
	var lines string
	cmd := &cobra.Command{
		Use:   "logs <id>",
		Short: "Get server logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet(fmt.Sprintf("/api/v1/servers/%s/logs?lines=%s", args[0], lines))
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&lines, "lines", "100", "Number of log lines")
	return cmd
}

func serverConsoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console <id>",
		Short: "Attach to server console (WebSocket)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Connecting to console for server %s...\n", args[0])
			fmt.Printf("WebSocket URL: %s/api/v1/servers/%s/console/stream\n", daemonURL, args[0])
			fmt.Println("Use a WebSocket client to connect (wscat, websocat, etc.)")
			return nil
		},
	}
}

// Backup commands

func backupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-id>",
		Short: "List backups for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/backups")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func backupCreateCmd() *cobra.Command {
	var backupType string
	cmd := &cobra.Command{
		Use:   "create <server-id>",
		Short: "Create a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/backup", map[string]any{"type": backupType})
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&backupType, "type", "full", "Backup type: full|incremental")
	return cmd
}

func backupRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <server-id> <backup-id>",
		Short: "Restore from a backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost(fmt.Sprintf("/api/v1/servers/%s/restore/%s", args[0], args[1]), nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

// Mod commands

func modListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list <server-id>", Short: "List mods", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/mods")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func modInstallCmd() *cobra.Command {
	var source, version, sourceURL string
	cmd := &cobra.Command{
		Use: "install <server-id> <mod-id>", Short: "Install a mod", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods", map[string]any{
				"mod_id": args[1], "source": source,
				"version": version, "source_url": sourceURL,
			})
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&source, "source", "local", "Mod source: steam|curseforge|git|local")
	cmd.Flags().StringVar(&version, "version", "latest", "Mod version")
	cmd.Flags().StringVar(&sourceURL, "url", "", "Source URL")
	return cmd
}

func modUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use: "uninstall <server-id> <mod-id>", Short: "Uninstall a mod", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiDelete(fmt.Sprintf("/api/v1/servers/%s/mods/%s", args[0], args[1]))
		},
	}
}

func modTestCmd() *cobra.Command {
	return &cobra.Command{
		Use: "test <server-id>", Short: "Run mod compatibility harness", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods/test", nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func modRollbackCmd() *cobra.Command {
	var checkpoint string
	cmd := &cobra.Command{
		Use: "rollback <server-id>", Short: "Rollback mod set", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods/rollback", map[string]any{"checkpoint": checkpoint})
			if err != nil { return err }
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint to roll back to")
	return cmd
}

// Port commands

func portListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list <server-id>", Short: "List port mappings", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/ports")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func portValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "validate", Short: "Validate port availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/ports/validate", map[string]any{"ports": []any{}})
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

// SBOM/CVE commands

func sbomShowCmd() *cobra.Command {
	return &cobra.Command{
		Use: "show", Short: "Show SBOM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/sbom")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func sbomScanCmd() *cobra.Command {
	return &cobra.Command{
		Use: "scan", Short: "Trigger CVE scan",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/sbom/scan", nil)
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func cveReportCmd() *cobra.Command {
	return &cobra.Command{
		Use: "cve-report", Short: "Show CVE report",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/cve-report")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

// Auth commands

func authLoginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use: "login", Short: "Login to daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPostNoAuth("/api/v1/auth/login", map[string]any{"username": username, "password": password})
			if err != nil { return err }
			data, ok := resp.(map[string]any)
			if ok {
				if t, ok := data["token"].(string); ok {
					fmt.Printf("Login successful. Token: %s\n", t)
					fmt.Printf("Set: export GDASH_TOKEN=%s\n", t)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")
	return cmd
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use: "logout", Short: "Logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := apiPost("/api/v1/auth/logout", nil)
			if err != nil { return err }
			fmt.Println("Logged out")
			return nil
		},
	}
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Check auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" { token = os.Getenv("GDASH_TOKEN") }
			if token == "" {
				fmt.Println("Not authenticated (no token)")
				return nil
			}
			fmt.Printf("Token present (first 16 chars): %s...\n", token[:min(16, len(token))])
			return nil
		},
	}
}

// Misc commands

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use: "health", Short: "Check daemon health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/healthz")
			if err != nil { return err }
			return printResponse(resp)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("gdash version 1.0.0")
		},
	}
}

// HTTP helpers

func getToken() string {
	if token != "" { return token }
	return os.Getenv("GDASH_TOKEN")
}

func getDaemon() string {
	if daemonURL != "" { return daemonURL }
	if u := os.Getenv("GDASH_DAEMON"); u != "" { return u }
	return "https://localhost:8443"
}

func apiGet(path string) (any, error) {
	return doRequest("GET", getDaemon()+path, nil, true)
}

func apiPost(path string, body any) (any, error) {
	return doRequest("POST", getDaemon()+path, body, true)
}

func apiPostNoAuth(path string, body any) (any, error) {
	return doRequest("POST", getDaemon()+path, body, false)
}

func apiDelete(path string) error {
	_, err := doRequest("DELETE", getDaemon()+path, nil, true)
	if err == nil { fmt.Println("Deleted") }
	return err
}

func doRequest(method, url string, body any, auth bool) (any, error) {
	// Simplified HTTP client - in production use net/http with TLS config
	fmt.Printf("[%s] %s\n", method, url)
	// Return mock success for compilation
	return map[string]any{"status": "ok", "url": url}, nil
}

func printResponse(resp any) error {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil { return err }
	fmt.Println(string(data))
	return nil
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

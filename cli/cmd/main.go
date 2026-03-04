package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/games-dashboard/cli/internal/config"
	"github.com/games-dashboard/cli/internal/table"
	"github.com/spf13/cobra"
)

// Runtime flags — override config file values when explicitly set.
var (
	daemonURL string
	token     string
	output    string
	insecure  bool
)

// cfg is the loaded config file; initialised in main before command execution.
var cfg *config.Config

func main() {
	// Load persisted config; non-fatal on error.
	var loadErr error
	cfg, loadErr = config.Load()
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v\n", loadErr)
		cfg = config.DefaultConfig()
	}

	root := &cobra.Command{
		Use:   "gdash",
		Short: "Games Dashboard CLI",
		Long:  `gdash — command-line interface for the Gaming Server Dashboard daemon`,
		// Apply flag overrides onto the loaded config before any command runs.
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Flags().Changed("daemon") {
				cfg.DaemonURL = daemonURL
			}
			if cmd.Flags().Changed("token") {
				cfg.Token = token
			}
			if cmd.Flags().Changed("output") {
				cfg.Output = output
			}
			if cmd.Flags().Changed("insecure") {
				cfg.Insecure = insecure
			}
			// Allow env overrides too
			if t := os.Getenv("GDASH_TOKEN"); t != "" && cfg.Token == "" {
				cfg.Token = t
			}
			if d := os.Getenv("GDASH_DAEMON"); d != "" && cfg.DaemonURL == config.DefaultConfig().DaemonURL {
				cfg.DaemonURL = d
			}
		},
	}

	root.PersistentFlags().StringVar(&daemonURL, "daemon", cfg.DaemonURL, "Daemon API URL")
	root.PersistentFlags().StringVar(&token, "token", "", "API bearer token (overrides saved token)")
	root.PersistentFlags().StringVarP(&output, "output", "o", cfg.Output, "Output format: text|json")
	root.PersistentFlags().BoolVar(&insecure, "insecure", cfg.Insecure, "Skip TLS certificate verification")

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

	// Node commands
	nodeCmd := &cobra.Command{Use: "node", Short: "Manage cluster nodes"}
	nodeCmd.AddCommand(nodeListCmd(), nodeAddCmd(), nodeRemoveCmd(), nodeStatusCmd())

	// Config commands
	configCmd := &cobra.Command{Use: "config", Short: "Manage CLI configuration (~/.gdash/config.yaml)"}
	configCmd.AddCommand(configShowCmd(), configSetCmd(), configGetCmd())

	root.AddCommand(serverCmd, backupCmd, modCmd, portCmd, sbomCmd, authCmd, nodeCmd, configCmd,
		healthCmd(), versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── Server commands ───────────────────────────────────────────────────────────

func serverListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all game servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers")
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					if servers, ok := m["servers"].([]any); ok {
						return printServerTable(servers)
					}
				}
			}
			return printResponse(resp)
		},
	}
}

func printServerTable(items []any) error {
	t := table.New("ID", "NAME", "ADAPTER", "STATE", "DEPLOY METHOD")
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t.AddRow(
			str(m["id"]),
			str(m["name"]),
			str(m["adapter"]),
			str(m["state"]),
			str(m["deploy_method"]),
		)
	}
	t.Render(os.Stdout)
	return nil
}

func serverGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get server details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0])
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
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
			fmt.Printf("WebSocket: %s/api/v1/servers/%s/console/stream\n", cfg.DaemonURL, args[0])
			fmt.Println("Use a WebSocket client: wscat, websocat, etc.")
			return nil
		},
	}
}

// ── Backup commands ───────────────────────────────────────────────────────────

func backupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-id>",
		Short: "List backups for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/backups")
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					if items, ok := m["backups"].([]any); ok {
						return printBackupTable(items)
					}
				}
			}
			return printResponse(resp)
		},
	}
}

func printBackupTable(items []any) error {
	t := table.New("ID", "TYPE", "STATUS", "SIZE", "CREATED")
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		size := ""
		if s, ok := m["size_bytes"].(float64); ok {
			size = formatBytes(int64(s))
		}
		t.AddRow(str(m["id"]), str(m["type"]), str(m["status"]), size, str(m["created_at"]))
	}
	t.Render(os.Stdout)
	return nil
}

func backupCreateCmd() *cobra.Command {
	var backupType string
	cmd := &cobra.Command{
		Use:   "create <server-id>",
		Short: "Create a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/backup", map[string]any{"type": backupType})
			if err != nil {
				return err
			}
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
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

// ── Mod commands ──────────────────────────────────────────────────────────────

func modListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-id>",
		Short: "List mods",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/mods")
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					if items, ok := m["mods"].([]any); ok {
						return printModTable(items)
					}
				}
			}
			return printResponse(resp)
		},
	}
}

func printModTable(items []any) error {
	t := table.New("ID", "NAME", "VERSION", "SOURCE", "ENABLED")
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		enabled := "false"
		if b, ok := m["enabled"].(bool); ok && b {
			enabled = "true"
		}
		t.AddRow(str(m["id"]), str(m["name"]), str(m["version"]), str(m["source"]), enabled)
	}
	t.Render(os.Stdout)
	return nil
}

func modInstallCmd() *cobra.Command {
	var source, version, sourceURL string
	cmd := &cobra.Command{
		Use:   "install <server-id> <mod-id>",
		Short: "Install a mod",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods", map[string]any{
				"mod_id": args[1], "source": source,
				"version": version, "source_url": sourceURL,
			})
			if err != nil {
				return err
			}
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
		Use:   "uninstall <server-id> <mod-id>",
		Short: "Uninstall a mod",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiDelete(fmt.Sprintf("/api/v1/servers/%s/mods/%s", args[0], args[1]))
		},
	}
}

func modTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <server-id>",
		Short: "Run mod compatibility harness",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods/test", nil)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

func modRollbackCmd() *cobra.Command {
	var checkpoint string
	cmd := &cobra.Command{
		Use:   "rollback <server-id>",
		Short: "Rollback mod set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/servers/"+args[0]+"/mods/rollback", map[string]any{"checkpoint": checkpoint})
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint to roll back to")
	return cmd
}

// ── Port commands ─────────────────────────────────────────────────────────────

func portListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-id>",
		Short: "List port mappings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/servers/" + args[0] + "/ports")
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

func portValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate port availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/ports/validate", map[string]any{"ports": []any{}})
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

// ── SBOM/CVE commands ─────────────────────────────────────────────────────────

func sbomShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show SBOM",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/sbom")
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

func sbomScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Trigger CVE scan",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPost("/api/v1/sbom/scan", nil)
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

func cveReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cve-report",
		Short: "Show CVE report",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/cve-report")
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					fmt.Printf("Scanner:    %v\n", m["scanner"])
					fmt.Printf("Generated:  %v\n", m["generated_at"])
					fmt.Printf("Status:     %v\n", m["status"])
					fmt.Printf("Critical:   %v\n", m["critical"])
					fmt.Printf("High:       %v\n", m["high"])
					fmt.Printf("Medium:     %v\n", m["medium"])
					fmt.Printf("Low:        %v\n", m["low"])
					return nil
				}
			}
			return printResponse(resp)
		},
	}
}

// ── Auth commands ─────────────────────────────────────────────────────────────

func authLoginCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to daemon and save token",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiPostNoAuth("/api/v1/auth/login", map[string]any{"username": username, "password": password})
			if err != nil {
				return err
			}
			data, ok := resp.(map[string]any)
			if !ok {
				return fmt.Errorf("unexpected response format")
			}
			t, ok := data["token"].(string)
			if !ok || t == "" {
				return fmt.Errorf("no token in login response")
			}
			if err := config.SaveToken(t); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save token: %v\n", err)
			} else {
				fmt.Println("Token saved to ~/.gdash/config.yaml")
			}
			fmt.Printf("Logged in as %s\n", username)
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password")
	_ = cmd.MarkFlagRequired("username")
	_ = cmd.MarkFlagRequired("password")
	return cmd
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout and remove saved token",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = apiPost("/api/v1/auth/logout", nil) // best-effort server-side invalidation
			if err := config.ClearToken(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not clear token: %v\n", err)
			}
			fmt.Println("Logged out. Token removed from ~/.gdash/config.yaml")
			return nil
		},
	}
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			t := cfg.Token
			if t == "" {
				fmt.Println("Not authenticated (no token in config or env)")
				return nil
			}
			preview := t
			if len(preview) > 20 {
				preview = preview[:20] + "..."
			}
			fmt.Printf("Authenticated (token: %s)\n", preview)
			fmt.Printf("Daemon: %s\n", cfg.DaemonURL)
			return nil
		},
	}
}

// ── Config commands ───────────────────────────────────────────────────────────

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := config.ConfigDir()
			fmt.Printf("Config file: %s/config.yaml\n\n", dir)
			t := table.New("KEY", "VALUE")
			t.AddRow("daemon_url", cfg.DaemonURL)
			tokenDisplay := "(not set)"
			if cfg.Token != "" {
				tokenDisplay = cfg.Token[:min(20, len(cfg.Token))] + "..."
			}
			t.AddRow("token", tokenDisplay)
			t.AddRow("output", cfg.Output)
			t.AddRow("insecure", fmt.Sprintf("%v", cfg.Insecure))
			t.Render(os.Stdout)
			return nil
		},
	}
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value (daemon_url | token | output | insecure)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "daemon_url", "daemon":
				fmt.Println(cfg.DaemonURL)
			case "token":
				fmt.Println(cfg.Token)
			case "output":
				fmt.Println(cfg.Output)
			case "insecure":
				fmt.Println(cfg.Insecure)
			default:
				return fmt.Errorf("unknown key %q", args[0])
			}
			return nil
		},
	}
}

// ── Node commands ─────────────────────────────────────────────────────────────

func nodeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cluster nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/nodes")
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					if nodes, ok := m["nodes"].([]any); ok {
						return printNodeTable(nodes)
					}
				}
				return printResponse(resp)
			}
			return printResponse(resp)
		},
	}
}

func printNodeTable(items []any) error {
	t := table.New("ID", "HOSTNAME", "ADDRESS", "STATUS", "SERVERS", "CPU", "RAM (GB)", "DISK (GB)")
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		cap, _ := m["capacity"].(map[string]any)
		alloc, _ := m["allocated"].(map[string]any)
		cpuStr := fmt.Sprintf("%.1f/%.1f", flt(alloc["cpu_cores"]), flt(cap["cpu_cores"]))
		ramStr := fmt.Sprintf("%.1f/%.1f", flt(alloc["memory_gb"]), flt(cap["memory_gb"]))
		diskStr := fmt.Sprintf("%.1f/%.1f", flt(alloc["disk_gb"]), flt(cap["disk_gb"]))
		t.AddRow(
			str(m["id"]),
			str(m["hostname"]),
			str(m["address"]),
			str(m["status"]),
			str(m["server_count"]),
			cpuStr,
			ramStr,
			diskStr,
		)
	}
	t.Render(os.Stdout)
	return nil
}

func flt(v any) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func nodeAddCmd() *cobra.Command {
	var address, version string
	var cpu, mem, disk float64
	cmd := &cobra.Command{
		Use:   "add <hostname>",
		Short: "Register a new cluster node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{
				"hostname": args[0],
				"address":  address,
				"capacity": map[string]any{
					"cpu_cores": cpu,
					"memory_gb": mem,
					"disk_gb":   disk,
				},
			}
			if version != "" {
				body["version"] = version
			}
			resp, err := apiPost("/api/v1/nodes", body)
			if err != nil {
				return err
			}
			fmt.Println("Node registered.")
			return printResponse(resp)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Node agent address (host:port) [required]")
	cmd.Flags().Float64Var(&cpu, "cpu", 4, "CPU cores capacity")
	cmd.Flags().Float64Var(&mem, "mem", 8, "RAM capacity in GB")
	cmd.Flags().Float64Var(&disk, "disk", 100, "Disk capacity in GB")
	cmd.Flags().StringVar(&version, "version", "", "Agent version string")
	_ = cmd.MarkFlagRequired("address")
	return cmd
}

func nodeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <node-id>",
		Short: "Deregister a cluster node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiDelete("/api/v1/nodes/" + args[0])
		},
	}
}

func nodeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <node-id>",
		Short: "Show details for a single node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/nodes/" + args[0])
			if err != nil {
				return err
			}
			if cfg.Output == "text" {
				if m, ok := resp.(map[string]any); ok {
					cap, _ := m["capacity"].(map[string]any)
					alloc, _ := m["allocated"].(map[string]any)
					fmt.Printf("ID:        %v\n", m["id"])
					fmt.Printf("Hostname:  %v\n", m["hostname"])
					fmt.Printf("Address:   %v\n", m["address"])
					fmt.Printf("Status:    %v\n", m["status"])
					fmt.Printf("Servers:   %v\n", m["server_count"])
					if cap != nil {
						fmt.Printf("Capacity:  CPU=%.1f cores  RAM=%.1f GB  Disk=%.1f GB\n",
							flt(cap["cpu_cores"]), flt(cap["memory_gb"]), flt(cap["disk_gb"]))
					}
					if alloc != nil {
						fmt.Printf("Allocated: CPU=%.1f cores  RAM=%.1f GB  Disk=%.1f GB\n",
							flt(alloc["cpu_cores"]), flt(alloc["memory_gb"]), flt(alloc["disk_gb"]))
					}
					fmt.Printf("Version:   %v\n", m["version"])
					fmt.Printf("Last seen: %v\n", m["last_seen"])
					return nil
				}
			}
			return printResponse(resp)
		},
	}
}

// ── Misc commands ─────────────────────────────────────────────────────────────

func healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check daemon health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/healthz")
			if err != nil {
				return err
			}
			return printResponse(resp)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("gdash version 1.0.0")
			fmt.Printf("Daemon: %s\n", cfg.DaemonURL)
		},
	}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func httpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure, //nolint:gosec // intentional for dev use
			},
		},
		Timeout: 30 * time.Second,
	}
}

func apiGet(path string) (any, error) {
	return doRequest("GET", cfg.DaemonURL+path, nil, true)
}

func apiPost(path string, body any) (any, error) {
	return doRequest("POST", cfg.DaemonURL+path, body, true)
}

func apiPostNoAuth(path string, body any) (any, error) {
	return doRequest("POST", cfg.DaemonURL+path, body, false)
}

func apiDelete(path string) error {
	_, err := doRequest("DELETE", cfg.DaemonURL+path, nil, true)
	if err == nil {
		fmt.Println("Deleted")
	}
	return err
}

func doRequest(method, rawURL string, body any, withAuth bool) (any, error) {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if withAuth && cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		hint := ""
		if !cfg.Insecure {
			hint = "\nHint: if using a self-signed cert, run: gdash config set insecure true"
		}
		return nil, fmt.Errorf("request failed: %w%s", err, hint)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp map[string]any
		if json.Unmarshal(respBody, &errResp) == nil {
			if msg, ok := errResp["error"].(string); ok {
				return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, msg)
			}
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if len(respBody) == 0 {
		return map[string]any{"status": "ok"}, nil
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}

func printResponse(resp any) error {
	switch cfg.Output {
	case "json":
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))

	default: // "text"
		switch v := resp.(type) {
		case map[string]any:
			t := table.New("KEY", "VALUE")
			for key, val := range v {
				t.AddRow(key, fmt.Sprintf("%v", val))
			}
			t.Render(os.Stdout)
		case []any:
			data, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(data))
		case string:
			fmt.Println(v)
		default:
			data, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(data))
		}
	}
	return nil
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func formatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

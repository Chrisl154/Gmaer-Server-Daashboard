// ── Server types ─────────────────────────────────────────────────────────────

export type ServerState =
  | 'idle'
  | 'running'
  | 'stopped'
  | 'starting'
  | 'stopping'
  | 'deploying'
  | 'error';

export interface PortMapping {
  internal: number;
  external: number;
  protocol: 'tcp' | 'udp';
  description?: string;
  exposed?: boolean;
}

export interface ResourceSpec {
  cpu_cores: number;
  ram_gb: number;
  disk_gb: number;
}

export interface Server {
  id: string;
  name: string;
  adapter: string;
  deploy_method: string;
  install_dir: string;
  state: ServerState;
  ports: PortMapping[];
  resources: ResourceSpec;
  config: Record<string, unknown>;
  last_started?: string;
  last_stopped?: string;
  created_at: string;
  // Live metrics — updated every 15 s by the broker
  cpu_pct?: number;
  ram_pct?: number;
  disk_pct?: number;
  player_count?: number; // -1 = not available for this game
  max_players?: number;  // 0 = unknown
}

export interface CreateServerRequest {
  id: string;
  name: string;
  adapter: string;
  deploy_method?: string;
  install_dir?: string;
  ports?: PortMapping[];
  resources?: ResourceSpec;
  config?: Record<string, unknown>;
  node_id?: string;
}

export interface UpdateServerRequest {
  name?: string;
  resources?: ResourceSpec;
  config?: Record<string, unknown>;
}

export interface ServersResponse {
  servers: Server[];
  count: number;
}

// ── Backup types ─────────────────────────────────────────────────────────────

export type BackupStatus = 'pending' | 'running' | 'complete' | 'failed';

export interface Backup {
  id: string;
  server_id: string;
  type: 'full' | 'incremental';
  target: string;
  size_bytes: number;
  checksum: string;
  paths: string[];
  created_at: string;
  status: BackupStatus;
  error?: string;
}

export interface BackupJob {
  id: string;
  server_id: string;
  type: 'backup' | 'restore';
  status: string;
  progress: number;
  message: string;
  created_at: string;
  updated_at: string;
}

export interface BackupsResponse {
  backups: Backup[];
  count: number;
}

// ── Mod types ─────────────────────────────────────────────────────────────────

export type ModSource = 'steam' | 'curseforge' | 'git' | 'local' | 'modrinth' | 'thunderstore';

export interface Mod {
  id: string;
  name: string;
  version: string;
  source: ModSource;
  source_url?: string;
  checksum: string;
  installed_at: string;
  enabled: boolean;
  size_bytes?: number;
}

export interface ModJob {
  id: string;
  server_id: string;
  type: string;
  mod_id?: string;
  status: string;
  progress: number;
  message: string;
  created_at: string;
  updated_at: string;
}

export interface ModsResponse {
  mods: Mod[];
  count: number;
}

export interface InstallModRequest {
  source: ModSource;
  mod_id: string;
  version?: string;
  source_url?: string;
}

export interface ModTestResult {
  name: string;
  passed: boolean;
  message: string;
  duration_ms: number;
}

export interface ModTestSuiteResult {
  passed: boolean;
  tests: ModTestResult[];
  duration_ms: number;
}

// ── Port types ─────────────────────────────────────────────────────────────────

export interface PortValidation {
  internal: number;
  external: number;
  protocol: string;
  available: boolean;
  reachable: boolean;
  conflict?: string;
  latency_ms?: number;
}

export interface PortValidateResponse {
  results: PortValidation[];
}

// ── SBOM / CVE types ──────────────────────────────────────────────────────────

export interface CVEFinding {
  id: string;
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
  package: string;
  version: string;
  fixed_in?: string;
  description: string;
  cvss?: number;
  link?: string;
  scanned_at: string;
}

export interface CVEReport {
  generated_at: string;
  scanner: string;
  status: 'clean' | 'findings' | 'not_scanned';
  total_count: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  findings: CVEFinding[];
  evidence: {
    last_checked: string;
    authoritative_link: string;
    cve_status: string;
  };
}

// ── Auth types ────────────────────────────────────────────────────────────────

export interface User {
  id: string;
  username: string;
  roles: string[];
  totp_enabled: boolean;
  created_at: string;
  last_login?: string;
}

export interface AuditEntry {
  id: string;
  user_id: string;
  username: string;
  action: string;
  resource: string;
  ip: string;
  timestamp: string;
  success: boolean;
  details?: unknown;
}

// ── System types ──────────────────────────────────────────────────────────────

export interface HealthComponent {
  healthy: boolean;
  message?: string;
}

export interface SystemStatus {
  healthy: boolean;
  timestamp: string;
  version: string;
  uptime_seconds: number;
  start_time: string;
  components: Record<string, HealthComponent>;
}

// ── Cluster / Node types ──────────────────────────────────────────────────────

export type NodeStatus = 'online' | 'offline' | 'draining';

export interface NodeCapacity {
  cpu_cores: number;
  memory_gb: number;
  disk_gb: number;
}

export interface Node {
  id: string;
  hostname: string;
  address: string;
  labels?: Record<string, string>;
  capacity: NodeCapacity;
  allocated: NodeCapacity;
  server_count: number;
  status: NodeStatus;
  version?: string;
  registered_at: string;
  last_seen: string;
}

export interface NodesResponse {
  nodes: Node[];
}

export interface RegisterNodeRequest {
  hostname: string;
  address: string;
  labels?: Record<string, string>;
  capacity: NodeCapacity;
  version?: string;
}

export interface HeartbeatRequest {
  allocated: NodeCapacity;
  server_count: number;
  status: NodeStatus;
}

// ── Daemon settings ───────────────────────────────────────────────────────────

export interface DaemonSettingsNFSMount {
  server: string;
  path: string;
  mount_point: string;
  options?: string;
}

export interface DaemonSettingsS3 {
  endpoint: string;
  bucket: string;
  region: string;
  use_ssl: boolean;
}

export interface DaemonSettingsTailscale {
  enabled: boolean;
  hostname: string;
  dual: boolean;
  has_auth_key: boolean;
}

export interface DaemonSettings {
  bind_addr: string;
  shutdown_timeout_s: number;
  data_dir: string;
  log_level: string;
  storage: {
    data_dir: string;
    nfs_mounts: DaemonSettingsNFSMount[];
    s3?: DaemonSettingsS3;
  };
  backup: {
    default_schedule: string;
    retain_days: number;
    compression: string;
  };
  metrics: {
    enabled: boolean;
    path: string;
  };
  cluster: {
    enabled: boolean;
    health_check_interval_s: number;
    node_timeout_s: number;
  };
  tailscale: DaemonSettingsTailscale;
}

export interface SettingsPatch {
  log_level?: string;
  backup?: {
    default_schedule?: string;
    retain_days?: number;
    compression?: string;
  };
  metrics?: {
    enabled?: boolean;
    path?: string;
  };
  cluster?: {
    health_check_interval_s?: number;
    node_timeout_s?: number;
  };
  tailscale?: {
    enabled?: boolean;
    auth_key?: string;
    hostname?: string;
    dual?: boolean;
  };
}

export interface NotificationSettings {
  webhook_url: string;
  webhook_format: 'discord' | 'slack' | 'generic';
  events: string[];
}

export interface NotificationSettingsPatch {
  webhook_url?: string;
  webhook_format?: string;
  events?: string[];
}

// ── API helpers ───────────────────────────────────────────────────────────────

export interface APIError {
  error: string;
  code?: string;
}

export interface JobResponse {
  job_id: string;
  status: string;
  message?: string;
}

// ── Metrics ──────────────────────────────────────────────────────────────────

export interface ServerMetricSample {
  ts: number;          // Unix seconds
  cpu_pct: number;
  ram_pct: number;
  disk_pct: number;
  player_count: number; // -1 = not available
  net_in_kbps?: number;
  net_out_kbps?: number;
}

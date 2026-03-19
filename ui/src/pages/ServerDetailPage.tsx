import React, { useEffect, useRef, useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, Play, Square, RotateCcw, HardDrive, Package,
  Download, Maximize2, Cpu, MemoryStick, Clock, Activity,
  FileText, RefreshCw, Save, FolderOpen, Share2, Copy, Check, X,
  Wifi, WifiOff, Upload, Trash2, Folder, File, ChevronRight, Users,
  ArrowDownToLine, Loader2, QrCode, AlertCircle, Stethoscope, CheckCircle, AlertTriangle,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api, getWsUrl } from '../utils/api';
import { cn } from '../utils/cn';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import type { ServerMetricSample } from '../types';

type Tab = 'overview' | 'console' | 'logs' | 'backups' | 'mods' | 'ports' | 'config' | 'files' | 'players' | 'schedule';

const STATE_CLASS: Record<string, string> = {
  running:   'status-running',
  stopped:   'status-stopped',
  starting:  'status-starting',
  stopping:  'status-stopping',
  error:     'status-error',
  deploying: 'status-deploying',
  idle:      'status-idle',
};

export function ServerDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [showShare, setShowShare] = useState(false);
  const [showClone, setShowClone] = useState(false);
  const queryClient = useQueryClient();

  const { data: server, isLoading } = useQuery({
    queryKey: ['server', id],
    queryFn: () => api.get(`/api/v1/servers/${id}`).then(r => r.data),
    refetchInterval: 10_000,
  });

  const startMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/start`),
    onSuccess: () => { toast.success('Server starting...'); queryClient.invalidateQueries({ queryKey: ['server', id] }); },
  });
  const stopMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/stop`),
    onSuccess: () => { toast.success('Server stopping...'); queryClient.invalidateQueries({ queryKey: ['server', id] }); },
  });
  const restartMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/restart`),
    onSuccess: () => { toast.success('Server restarting...'); queryClient.invalidateQueries({ queryKey: ['server', id] }); },
  });

  if (isLoading) {
    return (
      <div className="p-6 md:p-8">
        <div className="card h-64 animate-pulse" />
      </div>
    );
  }
  if (!server) {
    return (
      <div className="p-6 md:p-8">
        <Link to="/" className="flex items-center gap-2 text-sm mb-4 transition-colors"
          style={{ color: 'var(--text-secondary)' }}
          onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
          onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-secondary)')}>
          <ArrowLeft className="w-4 h-4" /> Back
        </Link>
        <p style={{ color: 'var(--text-secondary)' }}>Server not found.</p>
      </div>
    );
  }

  const isRunning = server.state === 'running';
  const isBusy = ['starting', 'stopping', 'deploying'].includes(server.state);

  const TABS: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview'  },
    { id: 'console',  label: 'Console'   },
    { id: 'logs',     label: 'Logs'      },
    { id: 'backups',  label: 'Backups'   },
    { id: 'mods',     label: 'Mods'      },
    { id: 'ports',    label: 'Ports'     },
    { id: 'config',   label: 'Config'    },
    { id: 'schedule', label: 'Schedule'  },
    { id: 'files',    label: 'Files'     },
    { id: 'players',  label: 'Players'   },
  ];

  return (
    <div className="p-6 md:p-8 animate-page space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm">
        <Link to="/" className="transition-colors" style={{ color: 'var(--text-secondary)' }}
          onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
          onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-secondary)')}>
          <ArrowLeft className="w-4 h-4 inline-block mr-1" />
          Dashboard
        </Link>
        <span style={{ color: 'var(--text-muted)' }}>/</span>
        <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{server.name}</span>
      </div>

      {/* Server header */}
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl flex items-center justify-center shrink-0"
            style={{ background: 'var(--primary-subtle)', border: '1px solid var(--primary-border)' }}>
            <Activity className="w-6 h-6" style={{ color: 'var(--primary)' }} />
          </div>
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>{server.name}</h1>
              <span className={cn('badge capitalize', STATE_CLASS[server.state] ?? 'status-stopped')}>
                {server.state}
              </span>
            </div>
            <p className="text-sm mt-0.5 capitalize" style={{ color: 'var(--text-secondary)' }}>
              {server.adapter}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {isRunning ? (
            <>
              <button
                onClick={() => stopMutation.mutate()}
                disabled={isBusy}
                className="btn-danger py-2"
              >
                <Square className="w-4 h-4" /> Stop
              </button>
              <button
                onClick={() => restartMutation.mutate()}
                disabled={isBusy}
                className="btn-ghost py-2"
              >
                <RotateCcw className="w-4 h-4" /> Restart
              </button>
            </>
          ) : (
            <button
              onClick={() => startMutation.mutate()}
              disabled={isBusy}
              className="btn-primary"
            >
              <Play className="w-4 h-4" /> Start
            </button>
          )}
          <button
            onClick={() => api.post(`/api/v1/servers/${id}/backup`, { type: 'full' }).then(() => toast.success('Backup triggered'))}
            className="btn-ghost py-2"
          >
            <HardDrive className="w-4 h-4" /> Backup
          </button>
          <button onClick={() => setShowClone(true)} className="btn-ghost py-2">
            <Copy className="w-4 h-4" /> Clone
          </button>
          <button
            onClick={() => setShowShare(true)}
            className="btn-ghost py-2"
          >
            <Share2 className="w-4 h-4" /> Share
          </button>
        </div>
      </div>

      {showShare && <ShareModal server={server} onClose={() => setShowShare(false)} />}
      {showClone && <CloneModal serverId={server.id} serverName={server.name} onClose={() => setShowClone(false)} />}

      {/* Tab bar */}
      <div className="flex gap-1 overflow-x-auto" style={{ borderBottom: '1px solid var(--border)' }}>
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              'px-4 py-2.5 text-sm font-medium border-b-2 transition-colors whitespace-nowrap -mb-px',
              activeTab === tab.id
                ? 'border-orange-500'
                : 'border-transparent hover:border-white/20'
            )}
            style={{
              color: activeTab === tab.id ? 'var(--primary)' : 'var(--text-secondary)',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div>
        {activeTab === 'overview' && <OverviewTab server={server} />}
        {activeTab === 'console'  && <ConsoleTab serverId={id!} serverState={server.state} />}
        {activeTab === 'logs'     && <LogsTab serverId={id!} />}
        {activeTab === 'backups'  && <BackupsTab serverId={id!} />}
        {activeTab === 'mods'     && <ModsTab serverId={id!} />}
        {activeTab === 'ports'    && <PortsTab server={server} />}
        {activeTab === 'config'   && <ConfigTab server={server} />}
        {activeTab === 'schedule' && <ScheduleTab server={server} />}
        {activeTab === 'files'    && <FilesTab serverId={id!} />}
        {activeTab === 'players'  && <PlayersTab serverId={id!} adapter={server.adapter} />}
      </div>
    </div>
  );
}

// ── Clone server modal ────────────────────────────────────────────────────────

function CloneModal({ serverId, serverName, onClose }: { serverId: string; serverName: string; onClose: () => void }) {
  const navigate = useNavigate();
  const [name, setName] = useState(`${serverName} (copy)`);
  const cloneMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${serverId}/clone`, { name }).then(r => r.data),
    onSuccess: (clone) => {
      toast.success(`Cloned as "${clone.name}" — deploy it to start.`);
      onClose();
      navigate(`/servers/${clone.id}`);
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Clone failed'),
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}
      onClick={e => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="card w-full max-w-sm p-6 space-y-5">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Clone Server</h2>
          <button onClick={onClose} className="btn-ghost p-1.5"><X className="w-4 h-4" /></button>
        </div>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Creates a stopped copy with the same adapter, ports, and config. You'll need to deploy it before starting.
        </p>
        <div>
          <label className="label">New server name</label>
          <input
            className="input"
            value={name}
            onChange={e => setName(e.target.value)}
            autoFocus
            onKeyDown={e => e.key === 'Enter' && name.trim() && cloneMutation.mutate()}
          />
        </div>
        <div className="flex gap-3">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={() => cloneMutation.mutate()}
            disabled={!name.trim() || cloneMutation.isPending}
            className="btn-primary flex-1 justify-center"
          >
            {cloneMutation.isPending ? 'Cloning…' : 'Clone'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Share with Friends modal ──────────────────────────────────────────────────

// Build a game-specific join string (e.g. steam:// deep link for Steam games).
function joinString(adapter: string, host: string, port: number): string {
  const steamConnect = `steam://connect/${host}:${port}`;
  switch (adapter) {
    case 'valheim':
    case 'counter-strike-2':
    case 'team-fortress-2':
    case 'garrys-mod':
    case 'left-4-dead-2':
    case 'dota2':
    case 'rust':
    case 'ark-survival-ascended':
    case 'conan-exiles':
    case 'dayz':
    case 'squad':
    case 'risk-of-rain-2':
    case '7-days-to-die':
      return steamConnect;
    default:
      return `${host}:${port}`;
  }
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };
  return (
    <button
      onClick={copy}
      className="flex items-center gap-1 text-xs px-2 py-1 rounded-lg transition-colors shrink-0"
      style={{
        background: copied ? 'rgba(34,197,94,0.15)' : 'var(--bg-elevated)',
        color: copied ? '#4ade80' : 'var(--text-muted)',
        border: `1px solid ${copied ? 'rgba(34,197,94,0.3)' : 'var(--border)'}`,
      }}
    >
      {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
      {copied ? 'Copied' : 'Copy'}
    </button>
  );
}

function ShareModal({ server, onClose }: { server: any; onClose: () => void }) {
  const [publicHost, setPublicHost] = useState(window.location.hostname || 'your-server-ip');
  const [ipLoading, setIpLoading] = useState(true);
  const [showQR, setShowQR] = useState(false);
  const [reachability, setReachability] = useState<Record<string, boolean | null>>({});
  const [checkingReach, setCheckingReach] = useState(false);

  // Auto-detect public IP from daemon
  useEffect(() => {
    api.get('/api/v1/system/public-ip')
      .then(r => { if (r.data.public_ip) setPublicHost(r.data.public_ip); })
      .catch(() => {/* keep window.location.hostname */})
      .finally(() => setIpLoading(false));
  }, []);

  // Primary game port: first exposed port that isn't an RCON/admin port
  const primaryPort: number = (() => {
    const ports: any[] = server.ports ?? [];
    const game = ports.find((p: any) => p.exposed && !p.description?.toLowerCase().includes('rcon'));
    return game?.external ?? game?.internal ?? 0;
  })();

  const joinStr = primaryPort > 0 ? joinString(server.adapter, publicHost, primaryPort) : null;

  const checkReachability = async () => {
    if ((server.ports ?? []).length === 0) return;
    setCheckingReach(true);
    try {
      const res = await api.post('/api/v1/ports/validate', { ports: server.ports });
      const map: Record<string, boolean | null> = {};
      for (const r of res.data.results ?? []) {
        map[`${r.port.protocol}:${r.port.external ?? r.port.internal}`] = r.reachable;
      }
      setReachability(map);
    } catch {
      toast.error('Reachability check failed');
    } finally {
      setCheckingReach(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.65)' }}
      onClick={onClose}
    >
      <div
        className="card w-full max-w-md p-0 overflow-hidden"
        style={{ maxHeight: '90vh', overflowY: 'auto' }}
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4"
          style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
          <div className="flex items-center gap-2.5">
            <Share2 className="w-4 h-4" style={{ color: 'var(--primary)' }} />
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              Share — {server.name}
            </span>
          </div>
          <button onClick={onClose} className="w-7 h-7 flex items-center justify-center rounded-lg transition-colors"
            style={{ color: 'var(--text-muted)' }}>
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 space-y-4">
          {/* Public host */}
          <div>
            <label className="text-xs font-semibold uppercase tracking-wide mb-1.5 block"
              style={{ color: 'var(--text-muted)' }}>
              Your public hostname / IP
            </label>
            <div className="relative">
              <input
                className="input w-full font-mono text-sm pr-8"
                value={publicHost}
                onChange={e => setPublicHost(e.target.value)}
                placeholder="your-server-ip or domain.com"
              />
              {ipLoading && (
                <Loader2 className="w-3.5 h-3.5 animate-spin absolute right-2.5 top-1/2 -translate-y-1/2"
                  style={{ color: 'var(--text-muted)' }} />
              )}
            </div>
            <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>
              Auto-detected from the server. Edit if incorrect.
            </p>
          </div>

          {/* Port list with reachability */}
          {(server.ports ?? []).length > 0 && (
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-semibold uppercase tracking-wide"
                  style={{ color: 'var(--text-muted)' }}>
                  Ports
                </label>
                <button
                  onClick={checkReachability}
                  disabled={checkingReach}
                  className="flex items-center gap-1 text-xs px-2 py-0.5 rounded-lg transition-colors"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  {checkingReach
                    ? <><Loader2 className="w-3 h-3 animate-spin" />Checking…</>
                    : <><Wifi className="w-3 h-3" />Check reachability</>
                  }
                </button>
              </div>
              <div className="card p-0 overflow-hidden">
                {(server.ports ?? []).map((p: any, i: number) => {
                  const key = `${p.protocol}:${p.external ?? p.internal}`;
                  const reached = reachability[key];
                  return (
                    <div key={i}
                      className="flex items-center justify-between px-4 py-2.5 text-sm"
                      style={{ borderBottom: i < server.ports.length - 1 ? '1px solid var(--border)' : undefined }}
                    >
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-semibold" style={{ color: 'var(--text-primary)' }}>
                          {p.external ?? p.internal}
                        </span>
                        <span className="badge uppercase text-[10px]"
                          style={{
                            background: p.protocol === 'tcp' ? 'rgba(59,130,246,0.12)' : 'rgba(168,85,247,0.12)',
                            color: p.protocol === 'tcp' ? '#60a5fa' : '#c084fc',
                          }}>
                          {p.protocol}
                        </span>
                        {p.description && (
                          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>{p.description}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-1.5">
                        {/* Live reachability result takes priority over static exposed flag */}
                        {reached === true ? (
                          <><Wifi className="w-3.5 h-3.5 text-green-400" /><span className="text-xs text-green-400">Reachable</span></>
                        ) : reached === false ? (
                          <><AlertCircle className="w-3.5 h-3.5 text-amber-400" /><span className="text-xs text-amber-400">Blocked</span></>
                        ) : p.exposed ? (
                          <><Wifi className="w-3.5 h-3.5 text-green-400" /><span className="text-xs text-green-400">Open</span></>
                        ) : (
                          <><WifiOff className="w-3.5 h-3.5" style={{ color: 'var(--text-muted)' }} /><span className="text-xs" style={{ color: 'var(--text-muted)' }}>Internal</span></>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Join string */}
          {joinStr && (
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-semibold uppercase tracking-wide"
                  style={{ color: 'var(--text-muted)' }}>
                  Join string
                </label>
                <button
                  onClick={() => setShowQR(v => !v)}
                  className="flex items-center gap-1 text-xs px-2 py-0.5 rounded-lg transition-colors"
                  style={{
                    background: showQR ? 'rgba(249,115,22,0.12)' : 'var(--bg-elevated)',
                    border: `1px solid ${showQR ? 'rgba(249,115,22,0.3)' : 'var(--border)'}`,
                    color: showQR ? '#f97316' : 'var(--text-secondary)',
                  }}
                >
                  <QrCode className="w-3 h-3" />
                  {showQR ? 'Hide QR' : 'QR code'}
                </button>
              </div>
              <div className="flex items-center gap-2 rounded-lg px-3 py-2.5"
                style={{ background: '#080810', border: '1px solid var(--border)' }}>
                <span className="flex-1 font-mono text-xs break-all" style={{ color: 'var(--primary)' }}>
                  {joinStr}
                </span>
                <CopyButton text={joinStr} />
              </div>
              {joinStr.startsWith('steam://') && (
                <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>
                  Send this link to friends — clicking it opens the game and connects directly.
                </p>
              )}
              {showQR && (
                <div className="flex justify-center mt-3 p-4 rounded-xl"
                  style={{ background: '#ffffff' }}>
                  <QRCodeSVG value={joinStr} size={180} level="M" />
                </div>
              )}
            </div>
          )}

          {/* Direct address */}
          {primaryPort > 0 && (
            <div>
              <label className="text-xs font-semibold uppercase tracking-wide mb-1.5 block"
                style={{ color: 'var(--text-muted)' }}>
                Direct address
              </label>
              <div className="flex items-center gap-2 rounded-lg px-3 py-2.5"
                style={{ background: '#080810', border: '1px solid var(--border)' }}>
                <span className="flex-1 font-mono text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {publicHost}:{primaryPort}
                </span>
                <CopyButton text={`${publicHost}:${primaryPort}`} />
              </div>
            </div>
          )}

          {(server.ports ?? []).length === 0 && (
            <p className="text-sm text-center py-4" style={{ color: 'var(--text-muted)' }}>
              No ports configured on this server yet.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Diagnostics modal ─────────────────────────────────────────────────────────

const SEVERITY_STYLE: Record<string, { icon: React.ElementType; color: string; bg: string; border: string }> = {
  ok:      { icon: CheckCircle,   color: '#4ade80', bg: 'rgba(34,197,94,0.08)',   border: 'rgba(34,197,94,0.2)'  },
  warning: { icon: AlertTriangle, color: '#fbbf24', bg: 'rgba(251,191,36,0.08)',  border: 'rgba(251,191,36,0.2)' },
  error:   { icon: AlertCircle,   color: '#f87171', bg: 'rgba(239,68,68,0.08)',   border: 'rgba(239,68,68,0.2)'  },
};

function DiagnosticsModal({ serverId, onClose }: { serverId: string; onClose: () => void }) {
  const { data, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ['diagnose', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/diagnose`).then(r => r.data),
    staleTime: 0,
    gcTime: 0,
  });

  const findings: Array<{ severity: string; title: string; detail?: string; fix?: string }> = data?.findings ?? [];
  const hasErrors = findings.some(f => f.severity === 'error');
  const hasWarnings = findings.some(f => f.severity === 'warning');
  const summary = hasErrors ? 'Issues found' : hasWarnings ? 'Warnings found' : 'All checks passed';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}
      onClick={e => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="card w-full max-w-lg p-6 space-y-5" style={{ maxHeight: '90vh', overflowY: 'auto' }}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Stethoscope className="w-4 h-4" style={{ color: 'var(--primary)' }} />
            <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Server Diagnostics</h2>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => refetch()}
              disabled={isLoading || isRefetching}
              className="btn-ghost p-1.5 text-xs"
              title="Re-run diagnostics"
            >
              <RefreshCw className={cn('w-4 h-4', (isLoading || isRefetching) && 'animate-spin')} />
            </button>
            <button onClick={onClose} className="btn-ghost p-1.5"><X className="w-4 h-4" /></button>
          </div>
        </div>

        {isLoading ? (
          <div className="flex flex-col items-center py-10 gap-3">
            <Loader2 className="w-6 h-6 animate-spin" style={{ color: 'var(--primary)' }} />
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Running diagnostics…</p>
          </div>
        ) : (
          <>
            <p className="text-sm font-medium" style={{ color: hasErrors ? '#f87171' : hasWarnings ? '#fbbf24' : '#4ade80' }}>
              {summary}
            </p>

            <div className="space-y-2">
              {findings.map((f, i) => {
                const style = SEVERITY_STYLE[f.severity] ?? SEVERITY_STYLE.warning;
                const Icon = style.icon;
                return (
                  <div key={i} className="rounded-xl p-4 space-y-1" style={{ background: style.bg, border: `1px solid ${style.border}` }}>
                    <div className="flex items-center gap-2">
                      <Icon className="w-4 h-4 shrink-0" style={{ color: style.color }} />
                      <span className="font-medium text-sm" style={{ color: 'var(--text-primary)' }}>{f.title}</span>
                    </div>
                    {f.detail && (
                      <p className="text-xs ml-6" style={{ color: 'var(--text-secondary)' }}>{f.detail}</p>
                    )}
                    {f.fix && (
                      <p className="text-xs ml-6 mt-1" style={{ color: style.color }}>
                        <span className="font-semibold">Fix: </span>{f.fix}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>

            {findings.length === 0 && (
              <div className="text-center py-6 text-sm" style={{ color: 'var(--text-muted)' }}>
                No findings returned.
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// ── Overview tab ─────────────────────────────────────────────────────────────

function OverviewTab({ server }: { server: any }) {
  const queryClient = useQueryClient();
  const [showDiagnostics, setShowDiagnostics] = useState(false);
  const playerCount: number = server.player_count ?? -1;
  const maxPlayers: number = server.max_players ?? 0;
  const playerLabel = server.state !== 'running' || playerCount < 0
    ? '—'
    : maxPlayers > 0
      ? `${playerCount} / ${maxPlayers}`
      : String(playerCount);

  const updateMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/update`),
    onSuccess: () => {
      toast.success('Update started — server will restart when complete');
      queryClient.invalidateQueries({ queryKey: ['server', server.id] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Update failed'),
  });

  const autoUpdateMutation = useMutation({
    mutationFn: (payload: { auto_update: boolean; auto_update_schedule?: string }) =>
      api.put(`/api/v1/servers/${server.id}`, payload),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['server', server.id] }),
  });

  const [schedule, setSchedule] = useState<string>(server.auto_update_schedule || '0 4 * * *');

  return (
    <div className="space-y-4">
      {showDiagnostics && (
        <DiagnosticsModal serverId={server.id} onClose={() => setShowDiagnostics(false)} />
      )}

      {/* Error state banner with diagnose CTA */}
      {server.state === 'error' && (
        <div className="rounded-xl px-4 py-3 flex items-center justify-between gap-3"
          style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)' }}>
          <div className="flex items-center gap-2 min-w-0">
            <AlertCircle className="w-4 h-4 shrink-0 text-red-400" />
            <span className="text-sm text-red-400 truncate">
              {server.last_error || 'Server stopped with an error.'}
            </span>
          </div>
          <button
            onClick={() => setShowDiagnostics(true)}
            className="btn-ghost shrink-0 text-xs py-1.5 px-3 text-red-400"
            style={{ borderColor: 'rgba(239,68,68,0.3)' }}
          >
            <Stethoscope className="w-3.5 h-3.5" /> Diagnose
          </button>
        </div>
      )}

      {/* Quick metrics */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {[
          { icon: Cpu,         label: 'CPU Cores',  value: String(server.resources?.cpu_cores ?? '—')                         },
          { icon: MemoryStick, label: 'RAM',        value: server.resources?.ram_gb  ? `${server.resources.ram_gb} GB`  : '—' },
          { icon: HardDrive,   label: 'Disk',       value: server.resources?.disk_gb ? `${server.resources.disk_gb} GB` : '—' },
          { icon: Clock,       label: 'State',      value: server.state                                                        },
          { icon: Users,       label: 'Players',    value: playerLabel                                                         },
        ].map(({ icon: Icon, label, value }) => (
          <div key={label} className="card p-4">
            <div className="flex items-center gap-2 mb-2">
              <Icon className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
              <span className="text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>{label}</span>
            </div>
            <div className="text-xl font-bold capitalize" style={{ color: 'var(--text-primary)' }}>{value}</div>
          </div>
        ))}
      </div>

      {/* Details grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <InfoCard title="Server Details">
          <InfoRow label="ID"             value={server.id}                             mono />
          <InfoRow label="Adapter"        value={server.adapter}                              />
          <InfoRow label="Deploy Method"  value={server.deploy_method || 'steamcmd'}          />
          <InfoRow label="State"          value={server.state}                                />
          <InfoRow label="Install Dir"    value={server.install_dir || '/opt/games'}     mono />
          {server.container_id && (
            <InfoRow label="Container ID" value={server.container_id.slice(0, 12)}       mono />
          )}
          {server.last_update_check && (
            <InfoRow label="Last Updated" value={new Date(server.last_update_check).toLocaleString()} />
          )}
        </InfoCard>

        <div className="card p-5">
          <h3 className="label mb-4">Resources</h3>
          <div className="space-y-3">
            {[
              { label: 'CPU Cores', value: String(server.resources?.cpu_cores ?? '—') },
              { label: 'RAM',       value: server.resources?.ram_gb  ? `${server.resources.ram_gb} GB`  : '—' },
              { label: 'Disk',      value: server.resources?.disk_gb ? `${server.resources.disk_gb} GB` : '—' },
            ].map(({ label, value }) => (
              <InfoRow key={label} label={label} value={value} />
            ))}
          </div>
        </div>

        {/* Auto-update card */}
        <div className="card p-5">
          <h3 className="label mb-4">Auto-Update</h3>
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>Enabled</span>
              <button
                onClick={() => autoUpdateMutation.mutate({
                  auto_update: !server.auto_update,
                  auto_update_schedule: schedule,
                })}
                className={cn('relative inline-flex h-5 w-9 rounded-full transition-colors',
                  server.auto_update ? 'bg-[var(--primary)]' : 'bg-[var(--border)]')}
              >
                <span className={cn('inline-block h-4 w-4 mt-0.5 rounded-full bg-white shadow transition-transform',
                  server.auto_update ? 'translate-x-4' : 'translate-x-0.5')} />
              </button>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-sm shrink-0" style={{ color: 'var(--text-secondary)' }}>Schedule (cron)</span>
              <input
                value={schedule}
                onChange={e => setSchedule(e.target.value)}
                onBlur={() => {
                  if (server.auto_update && schedule !== server.auto_update_schedule) {
                    autoUpdateMutation.mutate({ auto_update: true, auto_update_schedule: schedule });
                  }
                }}
                className="input text-xs flex-1"
                placeholder="0 4 * * *"
              />
            </div>
            <button
              onClick={() => updateMutation.mutate()}
              disabled={updateMutation.isPending || !server.deploy_method}
              className="btn-ghost w-full py-1.5 text-sm"
            >
              <ArrowDownToLine className="w-4 h-4" />
              {updateMutation.isPending ? 'Updating…' : 'Update Now'}
            </button>
          </div>
        </div>

        <div className="md:col-span-2">
          <MetricsChart serverId={server.id} />
        </div>

        {/* Diagnostics card */}
        <div className="card p-5">
          <h3 className="label mb-3">Troubleshoot</h3>
          <p className="text-sm mb-4" style={{ color: 'var(--text-muted)' }}>
            Run a self-diagnosis check — Docker status, disk space, port conflicts, memory, and crash history.
          </p>
          <button onClick={() => setShowDiagnostics(true)} className="btn-ghost w-full justify-center">
            <Stethoscope className="w-4 h-4" /> Run Diagnostics
          </button>
        </div>
      </div>
    </div>
  );
}

function MetricsChart({ serverId }: { serverId: string }) {
  const { data } = useQuery({
    queryKey: ['server-metrics', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/metrics?n=30`).then(r => r.data),
    refetchInterval: 15_000,
  });

  const samples: ServerMetricSample[] = data?.samples ?? [];
  const hasNetData = samples.some(s => (s.net_in_kbps ?? 0) > 0 || (s.net_out_kbps ?? 0) > 0);
  const chartData = samples.map(s => ({
    time:    new Date(s.ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    cpu:     parseFloat(s.cpu_pct.toFixed(1)),
    ram:     parseFloat(s.ram_pct.toFixed(1)),
    netIn:   parseFloat(((s.net_in_kbps  ?? 0) / 1000).toFixed(2)), // Mbps for chart
    netOut:  parseFloat(((s.net_out_kbps ?? 0) / 1000).toFixed(2)),
  }));

  return (
    <div className="card p-5 space-y-5">
      <h3 className="label">Resource Utilization</h3>
      {chartData.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12">
          <Activity className="w-8 h-8 mb-2" style={{ color: 'var(--text-muted)' }} />
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            No data yet — metrics are collected every 15 s while the server is running.
          </p>
        </div>
      ) : (
        <>
          {/* CPU / RAM chart */}
          <div>
            <ResponsiveContainer width="100%" height={160}>
              <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.04)" />
                <XAxis dataKey="time" tick={{ fontSize: 10, fill: 'var(--text-muted)' }} interval="preserveStartEnd" />
                <YAxis domain={[0, 100]} tick={{ fontSize: 10, fill: 'var(--text-muted)' }} unit="%" />
                <Tooltip
                  contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)', borderRadius: 10, fontSize: 12 }}
                  labelStyle={{ color: 'var(--text-secondary)' }}
                />
                <Line type="monotone" dataKey="cpu" stroke="#f97316" strokeWidth={2} dot={false} name="CPU %" />
                <Line type="monotone" dataKey="ram" stroke="#3b82f6" strokeWidth={2} dot={false} name="RAM %" />
              </LineChart>
            </ResponsiveContainer>
            <div className="flex gap-4 mt-2">
              <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
                <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#f97316' }} /> CPU %
              </span>
              <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
                <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#3b82f6' }} /> RAM %
              </span>
            </div>
          </div>

          {/* Network I/O chart — only rendered when we have data */}
          {hasNetData && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide mb-2" style={{ color: 'var(--text-muted)' }}>Network I/O (Mbps)</p>
              <ResponsiveContainer width="100%" height={130}>
                <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.04)" />
                  <XAxis dataKey="time" tick={{ fontSize: 10, fill: 'var(--text-muted)' }} interval="preserveStartEnd" />
                  <YAxis tick={{ fontSize: 10, fill: 'var(--text-muted)' }} unit=" M" />
                  <Tooltip
                    contentStyle={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)', borderRadius: 10, fontSize: 12 }}
                    labelStyle={{ color: 'var(--text-secondary)' }}
                    formatter={(v: any) => [`${v} Mbps`]}
                  />
                  <Line type="monotone" dataKey="netIn"  stroke="#22c55e" strokeWidth={2} dot={false} name="↓ In" />
                  <Line type="monotone" dataKey="netOut" stroke="#a855f7" strokeWidth={2} dot={false} name="↑ Out" />
                </LineChart>
              </ResponsiveContainer>
              <div className="flex gap-4 mt-2">
                <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
                  <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#22c55e' }} /> ↓ In
                </span>
                <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
                  <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#a855f7' }} /> ↑ Out
                </span>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// ── Console tab ───────────────────────────────────────────────────────────────

function ConsoleTab({ serverId, serverState }: { serverId: string; serverState: string }) {
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [lines, setLines] = useState<string[]>(['Connecting to console...']);
  const [connected, setConnected] = useState(false);
  const [reconnectKey, setReconnectKey] = useState(0);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const historyRef = useRef<string[]>([]);
  const historyIdx = useRef(-1);

  const isRunning = serverState === 'running';

  useEffect(() => {
    setLines(['Connecting to console...']);
    setConnected(false);
    const ws = new WebSocket(getWsUrl(`/api/v1/servers/${serverId}/console/stream`));
    wsRef.current = ws;
    ws.onopen  = () => { setConnected(true); setLines(p => [...p, '— Connected —']); };
    ws.onmessage = e => {
      try {
        const m = JSON.parse(e.data);
        const prefix = m.type === 'rcon_cmd' ? '' : m.type === 'rcon_resp' ? '← ' : '';
        setLines(p => [...p.slice(-500), prefix + (m.msg ?? e.data)]);
      } catch {
        setLines(p => [...p.slice(-500), e.data]);
      }
    };
    ws.onclose = () => { setConnected(false); setLines(p => [...p, '— Disconnected —']); };
    ws.onerror = () => setLines(p => [...p, '— Connection error —']);
    return () => ws.close();
  }, [serverId, reconnectKey]);

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [lines]);

  const sendCommand = async () => {
    const cmd = input.trim();
    if (!cmd) return;
    historyRef.current = [cmd, ...historyRef.current.slice(0, 49)];
    historyIdx.current = -1;
    setInput('');
    setSending(true);
    try {
      await api.post(`/api/v1/servers/${serverId}/console/command`, { command: cmd });
    } catch (e: any) {
      const errMsg = e.response?.data?.error ?? 'Command failed';
      setLines(p => [...p.slice(-500), `[error] ${errMsg}`]);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') { sendCommand(); return; }
    if (e.key === 'ArrowUp') {
      const next = Math.min(historyIdx.current + 1, historyRef.current.length - 1);
      historyIdx.current = next;
      setInput(historyRef.current[next] ?? '');
      e.preventDefault();
    }
    if (e.key === 'ArrowDown') {
      const next = Math.max(historyIdx.current - 1, -1);
      historyIdx.current = next;
      setInput(next === -1 ? '' : historyRef.current[next] ?? '');
      e.preventDefault();
    }
  };

  return (
    <div className="card overflow-hidden">
      {/* Console header bar */}
      <div className="flex items-center justify-between px-4 py-3"
        style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
        <div className="flex items-center gap-2">
          <div className={cn('w-2 h-2 rounded-full', connected ? 'bg-green-400' : 'bg-red-500')} />
          <span className="text-xs font-medium" style={{ color: connected ? '#4ade80' : '#f87171' }}>
            {connected ? 'Connected' : 'Disconnected'}
          </span>
          {!connected && (
            <button
              onClick={() => setReconnectKey(k => k + 1)}
              className="flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-md transition-colors"
              style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
            >
              <RefreshCw className="w-2.5 h-2.5" /> Reconnect
            </button>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-3 h-3 rounded-full bg-red-500/70" />
          <div className="w-3 h-3 rounded-full bg-yellow-500/70" />
          <div className="w-3 h-3 rounded-full bg-green-500/70" />
        </div>
      </div>

      {/* Hint when not running */}
      {!isRunning && (
        <div className="mx-4 mt-3 rounded-xl px-3 py-2 text-xs flex items-start gap-2"
          style={{ background: 'rgba(249,115,22,0.08)', border: '1px solid rgba(249,115,22,0.2)', color: '#fb923c' }}>
          <FileText className="w-3.5 h-3.5 mt-0.5 shrink-0" />
          <span>
            The console streams live output while the server is running. To see startup output and pre-run events, check the <strong>Logs</strong> tab.
          </span>
        </div>
      )}

      {/* Log output */}
      <div ref={logRef}
        className="p-4 h-96 overflow-y-auto font-mono text-xs space-y-0.5"
        style={{ background: '#080810' }}>
        {lines.map((line, i) => (
          <div key={i} className={cn(
            'whitespace-pre-wrap break-all leading-5',
            line.startsWith('← ')     && 'text-green-400',
            line.startsWith('[error]') && 'text-red-400',
            line.startsWith('—')      && 'opacity-40',
          )} style={
            !line.startsWith('← ') && !line.startsWith('[error]') && !line.startsWith('—')
              ? { color: 'var(--text-primary)' }
              : {}
          }>{line}</div>
        ))}
      </div>

      {/* Command input */}
      <div className="p-4 space-y-2" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="flex gap-2">
          <div className="flex-1 flex items-center rounded-lg overflow-hidden"
            style={{ background: '#080810', border: '1px solid var(--border-strong)' }}>
            <span className="px-3 font-mono text-xs" style={{ color: 'var(--primary)' }}>$</span>
            <input
              type="text"
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter RCON command… (↑↓ history)"
              disabled={!connected || sending}
              className="flex-1 bg-transparent py-2 pr-3 text-sm font-mono outline-none disabled:opacity-40"
              style={{
                color: 'var(--text-primary)',
              }}
            />
          </div>
          <button
            onClick={sendCommand}
            disabled={!connected || sending || !input.trim()}
            className="btn-primary py-2 px-5"
          >
            {sending ? '…' : 'Send'}
          </button>
        </div>
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          Requires RCON on the adapter +{' '}
          <code className="font-mono text-xs px-1 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)' }}>
            rcon_password
          </code>{' '}
          in server config.
        </p>
      </div>
    </div>
  );
}

// ── Logs tab ──────────────────────────────────────────────────────────────────

const LOG_LINE_OPTIONS = [100, 200, 400, 800];

function LogsTab({ serverId }: { serverId: string }) {
  const logRef = useRef<HTMLDivElement>(null);
  const [lineCount, setLineCount] = useState(200);
  const [autoScroll, setAutoScroll] = useState(true);

  const { data, isFetching, refetch } = useQuery<string[]>({
    queryKey: ['server-logs', serverId, lineCount],
    queryFn: () =>
      api
        .get(`/api/v1/servers/${serverId}/logs`, { params: { lines: lineCount } })
        .then(r => r.data.logs as string[]),
    refetchInterval: 3_000,
  });

  const logs = data ?? [];

  useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const handleScroll = () => {
    if (!logRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  };

  return (
    <div className="card overflow-hidden" style={{ position: 'relative' }}>
      <div className="flex items-center justify-between px-4 py-3"
        style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
        <div className="flex items-center gap-2">
          <FileText className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
          <span className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>Server Log</span>
          {isFetching && (
            <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>Refreshing…</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <label className="text-[11px]" style={{ color: 'var(--text-muted)' }}>Lines</label>
          <select
            className="input text-xs"
            style={{ width: 80, padding: '2px 6px' }}
            value={lineCount}
            onChange={e => setLineCount(Number(e.target.value))}
          >
            {LOG_LINE_OPTIONS.map(n => <option key={n} value={n}>{n}</option>)}
          </select>
          <button
            onClick={() => refetch()}
            className="flex items-center justify-center w-7 h-7 rounded-lg transition-colors"
            style={{ color: 'var(--text-muted)', background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
            title="Refresh logs"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div
        ref={logRef}
        onScroll={handleScroll}
        className="p-4 h-96 overflow-y-auto font-mono text-xs space-y-0.5"
        style={{ background: '#080810' }}
      >
        {logs.length === 0 ? (
          <p className="opacity-40">No log entries yet. Logs appear here once the server has been started or deployed.</p>
        ) : (
          logs.map((line, i) => (
            <div
              key={i}
              className={cn(
                'whitespace-pre-wrap break-all leading-5',
                line.includes('[error]') || line.includes('ERROR') || line.includes('FATAL') ? 'text-red-400' :
                line.includes('WARN') ? 'text-yellow-400' :
                line.startsWith('{') ? 'opacity-70' : '',
              )}
              style={
                !line.includes('[error]') && !line.includes('ERROR') && !line.includes('FATAL') && !line.includes('WARN')
                  ? { color: 'var(--text-primary)' }
                  : {}
              }
            >
              {line}
            </div>
          ))
        )}
      </div>

      {!autoScroll && (
        <button
          onClick={() => {
            setAutoScroll(true);
            if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
          }}
          className="absolute bottom-20 right-6 text-xs px-3 py-1.5 rounded-full shadow-lg"
          style={{ background: 'var(--primary)', color: '#fff' }}
        >
          ↓ Jump to bottom
        </button>
      )}
    </div>
  );
}

// ── Backups tab ───────────────────────────────────────────────────────────────

function BackupsTab({ serverId }: { serverId: string }) {
  const { data } = useQuery({
    queryKey: ['backups', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/backups`).then(r => r.data),
  });
  const backups = data?.backups ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
          Backups
          <span className="ml-2 font-normal text-sm" style={{ color: 'var(--text-muted)' }}>({backups.length})</span>
        </h3>
        <button
          onClick={() => api.post(`/api/v1/servers/${serverId}/backup`, { type: 'full' }).then(() => toast.success('Backup started'))}
          className="btn-blue py-1.5 px-3 text-xs"
        >
          <Download className="w-3.5 h-3.5" /> New Backup
        </button>
      </div>

      {backups.length === 0 ? (
        <div className="card p-10 text-center">
          <HardDrive className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-muted)' }} />
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No backups yet.</p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <div className="grid grid-cols-12 gap-4 px-5 py-3 text-xs font-semibold uppercase tracking-wider"
            style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
            <div className="col-span-5">ID</div>
            <div className="col-span-2">Type</div>
            <div className="col-span-3">Created</div>
            <div className="col-span-2 text-right">Action</div>
          </div>
          {backups.map((b: any, i: number) => (
            <div key={b.id}
              className="grid grid-cols-12 gap-4 px-5 py-3 items-center text-sm transition-colors"
              style={{ borderBottom: i < backups.length - 1 ? '1px solid var(--border)' : undefined }}
              onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
              onMouseLeave={e => (e.currentTarget.style.background = '')}
            >
              <div className="col-span-5 font-mono text-xs truncate" style={{ color: 'var(--text-primary)' }}>{b.id}</div>
              <div className="col-span-2">
                <span className="badge capitalize"
                  style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)' }}>
                  {b.type}
                </span>
              </div>
              <div className="col-span-3 text-xs" style={{ color: 'var(--text-muted)' }}>{b.created_at}</div>
              <div className="col-span-2 text-right">
                <button
                  onClick={() => api.post(`/api/v1/servers/${serverId}/restore/${b.id}`).then(() => toast.success('Restore started'))}
                  className="text-xs px-2 py-1 rounded transition-colors"
                  style={{ color: '#60a5fa' }}
                >
                  Restore
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Mods tab ─────────────────────────────────────────────────────────────────

const MOD_SOURCE_STYLES: Record<string, React.CSSProperties> = {
  steam:        { background: 'rgba(59,130,246,0.15)',  color: '#60a5fa' },
  curseforge:   { background: 'rgba(249,115,22,0.15)',  color: '#fb923c' },
  thunderstore: { background: 'rgba(34,197,94,0.15)',   color: '#4ade80' },
  git:          { background: 'rgba(168,85,247,0.15)',  color: '#c084fc' },
  local:        { background: 'rgba(128,128,168,0.12)', color: '#8080a8' },
  modrinth:     { background: 'rgba(34,197,94,0.15)',   color: '#4ade80' },
};

function ModsTab({ serverId }: { serverId: string }) {
  const { data } = useQuery({
    queryKey: ['mods', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/mods`).then(r => r.data),
  });
  const mods = data?.mods ?? [];

  const [testResult, setTestResult] = useState<null | { passed: boolean; duration: number; tests: { name: string; passed: boolean; message: string }[] }>(null);
  const runTests = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${serverId}/mods/test`).then(r => r.data),
    onSuccess: (data) => setTestResult(data),
    onError: () => toast.error('Test run failed'),
  });

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
          Mods
          <span className="ml-2 font-normal" style={{ color: 'var(--text-muted)' }}>({mods.length})</span>
        </h3>
        <div className="flex gap-2">
          <button
            onClick={() => runTests.mutate()}
            disabled={runTests.isPending}
            className="btn-ghost py-1.5 px-3 text-xs"
          >
            {runTests.isPending ? 'Running…' : 'Run Tests'}
          </button>
          <button className="btn-primary py-1.5 px-3 text-xs">
            <Package className="w-3.5 h-3.5" /> Add Mod
          </button>
        </div>
      </div>

      {/* Test Results */}
      {testResult && (
        <div className={cn(
          'card p-5 space-y-3',
          testResult.passed ? 'border-green-500/25' : 'border-red-500/25'
        )}>
          <div className="flex items-center justify-between">
            <span className={cn(
              'text-sm font-semibold',
              testResult.passed ? 'text-green-400' : 'text-red-400'
            )}>
              {testResult.passed ? 'All tests passed' : 'Some tests failed'}
            </span>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {Math.round(testResult.duration / 1e6)} ms
            </span>
          </div>
          <div className="space-y-1.5">
            {testResult.tests.map((t) => (
              <div key={t.name} className="flex items-start gap-2 text-xs">
                <span className={cn('mt-0.5 shrink-0 font-bold', t.passed ? 'text-green-400' : 'text-red-400')}>
                  {t.passed ? '✓' : '✗'}
                </span>
                <span className="font-mono w-36 shrink-0" style={{ color: 'var(--text-secondary)' }}>{t.name}</span>
                <span style={{ color: 'var(--text-muted)' }}>{t.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Mod grid */}
      {mods.length === 0 ? (
        <div className="card p-10 text-center">
          <Package className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-muted)' }} />
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No mods installed.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {mods.map((m: any) => (
            <div key={m.id} className="card p-4 space-y-2">
              <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{m.name}</div>
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-mono text-xs" style={{ color: 'var(--text-muted)' }}>{m.version}</span>
                <span className="badge text-[10px]"
                  style={MOD_SOURCE_STYLES[m.source] ?? { background: 'rgba(128,128,168,0.12)', color: '#8080a8' }}>
                  {m.source}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Ports tab ─────────────────────────────────────────────────────────────────

function PortsTab({ server }: { server: any }) {
  const ports = server.ports ?? [];
  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
        Port Mappings
        <span className="ml-2 font-normal" style={{ color: 'var(--text-muted)' }}>({ports.length})</span>
      </h3>
      {ports.length === 0 ? (
        <div className="card p-10 text-center">
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No ports configured.</p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          {ports.map((p: any, i: number) => (
            <div key={i}
              className="flex items-center justify-between px-5 py-3.5 transition-colors"
              style={{ borderBottom: i < ports.length - 1 ? '1px solid var(--border)' : undefined }}
              onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
              onMouseLeave={e => (e.currentTarget.style.background = '')}
            >
              <div className="flex items-center gap-3 font-mono text-sm">
                <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>{p.internal}</span>
                <span style={{ color: 'var(--text-muted)' }}>→</span>
                <span style={{ color: 'var(--text-secondary)' }}>{p.external}</span>
                <span className="badge uppercase text-[10px]"
                  style={{
                    background: p.protocol === 'tcp' ? 'rgba(59,130,246,0.12)' : 'rgba(168,85,247,0.12)',
                    color: p.protocol === 'tcp' ? '#60a5fa' : '#c084fc',
                  }}>
                  {p.protocol}
                </span>
              </div>
              <div className="flex items-center gap-1.5 text-xs">
                <div className={cn('w-1.5 h-1.5 rounded-full', p.exposed ? 'bg-green-400' : 'bg-gray-600')} />
                <span style={{ color: p.exposed ? '#4ade80' : 'var(--text-muted)' }}>
                  {p.exposed ? 'Exposed' : 'Internal'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── Config tab ────────────────────────────────────────────────────────────────

function ConfigTab({ server }: { server: any }) {
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [editorContent, setEditorContent] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);

  const { data: filesData } = useQuery({
    queryKey: ['config-files', server.id],
    queryFn: () => api.get(`/api/v1/servers/${server.id}/config-files`).then(r => r.data),
  });

  const files: Array<{ path: string; description: string; sample: string }> = filesData?.files ?? [];

  // Select first file automatically once list loads.
  useEffect(() => {
    if (files.length > 0 && !selectedPath) {
      setSelectedPath(files[0].path);
    }
  }, [files, selectedPath]);

  const { isFetching: loadingContent, refetch: refetchContent } = useQuery({
    queryKey: ['config-file-content', server.id, selectedPath],
    queryFn: async () => {
      if (!selectedPath) return { content: '' };
      const r = await api.get(`/api/v1/servers/${server.id}/config-files${selectedPath}`);
      setEditorContent(r.data.content ?? '');
      setDirty(false);
      return r.data;
    },
    enabled: !!selectedPath,
  });

  const handleSave = async () => {
    if (!selectedPath) return;
    setSaving(true);
    try {
      await api.put(`/api/v1/servers/${server.id}/config-files${selectedPath}`, { content: editorContent });
      toast.success('Config file saved');
      setDirty(false);
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleSelect = (path: string) => {
    if (dirty) {
      if (!window.confirm('Discard unsaved changes?')) return;
    }
    setSelectedPath(path);
    setDirty(false);
  };

  if (files.length === 0 && !filesData) {
    return (
      <div className="card p-10 text-center">
        <div className="animate-pulse h-4 w-48 rounded mx-auto mb-3" style={{ background: 'var(--bg-elevated)' }} />
        <div className="animate-pulse h-4 w-32 rounded mx-auto" style={{ background: 'var(--bg-elevated)' }} />
      </div>
    );
  }

  if (files.length === 0) {
    return (
      <div className="card p-10 text-center">
        <FolderOpen className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-muted)' }} />
        <p className="text-sm font-semibold mb-1" style={{ color: 'var(--text-primary)' }}>
          No config files declared
        </p>
        <p className="text-xs max-w-xs mx-auto" style={{ color: 'var(--text-muted)' }}>
          The {server.adapter} adapter has no well-known config files registered in its manifest.
        </p>
      </div>
    );
  }

  const selectedFile = files.find(f => f.path === selectedPath);

  return (
    <div className="flex gap-4 min-h-0" style={{ height: 560 }}>
      {/* File list sidebar */}
      <div className="w-52 shrink-0 card overflow-hidden flex flex-col">
        <div className="px-4 py-3 text-xs font-semibold uppercase tracking-wide"
          style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
          Config Files
        </div>
        <div className="overflow-y-auto flex-1">
          {files.map(f => (
            <button
              key={f.path}
              onClick={() => handleSelect(f.path)}
              className="w-full text-left px-4 py-3 transition-colors"
              style={{
                borderBottom: '1px solid var(--border)',
                background: selectedPath === f.path ? 'var(--primary-subtle)' : 'transparent',
                color: selectedPath === f.path ? 'var(--primary)' : 'var(--text-secondary)',
              }}
            >
              <div className="flex items-center gap-2 mb-0.5">
                <FileText className="w-3.5 h-3.5 shrink-0" />
                <span className="text-xs font-medium truncate">
                  {f.path.split('/').pop()}
                </span>
              </div>
              {f.description && (
                <div className="text-[11px] leading-tight ml-5.5 truncate"
                  style={{ color: 'var(--text-muted)' }}>
                  {f.description}
                </div>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Editor panel */}
      <div className="flex-1 card overflow-hidden flex flex-col min-w-0">
        {/* Editor header */}
        <div className="flex items-center justify-between px-4 py-2.5 shrink-0"
          style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
          <div className="flex items-center gap-2 min-w-0">
            <div className="flex items-center gap-1">
              <div className="w-2.5 h-2.5 rounded-full bg-red-500/70" />
              <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/70" />
              <div className="w-2.5 h-2.5 rounded-full bg-green-500/70" />
            </div>
            <span className="text-xs font-mono ml-1 truncate" style={{ color: 'var(--text-muted)' }}>
              {selectedPath ?? ''}
            </span>
            {dirty && (
              <span className="text-[10px] px-1.5 py-0.5 rounded"
                style={{ background: 'rgba(249,115,22,0.15)', color: '#fb923c' }}>
                unsaved
              </span>
            )}
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {loadingContent && (
              <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>Loading…</span>
            )}
            <button
              onClick={handleSave}
              disabled={!dirty || saving}
              className="btn-primary py-1.5 px-3 text-xs flex items-center gap-1.5 disabled:opacity-40"
            >
              <Save className="w-3.5 h-3.5" />
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>

        {/* Description bar */}
        {selectedFile?.description && (
          <div className="px-4 py-1.5 text-xs shrink-0"
            style={{ background: 'rgba(249,115,22,0.06)', borderBottom: '1px solid var(--border)', color: 'var(--text-muted)' }}>
            {selectedFile.description}
          </div>
        )}

        {/* Textarea */}
        <textarea
          className="flex-1 p-4 font-mono text-xs resize-none outline-none w-full"
          style={{
            background: '#080810',
            color: 'var(--text-primary)',
            caretColor: 'var(--primary)',
            tabSize: 2,
          }}
          value={editorContent}
          onChange={e => { setEditorContent(e.target.value); setDirty(true); }}
          spellCheck={false}
          placeholder={loadingContent ? 'Loading…' : 'File is empty. Start typing to create it.'}
        />

        {/* Status bar */}
        <div className="px-4 py-1.5 flex items-center justify-between text-[11px] shrink-0"
          style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
          <span>{editorContent.split('\n').length} lines · {editorContent.length} chars</span>
          <span>{selectedPath?.split('/').pop() ?? ''}</span>
        </div>
      </div>
    </div>
  );
}

// ── Schedule tab ──────────────────────────────────────────────────────────────

// Minimal cron expression human-readable label (covers the common cases).
function describeCron(expr: string): string {
  if (!expr) return '';
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 5) return expr;
  const [min, hour, dom, , dow] = parts;
  const days: Record<string, string> = { '0': 'Sun', '1': 'Mon', '2': 'Tue', '3': 'Wed', '4': 'Thu', '5': 'Fri', '6': 'Sat' };
  const pad = (n: string) => n.padStart(2, '0');
  const time = (h: string, m: string) => `${pad(h)}:${pad(m)}`;

  if (dom === '*' && dow === '*') return `Every day at ${time(hour, min)}`;
  if (dom === '*' && dow !== '*') {
    const dayNames = dow.split(',').map(d => days[d] ?? d).join(', ');
    return `Every ${dayNames} at ${time(hour, min)}`;
  }
  return expr;
}

function ScheduleTab({ server }: { server: any }) {
  const qc = useQueryClient();
  const [startExpr, setStartExpr] = useState<string>(server.start_schedule ?? '');
  const [stopExpr, setStopExpr]   = useState<string>(server.stop_schedule  ?? '');
  const [saving, setSaving] = useState(false);

  const save = async () => {
    setSaving(true);
    try {
      await api.put(`/api/v1/servers/${server.id}`, {
        start_schedule: startExpr,
        stop_schedule:  stopExpr,
      });
      qc.invalidateQueries({ queryKey: ['server', server.id] });
      toast.success('Schedule saved');
    } catch {
      toast.error('Failed to save schedule');
    } finally {
      setSaving(false);
    }
  };

  const startDesc = describeCron(startExpr);
  const stopDesc  = describeCron(stopExpr);

  return (
    <div className="space-y-6 max-w-xl">
      <div>
        <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Scheduled Start / Stop</h3>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Automatically start and stop this server on a cron schedule.
          Uses standard 5-field cron syntax: <code className="font-mono text-xs px-1 rounded" style={{ background: 'var(--bg-elevated)' }}>minute hour day month weekday</code>
        </p>
      </div>

      {/* Start schedule */}
      <div className="card p-5 space-y-3">
        <div className="flex items-center gap-2">
          <Play className="w-4 h-4" style={{ color: '#4ade80' }} />
          <span className="font-medium text-sm">Auto-start</span>
        </div>
        <input
          value={startExpr}
          onChange={e => setStartExpr(e.target.value)}
          placeholder="e.g. 0 18 * * 1-5  (weekdays at 18:00)"
          className="input w-full font-mono text-sm"
        />
        {startDesc && (
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            → {startDesc}
          </p>
        )}
        {startExpr && !startDesc && (
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Custom expression — verify with <code className="font-mono">crontab.guru</code>
          </p>
        )}
        <div className="flex gap-2 flex-wrap">
          {[
            { label: 'Daily 6 PM',       value: '0 18 * * *'   },
            { label: 'Weekdays 6 PM',    value: '0 18 * * 1-5' },
            { label: 'Daily 8 AM',       value: '0 8 * * *'    },
            { label: 'Clear',            value: ''              },
          ].map(p => (
            <button key={p.label} onClick={() => setStartExpr(p.value)}
              className="text-xs px-2 py-1 rounded-md transition-colors"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)', border: '1px solid var(--border)' }}
              onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
              onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-secondary)')}>
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {/* Stop schedule */}
      <div className="card p-5 space-y-3">
        <div className="flex items-center gap-2">
          <Square className="w-4 h-4" style={{ color: '#f87171' }} />
          <span className="font-medium text-sm">Auto-stop</span>
        </div>
        <input
          value={stopExpr}
          onChange={e => setStopExpr(e.target.value)}
          placeholder="e.g. 0 23 * * 1-5  (weekdays at 23:00)"
          className="input w-full font-mono text-sm"
        />
        {stopDesc && (
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            → {stopDesc}
          </p>
        )}
        <div className="flex gap-2 flex-wrap">
          {[
            { label: 'Daily 11 PM',      value: '0 23 * * *'   },
            { label: 'Weekdays 11 PM',   value: '0 23 * * 1-5' },
            { label: 'Daily 6 AM',       value: '0 6 * * *'    },
            { label: 'Clear',            value: ''              },
          ].map(p => (
            <button key={p.label} onClick={() => setStopExpr(p.value)}
              className="text-xs px-2 py-1 rounded-md transition-colors"
              style={{ background: 'var(--bg-elevated)', color: 'var(--text-secondary)', border: '1px solid var(--border)' }}
              onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-primary)')}
              onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-secondary)')}>
              {p.label}
            </button>
          ))}
        </div>
      </div>

      <button onClick={save} disabled={saving} className="btn-primary">
        {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
        {saving ? 'Saving…' : 'Save schedule'}
      </button>
    </div>
  );
}

// ── Files tab ─────────────────────────────────────────────────────────────────

interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  modified: string;
  path: string;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function FilesTab({ serverId }: { serverId: string }) {
  const [currentPath, setCurrentPath] = useState('/');
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null);
  const uploadRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ['files', serverId, currentPath],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/files`, { params: { path: currentPath } }).then(r => r.data),
  });

  const entries: FileEntry[] = data?.entries ?? [];

  const deleteMutation = useMutation({
    mutationFn: (path: string) => api.delete(`/api/v1/servers/${serverId}/files`, { params: { path } }),
    onSuccess: () => {
      toast.success('File deleted');
      setDeleteTarget(null);
      queryClient.invalidateQueries({ queryKey: ['files', serverId, currentPath] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Delete failed'),
  });

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    const form = new FormData();
    for (let i = 0; i < files.length; i++) form.append('file', files[i]);
    try {
      await api.post(`/api/v1/servers/${serverId}/files/upload`, form, {
        params: { dir: currentPath },
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      toast.success(`Uploaded ${files.length} file(s)`);
      queryClient.invalidateQueries({ queryKey: ['files', serverId, currentPath] });
    } catch (e: any) {
      toast.error(e?.response?.data?.error ?? 'Upload failed');
    }
    if (uploadRef.current) uploadRef.current.value = '';
  };

  const handleDownload = (entry: FileEntry) => {
    const url = `/api/v1/servers/${serverId}/files/download?path=${encodeURIComponent(entry.path)}`;
    const a = document.createElement('a');
    a.href = url;
    a.download = entry.name;
    a.click();
  };

  // Build breadcrumb segments from currentPath.
  const segments = currentPath === '/'
    ? []
    : currentPath.split('/').filter(Boolean);

  const navigateTo = (idx: number) => {
    if (idx < 0) { setCurrentPath('/'); return; }
    setCurrentPath('/' + segments.slice(0, idx + 1).join('/'));
  };

  return (
    <div className="card overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-3 gap-3"
        style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
        {/* Breadcrumb */}
        <div className="flex items-center gap-1 text-sm flex-1 min-w-0 overflow-hidden">
          <button
            onClick={() => setCurrentPath('/')}
            className="transition-colors hover:underline shrink-0"
            style={{ color: 'var(--text-secondary)' }}
          >
            <FolderOpen className="w-4 h-4 inline-block mr-1" />
            root
          </button>
          {segments.map((seg, i) => (
            <React.Fragment key={i}>
              <ChevronRight className="w-3.5 h-3.5 shrink-0" style={{ color: 'var(--text-muted)' }} />
              <button
                onClick={() => navigateTo(i)}
                className="transition-colors hover:underline truncate max-w-[120px]"
                style={{ color: i === segments.length - 1 ? 'var(--text-primary)' : 'var(--text-secondary)' }}
              >
                {seg}
              </button>
            </React.Fragment>
          ))}
        </div>
        {/* Actions */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            className="btn-secondary flex items-center gap-1.5 text-xs px-3 py-1.5"
            onClick={() => uploadRef.current?.click()}
          >
            <Upload className="w-3.5 h-3.5" /> Upload
          </button>
          <input ref={uploadRef} type="file" multiple className="hidden" onChange={handleUpload} />
        </div>
      </div>

      {/* File list */}
      {isLoading ? (
        <div className="p-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>Loading…</div>
      ) : error ? (
        <div className="p-8 text-center text-sm" style={{ color: 'var(--error)' }}>
          Could not load files — make sure the server has been deployed.
        </div>
      ) : entries.length === 0 ? (
        <div className="p-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
          This directory is empty.
        </div>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
              <th className="text-left px-4 py-2 font-medium" style={{ color: 'var(--text-secondary)' }}>Name</th>
              <th className="text-right px-4 py-2 font-medium hidden sm:table-cell" style={{ color: 'var(--text-secondary)' }}>Size</th>
              <th className="text-right px-4 py-2 font-medium hidden md:table-cell" style={{ color: 'var(--text-secondary)' }}>Modified</th>
              <th className="px-4 py-2" />
            </tr>
          </thead>
          <tbody>
            {/* Parent dir row */}
            {currentPath !== '/' && (
              <tr
                className="cursor-pointer transition-colors"
                style={{ borderBottom: '1px solid var(--border)' }}
                onClick={() => {
                  const parent = currentPath.split('/').slice(0, -1).join('/') || '/';
                  setCurrentPath(parent);
                }}
              >
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-2.5">
                    <Folder className="w-4 h-4 shrink-0" style={{ color: 'var(--text-muted)' }} />
                    <span style={{ color: 'var(--text-secondary)' }}>..</span>
                  </div>
                </td>
                <td className="px-4 py-2.5 hidden sm:table-cell" />
                <td className="px-4 py-2.5 hidden md:table-cell" />
                <td className="px-4 py-2.5" />
              </tr>
            )}
            {entries
              .slice()
              .sort((a, b) => {
                if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                return a.name.localeCompare(b.name);
              })
              .map(entry => (
                <tr
                  key={entry.path}
                  className="transition-colors"
                  style={{ borderBottom: '1px solid var(--border)' }}
                >
                  <td className="px-4 py-2.5">
                    <div className="flex items-center gap-2.5">
                      {entry.is_dir
                        ? <Folder className="w-4 h-4 shrink-0 text-yellow-400" />
                        : <File className="w-4 h-4 shrink-0" style={{ color: 'var(--text-muted)' }} />}
                      {entry.is_dir ? (
                        <button
                          className="hover:underline text-left truncate"
                          style={{ color: 'var(--text-primary)' }}
                          onClick={() => setCurrentPath(entry.path)}
                        >
                          {entry.name}
                        </button>
                      ) : (
                        <span className="truncate" style={{ color: 'var(--text-primary)' }}>{entry.name}</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2.5 text-right hidden sm:table-cell font-mono text-xs"
                    style={{ color: 'var(--text-secondary)' }}>
                    {entry.is_dir ? '—' : formatBytes(entry.size)}
                  </td>
                  <td className="px-4 py-2.5 text-right hidden md:table-cell text-xs"
                    style={{ color: 'var(--text-muted)' }}>
                    {new Date(entry.modified).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2.5">
                    <div className="flex items-center justify-end gap-1">
                      {!entry.is_dir && (
                        <button
                          className="p-1.5 rounded transition-colors"
                          style={{ color: 'var(--text-muted)' }}
                          title="Download"
                          onClick={() => handleDownload(entry)}
                        >
                          <Download className="w-3.5 h-3.5" />
                        </button>
                      )}
                      <button
                        className="p-1.5 rounded transition-colors"
                        style={{ color: 'var(--text-muted)' }}
                        title="Delete"
                        onClick={() => setDeleteTarget(entry)}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      )}

      {/* Delete confirmation modal */}
      {deleteTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ background: 'rgba(0,0,0,0.6)' }}
          onClick={() => setDeleteTarget(null)}
        >
          <div
            className="card p-6 max-w-sm w-full"
            style={{ background: 'var(--bg-card)' }}
            onClick={e => e.stopPropagation()}
          >
            <h3 className="font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
              Delete {deleteTarget.is_dir ? 'folder' : 'file'}?
            </h3>
            <p className="text-sm mb-5" style={{ color: 'var(--text-secondary)' }}>
              <span className="font-mono">{deleteTarget.name}</span> will be permanently deleted.
            </p>
            <div className="flex justify-end gap-3">
              <button className="btn-secondary" onClick={() => setDeleteTarget(null)}>Cancel</button>
              <button
                className="btn-danger"
                onClick={() => deleteMutation.mutate(deleteTarget.path)}
                disabled={deleteMutation.isPending}
              >
                {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Shared helpers ────────────────────────────────────────────────────────────

function InfoCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="card p-5">
      <h3 className="label mb-4">{title}</h3>
      <div className="space-y-2">{children}</div>
    </div>
  );
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm py-1"
      style={{ borderBottom: '1px solid var(--border)' }}>
      <span style={{ color: 'var(--text-secondary)' }}>{label}</span>
      <span className={cn(mono && 'font-mono text-xs')} style={{ color: 'var(--text-primary)' }}>{value}</span>
    </div>
  );
}

// ── Players tab ───────────────────────────────────────────────────────────────

function PlayersTab({ serverId, adapter }: { serverId: string; adapter: string }) {
  const queryClient = useQueryClient();
  const [banInput, setBanInput] = React.useState('');
  const [banReason, setBanReason] = React.useState('');
  const [wlInput, setWlInput] = React.useState('');

  const banlistQuery = useQuery({
    queryKey: ['banlist', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/banlist`).then(r => r.data as { players: string[]; supported: boolean; online: boolean; error?: string }),
    retry: false,
  });

  const whitelistQuery = useQuery({
    queryKey: ['whitelist', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/whitelist`).then(r => r.data as { players: string[]; supported: boolean; online: boolean; error?: string }),
    retry: false,
  });

  const banMutation = useMutation({
    mutationFn: ({ player, reason }: { player: string; reason: string }) =>
      api.post(`/api/v1/servers/${serverId}/banlist`, { player, reason }),
    onSuccess: () => {
      toast.success('Player banned');
      setBanInput('');
      setBanReason('');
      queryClient.invalidateQueries({ queryKey: ['banlist', serverId] });
    },
    onError: (e: { response?: { data?: { error?: string } } }) =>
      toast.error(e.response?.data?.error ?? 'Failed to ban player'),
  });

  const unbanMutation = useMutation({
    mutationFn: (player: string) =>
      api.delete(`/api/v1/servers/${serverId}/banlist/${encodeURIComponent(player)}`),
    onSuccess: () => {
      toast.success('Player unbanned');
      queryClient.invalidateQueries({ queryKey: ['banlist', serverId] });
    },
    onError: (e: { response?: { data?: { error?: string } } }) =>
      toast.error(e.response?.data?.error ?? 'Failed to unban player'),
  });

  const wlAddMutation = useMutation({
    mutationFn: (player: string) =>
      api.post(`/api/v1/servers/${serverId}/whitelist`, { player }),
    onSuccess: () => {
      toast.success('Added to whitelist');
      setWlInput('');
      queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] });
    },
    onError: (e: { response?: { data?: { error?: string } } }) =>
      toast.error(e.response?.data?.error ?? 'Failed to add player'),
  });

  const wlRemoveMutation = useMutation({
    mutationFn: (player: string) =>
      api.delete(`/api/v1/servers/${serverId}/whitelist/${encodeURIComponent(player)}`),
    onSuccess: () => {
      toast.success('Removed from whitelist');
      queryClient.invalidateQueries({ queryKey: ['whitelist', serverId] });
    },
    onError: (e: { response?: { data?: { error?: string } } }) =>
      toast.error(e.response?.data?.error ?? 'Failed to remove player'),
  });

  const banSupported = banlistQuery.data?.supported !== false;
  const wlSupported = whitelistQuery.data?.supported !== false;

  return (
    <div className="space-y-6">
      {/* Ban List */}
      <div className="card p-5">
        <div className="flex items-center gap-2 mb-4">
          <Users className="w-4 h-4" style={{ color: 'var(--primary)' }} />
          <h3 className="label">Ban List</h3>
        </div>

        {!banSupported ? (
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Ban management is not supported for <span className="font-medium capitalize">{adapter}</span> servers.
            {(adapter === 'ark-survival-ascended' || adapter === 'conan-exiles') &&
              ' You can ban players by Steam ID via the console tab.'}
          </p>
        ) : (
          <>
            {/* Ban input */}
            <div className="flex gap-2 mb-4">
              <input
                className="input flex-1"
                placeholder="Player name"
                value={banInput}
                onChange={e => setBanInput(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && banInput.trim() && banMutation.mutate({ player: banInput.trim(), reason: banReason.trim() })}
              />
              <input
                className="input w-40"
                placeholder="Reason (optional)"
                value={banReason}
                onChange={e => setBanReason(e.target.value)}
              />
              <button
                className="btn-danger py-2"
                disabled={!banInput.trim() || banMutation.isPending}
                onClick={() => banMutation.mutate({ player: banInput.trim(), reason: banReason.trim() })}
              >
                Ban
              </button>
            </div>

            {/* Banned players list */}
            {banlistQuery.isLoading ? (
              <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>Loading…</div>
            ) : banlistQuery.data?.online === false ? (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Start the server to view and manage the ban list.
              </p>
            ) : banlistQuery.isError ? (
              <div className="text-sm" style={{ color: 'var(--danger)' }}>
                Could not load ban list — check that RCON is enabled and the password is set.
              </div>
            ) : banlistQuery.data?.players.length === 0 ? (
              <p className="text-sm" style={{ color: 'var(--text-muted)' }}>No banned players.</p>
            ) : (
              <ul className="space-y-1">
                {banlistQuery.data!.players.map(player => (
                  <li key={player}
                    className="flex items-center justify-between text-sm px-3 py-2 rounded-lg"
                    style={{ background: 'var(--surface-raised)', border: '1px solid var(--border)' }}>
                    <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{player}</span>
                    <button
                      className="btn-ghost py-1 px-2 text-xs"
                      style={{ color: 'var(--danger)' }}
                      disabled={unbanMutation.isPending}
                      onClick={() => unbanMutation.mutate(player)}
                    >
                      <X className="w-3 h-3 inline-block mr-1" />Unban
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>

      {/* Whitelist */}
      <div className="card p-5">
        <div className="flex items-center gap-2 mb-4">
          <Check className="w-4 h-4" style={{ color: 'var(--success, #22c55e)' }} />
          <h3 className="label">Whitelist</h3>
        </div>

        {!wlSupported ? (
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Whitelist management via RCON is only supported for Minecraft servers.
          </p>
        ) : (
          <>
            {/* Add input */}
            <div className="flex gap-2 mb-4">
              <input
                className="input flex-1"
                placeholder="Player name"
                value={wlInput}
                onChange={e => setWlInput(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && wlInput.trim() && wlAddMutation.mutate(wlInput.trim())}
              />
              <button
                className="btn-primary py-2"
                disabled={!wlInput.trim() || wlAddMutation.isPending}
                onClick={() => wlAddMutation.mutate(wlInput.trim())}
              >
                Add
              </button>
            </div>

            {/* Whitelist */}
            {whitelistQuery.isLoading ? (
              <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>Loading…</div>
            ) : whitelistQuery.data?.online === false ? (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Start the server to view and manage the whitelist.
              </p>
            ) : whitelistQuery.isError ? (
              <div className="text-sm" style={{ color: 'var(--danger)' }}>
                Could not load whitelist — check that RCON is enabled and the password is set.
              </div>
            ) : whitelistQuery.data?.players.length === 0 ? (
              <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Whitelist is empty — all players can join.</p>
            ) : (
              <ul className="space-y-1">
                {whitelistQuery.data!.players.map(player => (
                  <li key={player}
                    className="flex items-center justify-between text-sm px-3 py-2 rounded-lg"
                    style={{ background: 'var(--surface-raised)', border: '1px solid var(--border)' }}>
                    <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{player}</span>
                    <button
                      className="btn-ghost py-1 px-2 text-xs"
                      style={{ color: 'var(--danger)' }}
                      disabled={wlRemoveMutation.isPending}
                      onClick={() => wlRemoveMutation.mutate(player)}
                    >
                      <X className="w-3 h-3 inline-block mr-1" />Remove
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </div>
  );
}

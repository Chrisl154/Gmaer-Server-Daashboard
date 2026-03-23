import React, { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Activity, Server, HardDrive, Shield, AlertTriangle, ExternalLink,
  CheckCircle2, Circle, X, ChevronDown, ChevronUp, Bell, Users,
} from 'lucide-react';
import { useNavigate, Link } from 'react-router-dom';
import { StatsCard } from '../components/Dashboard/StatsCard';
import { api } from '../utils/api';
import { ADAPTER_ICONS, ADAPTER_NAMES } from '../utils/adapters';
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer,
} from 'recharts';

interface TrendPoint { time: string; running: number; }

// Format kbps for display — shows Kbps below 1000, Mbps above.
function fmtKbps(kbps: number): string {
  if (kbps <= 0) return '0 Kbps';
  if (kbps < 1000) return `${Math.round(kbps)} Kbps`;
  return `${(kbps / 1000).toFixed(1)} Mbps`;
}

export function DashboardPage() {
  const { data: serversData, isLoading } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
    refetchInterval: 15_000,
  });

  const { data: statusData } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => api.get('/api/v1/status').then(r => r.data),
    refetchInterval: 30_000,
  });

  const servers = serversData?.servers ?? [];
  const running = servers.filter((s: any) => s.state === 'running').length;
  const stopped = servers.filter((s: any) => s.state === 'stopped').length;
  const healthy = statusData?.healthy;

  // Disk warnings: collect servers whose install partition is >85% full.
  // P54: explicit null check before numeric comparison to handle servers where
  // disk_pct is absent or null (e.g. offline servers with no metrics).
  const diskWarnServers: Array<{ name: string; pct: number }> = servers
    .filter((s: any) => s.disk_pct != null && s.disk_pct >= 85)
    .map((s: any) => ({ name: s.name, pct: Math.round(s.disk_pct) }))
    .sort((a: any, b: any) => b.pct - a.pct);

  // Accumulate running-server count history locally (no extra API call).
  const historyRef = useRef<TrendPoint[]>([]);
  const [, forceUpdate] = useState(0);
  useEffect(() => {
    if (!serversData) return;
    const point: TrendPoint = {
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      running,
    };
    historyRef.current = [...historyRef.current.slice(-19), point];
    forceUpdate(n => n + 1);
  }, [serversData]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="p-6 md:p-8 animate-page">
      {/* Disk space warning banner */}
      {diskWarnServers.length > 0 && (
        <div
          className="flex items-start gap-3 rounded-xl px-4 py-3 mb-6"
          style={{
            background: diskWarnServers.some(s => s.pct >= 95) ? 'rgba(239,68,68,0.12)' : 'rgba(249,115,22,0.12)',
            border: `1px solid ${diskWarnServers.some(s => s.pct >= 95) ? 'rgba(239,68,68,0.35)' : 'rgba(249,115,22,0.35)'}`,
          }}
        >
          <AlertTriangle
            className="w-5 h-5 shrink-0 mt-0.5"
            style={{ color: diskWarnServers.some(s => s.pct >= 95) ? '#ef4444' : '#f97316' }}
          />
          <div>
            <p className="text-sm font-semibold" style={{ color: diskWarnServers.some(s => s.pct >= 95) ? '#fca5a5' : '#fdba74' }}>
              {diskWarnServers.some(s => s.pct >= 95) ? 'Critical: Disk almost full' : 'Warning: Disk space running low'}
            </p>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
              {diskWarnServers.map(s => `${s.name} (${s.pct}%)`).join(' · ')}
              {' — '}Free up disk space to prevent server crashes.
            </p>
          </div>
        </div>
      )}

      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
          Dashboard
        </h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          {servers.length} server{servers.length !== 1 ? 's' : ''} managed &middot; System{' '}
          {healthy == null ? 'checking...' : healthy ? 'healthy' : 'degraded'}
        </p>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatsCard
          icon={Server}
          title="Total Servers"
          value={servers.length}
          color="blue"
        />
        <StatsCard
          icon={Activity}
          title="Running"
          value={running}
          color="green"
          trend={running > 0 ? 'Active' : undefined}
        />
        <StatsCard
          icon={HardDrive}
          title="Stopped"
          value={stopped}
          color="gray"
        />
        <StatsCard
          icon={Shield}
          title="System Health"
          value={healthy == null ? '...' : healthy ? 'Healthy' : 'Degraded'}
          color={healthy ? 'orange' : 'gray'}
          trend={healthy ? 'OK' : undefined}
        />
      </div>

      {/* Running servers trend chart */}
      {historyRef.current.length > 1 && (
        <div
          className="card p-5 mb-8"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                Server Activity
              </h2>
              <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                Running servers — last {historyRef.current.length} polls
              </p>
            </div>
            <span
              className="badge"
              style={{
                background: 'rgba(249,115,22,0.1)',
                color: '#fb923c',
                border: '1px solid rgba(249,115,22,0.2)',
              }}
            >
              Live
            </span>
          </div>
          <ResponsiveContainer width="100%" height={130}>
            <AreaChart
              data={historyRef.current}
              margin={{ top: 4, right: 8, bottom: 0, left: -20 }}
            >
              <defs>
                <linearGradient id="runningGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#f97316" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="time"
                tick={{ fontSize: 10, fill: 'var(--text-muted)' }}
                interval="preserveStartEnd"
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                allowDecimals={false}
                tick={{ fontSize: 10, fill: 'var(--text-muted)' }}
                width={24}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                contentStyle={{
                  background: 'var(--bg-card)',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 10,
                  fontSize: 12,
                  color: 'var(--text-primary)',
                }}
                labelStyle={{ color: 'var(--text-secondary)' }}
                cursor={{ stroke: 'rgba(249,115,22,0.3)', strokeWidth: 1 }}
              />
              <Area
                type="monotone"
                dataKey="running"
                stroke="#f97316"
                strokeWidth={2}
                fill="url(#runningGrad)"
                dot={false}
                name="Running"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Getting Started checklist — shown until dismissed */}
      <GettingStartedChecklist serverCount={servers.length} />

      {/* Resource Table */}
      <ResourceTable servers={servers} isLoading={isLoading} />
    </div>
  );
}

// ── Getting Started checklist ─────────────────────────────────────────────────

const CHECKLIST_KEY = 'gdash_checklist_dismissed';
const CHECKLIST_STEPS_KEY = 'gdash_checklist_steps';

interface ChecklistStep {
  id: string;
  label: string;
  description: string;
  link: string;
  linkLabel: string;
  icon: React.FC<any>;
}

const STEPS: ChecklistStep[] = [
  {
    id: 'server',
    label: 'Add your first game server',
    description: 'Deploy Valheim, Minecraft, Satisfactory or any of 24 supported games.',
    link: '/servers',
    linkLabel: 'Go to Servers',
    icon: Server,
  },
  {
    id: 'backup',
    label: 'Take a backup',
    description: 'Protect your world saves and config files with one click.',
    link: '/backups',
    linkLabel: 'Go to Backups',
    icon: HardDrive,
  },
  {
    id: 'notifications',
    label: 'Set up crash notifications',
    description: 'Get a Discord or Slack alert when a server crashes or disk fills up.',
    link: '/settings',
    linkLabel: 'Open Settings',
    icon: Bell,
  },
  {
    id: 'user',
    label: 'Invite a user',
    description: 'Let friends or co-admins manage servers without sharing your password.',
    link: '/settings',
    linkLabel: 'Open Settings',
    icon: Users,
  },
];

function GettingStartedChecklist({ serverCount }: { serverCount: number }) {
  const [dismissed, setDismissed] = useState<boolean>(() => {
    try { return localStorage.getItem(CHECKLIST_KEY) === '1'; } catch { return false; }
  });
  const [doneSteps, setDoneSteps] = useState<string[]>(() => {
    try {
      const raw = localStorage.getItem(CHECKLIST_STEPS_KEY);
      return raw ? JSON.parse(raw) : [];
    } catch { return []; }
  });
  const [collapsed, setCollapsed] = useState(false);

  // Auto-mark "server" step done when servers exist.
  useEffect(() => {
    if (serverCount > 0 && !doneSteps.includes('server')) {
      const next = [...doneSteps, 'server'];
      setDoneSteps(next);
      try { localStorage.setItem(CHECKLIST_STEPS_KEY, JSON.stringify(next)); } catch {}
    }
  }, [serverCount]); // eslint-disable-line react-hooks/exhaustive-deps

  const toggleStep = (id: string) => {
    const next = doneSteps.includes(id)
      ? doneSteps.filter(s => s !== id)
      : [...doneSteps, id];
    setDoneSteps(next);
    try { localStorage.setItem(CHECKLIST_STEPS_KEY, JSON.stringify(next)); } catch {}
  };

  const dismiss = () => {
    setDismissed(true);
    try { localStorage.setItem(CHECKLIST_KEY, '1'); } catch {}
  };

  const allDone = STEPS.every(s => doneSteps.includes(s.id));
  const doneCount = STEPS.filter(s => doneSteps.includes(s.id)).length;

  // Hide if manually dismissed or (all done + dismissed automatically after 1 day)
  if (dismissed) return null;

  return (
    <div
      className="card mb-8 overflow-hidden"
      style={{ border: '1px solid var(--border)' }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-5 py-3.5 cursor-pointer"
        style={{ background: 'var(--bg-elevated)', borderBottom: collapsed ? 'none' : '1px solid var(--border)' }}
        onClick={() => setCollapsed(c => !c)}
      >
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5">
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              Getting Started
            </span>
            <span
              className="text-xs px-2 py-0.5 rounded-full font-medium"
              style={{
                background: allDone ? 'rgba(34,197,94,0.15)' : 'rgba(249,115,22,0.12)',
                color: allDone ? '#4ade80' : '#fb923c',
              }}
            >
              {doneCount}/{STEPS.length}
            </span>
          </div>
          {!collapsed && (
            <p className="text-xs hidden sm:block" style={{ color: 'var(--text-muted)' }}>
              Complete these steps to get your game servers up and running.
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {collapsed
            ? <ChevronDown className="w-4 h-4" style={{ color: 'var(--text-muted)' }} />
            : <ChevronUp className="w-4 h-4" style={{ color: 'var(--text-muted)' }} />}
          <button
            onClick={e => { e.stopPropagation(); dismiss(); }}
            className="w-6 h-6 flex items-center justify-center rounded transition-colors"
            style={{ color: 'var(--text-muted)' }}
            title="Dismiss"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Steps */}
      {!collapsed && (
        <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
          {STEPS.map(step => {
            const done = doneSteps.includes(step.id);
            const Icon = step.icon;
            return (
              <div
                key={step.id}
                className="flex items-start gap-4 px-5 py-4 transition-colors"
                style={{ background: done ? 'rgba(34,197,94,0.04)' : 'transparent' }}
              >
                {/* Checkbox */}
                <button
                  onClick={() => toggleStep(step.id)}
                  className="mt-0.5 shrink-0"
                  title={done ? 'Mark as not done' : 'Mark as done'}
                >
                  {done
                    ? <CheckCircle2 className="w-5 h-5 text-green-400" />
                    : <Circle className="w-5 h-5" style={{ color: 'var(--text-muted)' }} />}
                </button>

                {/* Icon */}
                <div
                  className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                  style={{
                    background: done ? 'rgba(34,197,94,0.12)' : 'var(--bg-elevated)',
                    border: '1px solid var(--border)',
                  }}
                >
                  <Icon className="w-4 h-4" style={{ color: done ? '#4ade80' : 'var(--text-muted)' }} />
                </div>

                {/* Text */}
                <div className="flex-1 min-w-0">
                  <p
                    className="text-sm font-medium"
                    style={{
                      color: done ? 'var(--text-muted)' : 'var(--text-primary)',
                      textDecoration: done ? 'line-through' : 'none',
                    }}
                  >
                    {step.label}
                  </p>
                  {!done && (
                    <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                      {step.description}
                    </p>
                  )}
                </div>

                {/* Action link */}
                {!done && (
                  <Link
                    to={step.link}
                    className="shrink-0 text-xs px-3 py-1.5 rounded-lg transition-colors"
                    style={{
                      background: 'var(--primary-subtle)',
                      color: 'var(--primary)',
                      border: '1px solid var(--primary-border)',
                      textDecoration: 'none',
                    }}
                  >
                    {step.linkLabel}
                  </Link>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* All done banner */}
      {allDone && !collapsed && (
        <div
          className="flex items-center justify-between px-5 py-3"
          style={{ background: 'rgba(34,197,94,0.08)', borderTop: '1px solid rgba(34,197,94,0.2)' }}
        >
          <div className="flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-green-400" />
            <span className="text-sm font-medium text-green-400">You're all set! Nice work.</span>
          </div>
          <button
            onClick={dismiss}
            className="text-xs px-3 py-1.5 rounded-lg"
            style={{ background: 'rgba(34,197,94,0.15)', color: '#4ade80' }}
          >
            Dismiss
          </button>
        </div>
      )}
    </div>
  );
}

// Mini inline progress bar used in the resource table cells.
function MiniBar({ pct, color }: { pct: number; color: string }) {
  return (
    <div className="flex items-center gap-2 min-w-0">
      <div
        className="flex-1 h-1.5 rounded-full"
        style={{ background: 'var(--bg-elevated)', minWidth: 60 }}
      >
        <div
          className="h-full rounded-full transition-all duration-700"
          style={{ width: `${Math.min(100, pct)}%`, background: color }}
        />
      </div>
      <span className="text-xs font-mono w-9 text-right shrink-0" style={{ color: 'var(--text-secondary)' }}>
        {pct > 0 ? `${Math.round(pct)}%` : '—'}
      </span>
    </div>
  );
}

const STATE_COLOR: Record<string, string> = {
  running:   '#22c55e',
  stopped:   '#6b7280',
  starting:  '#f59e0b',
  stopping:  '#f59e0b',
  deploying: '#3b82f6',
  updating:  '#06b6d4',
  error:     '#ef4444',
  idle:      '#6b7280',
};
const STATE_LABEL: Record<string, string> = {
  running: 'Running', stopped: 'Stopped', starting: 'Starting',
  stopping: 'Stopping', deploying: 'Deploying', updating: 'Updating', error: 'Error', idle: 'Idle',
};

function ResourceTable({ servers, isLoading }: { servers: any[]; isLoading: boolean }) {
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <div className="card p-0 overflow-hidden">
        <div className="px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="h-5 w-40 rounded" style={{ background: 'var(--bg-elevated)' }} />
        </div>
        {[1, 2, 3].map(i => (
          <div key={i} className="px-5 py-4 flex gap-4" style={{ borderBottom: '1px solid var(--border)', opacity: 0.5 }}>
            <div className="h-4 w-32 rounded" style={{ background: 'var(--bg-elevated)' }} />
            <div className="h-4 flex-1 rounded" style={{ background: 'var(--bg-elevated)' }} />
          </div>
        ))}
      </div>
    );
  }

  if (servers.length === 0) {
    return <EmptyServers />;
  }

  return (
    <div className="card p-0 overflow-hidden">
      {/* Table header */}
      <div
        className="flex items-center justify-between px-5 py-3"
        style={{ borderBottom: '1px solid var(--border)' }}
      >
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
          Resource Overview
        </h2>
        <div className="flex items-center gap-2">
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Updates every 15 s
          </span>
          <span
            className="badge"
            style={{ background: 'rgba(249,115,22,0.1)', color: '#fb923c', border: '1px solid rgba(249,115,22,0.2)' }}
          >
            Live
          </span>
        </div>
      </div>

      {/* Column headers */}
      <div
        className="grid px-5 py-2 text-xs font-medium uppercase tracking-wide"
        style={{
          color: 'var(--text-muted)',
          borderBottom: '1px solid var(--border)',
          gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr 1fr 1fr 1fr',
          background: 'var(--bg-elevated)',
        }}
      >
        <span>Server</span>
        <span>Status</span>
        <span>CPU</span>
        <span>RAM</span>
        <span>Disk</span>
        <span>Network</span>
        <span>Players</span>
        <span>Allocated</span>
      </div>

      {/* Rows */}
      {servers.map((s: any, idx: number) => {
        const AdapterIcon = ADAPTER_ICONS[s.adapter] ?? ADAPTER_ICONS.default;
        const adapterName = ADAPTER_NAMES[s.adapter] ?? s.adapter;
        const stateColor = STATE_COLOR[s.state] ?? '#6b7280';
        const stateLabel = STATE_LABEL[s.state] ?? s.state;
        const isRunning = s.state === 'running';

        const cpuColor = s.cpu_pct >= 90 ? '#ef4444' : s.cpu_pct >= 70 ? '#f97316' : '#22c55e';
        const ramColor = s.ram_pct >= 90 ? '#ef4444' : s.ram_pct >= 70 ? '#f97316' : '#3b82f6';
        const diskColor = s.disk_pct >= 95 ? '#ef4444' : s.disk_pct >= 85 ? '#f97316' : s.disk_pct >= 70 ? '#eab308' : '#22c55e';

        const playerCount: number = s.player_count ?? -1;
        const maxPlayers: number = s.max_players ?? 0;
        const playerLabel = !isRunning || playerCount < 0
          ? '—'
          : maxPlayers > 0
            ? `${playerCount} / ${maxPlayers}`
            : String(playerCount);

        return (
          <div
            key={s.id}
            className="grid px-5 py-3 items-center cursor-pointer hover:bg-white/[0.02] transition-colors"
            style={{
              gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr 1fr 1fr 1fr',
              borderBottom: idx < servers.length - 1 ? '1px solid var(--border)' : 'none',
            }}
            onClick={() => navigate(`/servers/${s.id}`)}
          >
            {/* Server name + game */}
            <div className="flex items-center gap-2.5 min-w-0 pr-4">
              <AdapterIcon className="w-4 h-4 shrink-0" style={{ color: 'var(--text-muted)' }} />
              <div className="min-w-0">
                <div className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                  {s.name}
                </div>
                <div className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>
                  {adapterName}
                </div>
              </div>
              <ExternalLink className="w-3 h-3 shrink-0 opacity-0 group-hover:opacity-100 ml-auto" style={{ color: 'var(--text-muted)' }} />
            </div>

            {/* Status */}
            <div className="flex items-center gap-1.5">
              <span
                className="w-2 h-2 rounded-full shrink-0"
                style={{ background: stateColor, boxShadow: isRunning ? `0 0 6px ${stateColor}80` : 'none' }}
              />
              <span className="text-xs" style={{ color: stateColor }}>{stateLabel}</span>
            </div>

            {/* CPU */}
            <div className="pr-4">
              <MiniBar pct={isRunning ? (s.cpu_pct ?? 0) : 0} color={cpuColor} />
            </div>

            {/* RAM */}
            <div className="pr-4">
              <MiniBar pct={isRunning ? (s.ram_pct ?? 0) : 0} color={ramColor} />
            </div>

            {/* Disk */}
            <div className="pr-4">
              <MiniBar pct={s.disk_pct ?? 0} color={diskColor} />
            </div>

            {/* Network I/O */}
            <div className="text-xs space-y-0.5 pr-2" style={{ color: isRunning && (s.net_in_kbps || s.net_out_kbps) ? 'var(--text-secondary)' : 'var(--text-muted)', opacity: isRunning ? 1 : 0.4 }}>
              {isRunning && (s.net_in_kbps > 0 || s.net_out_kbps > 0) ? (
                <>
                  <div>↓ {fmtKbps(s.net_in_kbps ?? 0)}</div>
                  <div>↑ {fmtKbps(s.net_out_kbps ?? 0)}</div>
                </>
              ) : <span>—</span>}
            </div>

            {/* Players */}
            <div className="flex items-center gap-1.5" style={{ color: playerCount >= 0 && isRunning ? 'var(--text-primary)' : 'var(--text-muted)', opacity: playerCount < 0 || !isRunning ? 0.5 : 1 }}>
              <Users className="w-3 h-3 shrink-0" />
              <span className="text-xs font-medium">{playerLabel}</span>
            </div>

            {/* Allocated resources */}
            <div className="text-xs space-y-0.5" style={{ color: 'var(--text-muted)' }}>
              {s.resources?.cpu_cores > 0 && (
                <div>{s.resources.cpu_cores} core{s.resources.cpu_cores !== 1 ? 's' : ''}</div>
              )}
              {s.resources?.ram_gb > 0 && (
                <div>{s.resources.ram_gb} GB RAM</div>
              )}
              {s.resources?.disk_gb > 0 && (
                <div>{s.resources.disk_gb} GB disk</div>
              )}
              {!s.resources?.cpu_cores && !s.resources?.ram_gb && (
                <div style={{ color: 'var(--text-muted)' }}>—</div>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function EmptyServers() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div
        className="w-20 h-20 rounded-2xl flex items-center justify-center mb-5"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border)',
        }}
      >
        <Server className="w-10 h-10" style={{ color: 'var(--text-muted)' }} />
      </div>
      <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
        No servers yet
      </h3>
      <p className="text-sm max-w-xs mb-6" style={{ color: 'var(--text-secondary)' }}>
        Add your first game server to get started. Supports Valheim, Minecraft, Satisfactory and more.
      </p>
      <a href="/servers" className="btn-primary">
        Add Server
      </a>
    </div>
  );
}

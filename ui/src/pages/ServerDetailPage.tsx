import React, { useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft, Play, Square, RotateCcw, HardDrive, Package,
  Download, Maximize2, Cpu, MemoryStick, Clock, Activity,
  FileText, RefreshCw,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api, getWsUrl } from '../utils/api';
import { cn } from '../utils/cn';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import type { ServerMetricSample } from '../types';

type Tab = 'overview' | 'console' | 'logs' | 'backups' | 'mods' | 'ports' | 'config';

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
        </div>
      </div>

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
      </div>
    </div>
  );
}

// ── Overview tab ─────────────────────────────────────────────────────────────

function OverviewTab({ server }: { server: any }) {
  return (
    <div className="space-y-4">
      {/* Quick metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { icon: Cpu,         label: 'CPU Cores',  value: String(server.resources?.cpu_cores ?? '—')                         },
          { icon: MemoryStick, label: 'RAM',        value: server.resources?.ram_gb  ? `${server.resources.ram_gb} GB`  : '—' },
          { icon: HardDrive,   label: 'Disk',       value: server.resources?.disk_gb ? `${server.resources.disk_gb} GB` : '—' },
          { icon: Clock,       label: 'State',      value: server.state                                                        },
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

        <div className="md:col-span-2">
          <MetricsChart serverId={server.id} />
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
  const chartData = samples.map(s => ({
    time: new Date(s.ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    cpu:  parseFloat(s.cpu_pct.toFixed(1)),
    ram:  parseFloat(s.ram_pct.toFixed(1)),
  }));

  return (
    <div className="card p-5">
      <h3 className="label mb-4">Resource Utilization</h3>
      {chartData.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12">
          <Activity className="w-8 h-8 mb-2" style={{ color: 'var(--text-muted)' }} />
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
            No data yet — metrics are collected every 15 s while the server is running.
          </p>
        </div>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.04)" />
              <XAxis dataKey="time" tick={{ fontSize: 10, fill: 'var(--text-muted)' }} interval="preserveStartEnd" />
              <YAxis domain={[0, 100]} tick={{ fontSize: 10, fill: 'var(--text-muted)' }} unit="%" />
              <Tooltip
                contentStyle={{
                  background: 'var(--bg-elevated)',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 10,
                  fontSize: 12,
                }}
                labelStyle={{ color: 'var(--text-secondary)' }}
              />
              <Line type="monotone" dataKey="cpu" stroke="#f97316" strokeWidth={2} dot={false} name="CPU %" />
              <Line type="monotone" dataKey="ram" stroke="#3b82f6" strokeWidth={2} dot={false} name="RAM %" />
            </LineChart>
          </ResponsiveContainer>
          <div className="flex gap-4 mt-3">
            <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
              <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#f97316' }} /> CPU %
            </span>
            <span className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-muted)' }}>
              <span className="w-3 h-0.5 rounded inline-block" style={{ background: '#3b82f6' }} /> RAM %
            </span>
          </div>
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
  return (
    <div>
      <h3 className="text-sm font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>Configuration</h3>
      <div className="card overflow-hidden">
        <div className="px-4 py-2.5 flex items-center gap-2"
          style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
          <div className="flex items-center gap-1.5">
            <div className="w-2.5 h-2.5 rounded-full bg-red-500/70" />
            <div className="w-2.5 h-2.5 rounded-full bg-yellow-500/70" />
            <div className="w-2.5 h-2.5 rounded-full bg-green-500/70" />
          </div>
          <span className="text-xs font-mono ml-2" style={{ color: 'var(--text-muted)' }}>config.json</span>
        </div>
        <pre className="p-5 text-xs font-mono overflow-auto max-h-96"
          style={{ background: '#080810', color: 'var(--text-primary)' }}>
          {JSON.stringify(server.config ?? {}, null, 2)}
        </pre>
      </div>
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

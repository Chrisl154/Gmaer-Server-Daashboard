import React, { useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Play, Square, RotateCcw, HardDrive, Package, Download, Maximize2 } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api, getWsUrl } from '../utils/api';
import { clsx } from 'clsx';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts';
import type { ServerMetricSample } from '../types';

type Tab = 'overview' | 'console' | 'backups' | 'mods' | 'ports' | 'config';

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
    return <div className="p-6"><div className="h-64 bg-gray-800 rounded-xl animate-pulse" /></div>;
  }
  if (!server) {
    return (
      <div className="p-6">
        <Link to="/" className="text-gray-400 hover:text-gray-100 flex items-center gap-2 text-sm mb-4">
          <ArrowLeft className="w-4 h-4" /> Back
        </Link>
        <p className="text-gray-400">Server not found.</p>
      </div>
    );
  }

  const isRunning = server.state === 'running';
  const isBusy = ['starting', 'stopping', 'deploying'].includes(server.state);
  const TABS: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'console',  label: 'Console'  },
    { id: 'backups',  label: 'Backups'  },
    { id: 'mods',     label: 'Mods'     },
    { id: 'ports',    label: 'Ports'    },
    { id: 'config',   label: 'Config'   },
  ];

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/" className="text-gray-400 hover:text-gray-100 flex items-center gap-1.5 text-sm">
          <ArrowLeft className="w-4 h-4" /> Dashboard
        </Link>
        <span className="text-gray-600">/</span>
        <span className="text-gray-100 text-sm font-medium">{server.name}</span>
      </div>

      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-gray-100">{server.name}</h1>
          <p className="text-sm text-gray-400 mt-0.5 capitalize">{server.adapter} · {server.state}</p>
        </div>
        <div className="flex items-center gap-2">
          {isRunning ? (
            <>
              <button onClick={() => stopMutation.mutate()} disabled={isBusy}
                className="flex items-center gap-1.5 px-3 py-2 text-sm bg-red-900/30 hover:bg-red-900/50 text-red-400 rounded-lg transition-colors disabled:opacity-50">
                <Square className="w-4 h-4" /> Stop
              </button>
              <button onClick={() => restartMutation.mutate()} disabled={isBusy}
                className="flex items-center gap-1.5 px-3 py-2 text-sm bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg transition-colors disabled:opacity-50">
                <RotateCcw className="w-4 h-4" /> Restart
              </button>
            </>
          ) : (
            <button onClick={() => startMutation.mutate()} disabled={isBusy}
              className="flex items-center gap-1.5 px-3 py-2 text-sm bg-green-900/30 hover:bg-green-900/50 text-green-400 rounded-lg transition-colors disabled:opacity-50">
              <Play className="w-4 h-4" /> Start
            </button>
          )}
          <button onClick={() => api.post(`/api/v1/servers/${id}/backup`, { type: 'full' }).then(() => toast.success('Backup triggered'))}
            className="flex items-center gap-1.5 px-3 py-2 text-sm bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg transition-colors">
            <HardDrive className="w-4 h-4" /> Backup
          </button>
        </div>
      </div>

      <div className="flex gap-1 border-b border-[#1a1a1a]">
        {TABS.map(tab => (
          <button key={tab.id} onClick={() => setActiveTab(tab.id)}
            className={clsx('px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px',
              activeTab === tab.id ? 'text-blue-400 border-blue-400' : 'text-gray-400 border-transparent hover:text-gray-200')}>
            {tab.label}
          </button>
        ))}
      </div>

      <div>
        {activeTab === 'overview' && <OverviewTab server={server} />}
        {activeTab === 'console'  && <ConsoleTab serverId={id!} />}
        {activeTab === 'backups'  && <BackupsTab serverId={id!} />}
        {activeTab === 'mods'     && <ModsTab serverId={id!} />}
        {activeTab === 'ports'    && <PortsTab server={server} />}
        {activeTab === 'config'   && <ConfigTab server={server} />}
      </div>
    </div>
  );
}

function OverviewTab({ server }: { server: any }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      <InfoCard title="Server Details">
        <InfoRow label="ID" value={server.id} mono />
        <InfoRow label="Adapter" value={server.adapter} />
        <InfoRow label="Deploy Method" value={server.deploy_method || 'steamcmd'} />
        <InfoRow label="State" value={server.state} />
        <InfoRow label="Install Dir" value={server.install_dir || '/opt/games'} mono />
        {server.container_id && (
          <InfoRow label="Container ID" value={server.container_id.slice(0, 12)} mono />
        )}
      </InfoCard>
      <InfoCard title="Resources">
        <InfoRow label="CPU Cores" value={String(server.resources?.cpu_cores ?? '—')} />
        <InfoRow label="RAM" value={server.resources?.ram_gb ? `${server.resources.ram_gb} GB` : '—'} />
        <InfoRow label="Disk" value={server.resources?.disk_gb ? `${server.resources.disk_gb} GB` : '—'} />
      </InfoCard>
      <div className="md:col-span-2">
        <MetricsChart serverId={server.id} />
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
    <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
      <h3 className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-3">
        Resource Utilization
      </h3>
      {chartData.length === 0 ? (
        <p className="text-sm text-gray-500 py-8 text-center">
          No data yet — metrics are collected every 15 s while the server is running.
        </p>
      ) : (
        <>
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1e1e1e" />
              <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#6b7280' }} interval="preserveStartEnd" />
              <YAxis domain={[0, 100]} tick={{ fontSize: 10, fill: '#6b7280' }} unit="%" />
              <Tooltip
                contentStyle={{ background: '#141414', border: '1px solid #252525', borderRadius: 8, fontSize: 12 }}
                labelStyle={{ color: '#9ca3af' }}
              />
              <Line type="monotone" dataKey="cpu" stroke="#3b82f6" strokeWidth={1.5} dot={false} name="CPU %" />
              <Line type="monotone" dataKey="ram" stroke="#8b5cf6" strokeWidth={1.5} dot={false} name="RAM %" />
            </LineChart>
          </ResponsiveContainer>
          <div className="flex gap-4 mt-2 text-xs text-gray-500">
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-0.5 bg-blue-500 inline-block rounded" /> CPU %
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-0.5 bg-purple-500 inline-block rounded" /> RAM %
            </span>
          </div>
        </>
      )}
    </div>
  );
}

function ConsoleTab({ serverId }: { serverId: string }) {
  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [lines, setLines] = useState<string[]>(['Connecting to console...']);
  const [connected, setConnected] = useState(false);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const historyRef = useRef<string[]>([]);
  const historyIdx = useRef(-1);

  useEffect(() => {
    const ws = new WebSocket(getWsUrl(`/api/v1/servers/${serverId}/console/stream`));
    wsRef.current = ws;
    ws.onopen = () => { setConnected(true); setLines(p => [...p, '— Connected —']); };
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
  }, [serverId]);

  useEffect(() => { if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight; }, [lines]);

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
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className={clsx('w-2 h-2 rounded-full', connected ? 'bg-green-400' : 'bg-gray-600')} />
          <span className="text-xs text-gray-400">{connected ? 'Connected' : 'Disconnected'}</span>
        </div>
        <button className="p-1.5 text-gray-500 hover:text-gray-300"><Maximize2 className="w-4 h-4" /></button>
      </div>
      <div ref={logRef}
        className="bg-[#0d0d0d] border border-[#1a1a1a] rounded-xl p-4 h-80 overflow-y-auto font-mono text-xs text-gray-300 space-y-0.5">
        {lines.map((line, i) => (
          <div key={i} className={clsx(
            'whitespace-pre-wrap break-all leading-5',
            line.startsWith('> ') && 'text-blue-400',
            line.startsWith('← ') && 'text-green-400',
            line.startsWith('[error]') && 'text-red-400',
          )}>{line}</div>
        ))}
      </div>
      <div className="flex gap-2">
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter RCON command… (↑↓ history)"
          disabled={!connected || sending}
          className="flex-1 bg-[#0d0d0d] border border-[#1a1a1a] rounded-lg px-3 py-2 text-sm
                     text-gray-100 font-mono placeholder-gray-600
                     focus:outline-none focus:border-blue-500 disabled:opacity-40"
        />
        <button
          onClick={sendCommand}
          disabled={!connected || sending || !input.trim()}
          className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-700 text-white
                     rounded-lg disabled:opacity-40 transition-colors">
          {sending ? '…' : 'Send'}
        </button>
      </div>
      <p className="text-xs text-gray-600">
        Requires RCON on the adapter + <code className="font-mono">rcon_password</code> in server config.
      </p>
    </div>
  );
}

function BackupsTab({ serverId }: { serverId: string }) {
  const { data } = useQuery({
    queryKey: ['backups', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/backups`).then(r => r.data),
  });
  const backups = data?.backups ?? [];
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-200">Backups ({backups.length})</h3>
        <button onClick={() => api.post(`/api/v1/servers/${serverId}/backup`, { type: 'full' }).then(() => toast.success('Backup started'))}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg">
          <Download className="w-3 h-3" /> New Backup
        </button>
      </div>
      {backups.length === 0 ? <p className="text-gray-500 text-sm">No backups yet.</p> : (
        <div className="space-y-2">
          {backups.map((b: any) => (
            <div key={b.id} className="bg-[#141414] border border-[#252525] rounded-lg p-3 flex items-center justify-between">
              <div><span className="text-sm text-gray-200 font-mono">{b.id}</span><span className="ml-2 text-xs text-gray-500">{b.type}</span></div>
              <div className="flex items-center gap-3">
                <span className="text-xs text-gray-400">{b.created_at}</span>
                <button onClick={() => api.post(`/api/v1/servers/${serverId}/restore/${b.id}`).then(() => toast.success('Restore started'))}
                  className="text-xs text-blue-400 hover:text-blue-300">Restore</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

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
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-200">Mods ({mods.length})</h3>
        <div className="flex gap-2">
          <button
            onClick={() => runTests.mutate()}
            disabled={runTests.isPending}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-green-600/20 hover:bg-green-600/30 text-green-400 rounded-lg disabled:opacity-50 transition-colors"
          >
            {runTests.isPending ? 'Running…' : 'Run Tests'}
          </button>
          <button className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg">
            <Package className="w-3 h-3" /> Add Mod
          </button>
        </div>
      </div>

      {/* Test Results */}
      {testResult && (
        <div className={clsx(
          'border rounded-xl p-4 space-y-3',
          testResult.passed ? 'bg-green-950/20 border-green-800/40' : 'bg-red-950/20 border-red-800/40',
        )}>
          <div className="flex items-center justify-between">
            <span className={clsx('text-sm font-medium', testResult.passed ? 'text-green-400' : 'text-red-400')}>
              {testResult.passed ? 'All tests passed' : 'Some tests failed'}
            </span>
            <span className="text-xs text-gray-500">{Math.round(testResult.duration / 1e6)} ms</span>
          </div>
          <div className="space-y-1.5">
            {testResult.tests.map((t) => (
              <div key={t.name} className="flex items-start gap-2 text-xs">
                <span className={clsx('mt-0.5 shrink-0', t.passed ? 'text-green-400' : 'text-red-400')}>
                  {t.passed ? '✓' : '✗'}
                </span>
                <span className="font-mono text-gray-400 w-36 shrink-0">{t.name}</span>
                <span className="text-gray-500">{t.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Mod list */}
      {mods.length === 0 ? <p className="text-gray-500 text-sm">No mods installed.</p> : (
        <div className="space-y-2">
          {mods.map((m: any) => (
            <div key={m.id} className="bg-[#141414] border border-[#252525] rounded-lg p-3 flex items-center justify-between">
              <div>
                <span className="text-sm text-gray-200">{m.name}</span>
                <span className="ml-2 text-xs text-gray-500 font-mono">{m.version}</span>
              </div>
              <span className="text-xs text-gray-500 capitalize">{m.source}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function PortsTab({ server }: { server: any }) {
  const ports = server.ports ?? [];
  return (
    <div className="space-y-3">
      <h3 className="text-sm font-medium text-gray-200">Port Mappings ({ports.length})</h3>
      {ports.length === 0 ? <p className="text-gray-500 text-sm">No ports configured.</p> : (
        <div className="space-y-2">
          {ports.map((p: any, i: number) => (
            <div key={i} className="bg-[#141414] border border-[#252525] rounded-lg p-3 flex items-center justify-between">
              <div className="font-mono text-sm">
                <span className="text-gray-400">{p.internal}</span>
                <span className="text-gray-600 mx-2">→</span>
                <span className="text-gray-200">{p.external}</span>
                <span className="text-gray-500 ml-2 text-xs">{p.protocol}</span>
              </div>
              <div className={clsx('text-xs', p.exposed ? 'text-green-400' : 'text-gray-500')}>
                {p.exposed ? 'Exposed' : 'Internal'}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ConfigTab({ server }: { server: any }) {
  return (
    <div>
      <h3 className="text-sm font-medium text-gray-200 mb-3">Configuration</h3>
      <pre className="bg-[#0d0d0d] border border-[#1a1a1a] rounded-xl p-4 text-xs text-gray-300 font-mono overflow-auto max-h-96">
        {JSON.stringify(server.config ?? {}, null, 2)}
      </pre>
    </div>
  );
}

function InfoCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
      <h3 className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-3">{title}</h3>
      <div className="space-y-2">{children}</div>
    </div>
  );
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-gray-500">{label}</span>
      <span className={clsx('text-gray-200', mono && 'font-mono text-xs')}>{value}</span>
    </div>
  );
}

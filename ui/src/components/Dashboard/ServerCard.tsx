import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Play, Square, RotateCcw, Terminal } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'react-hot-toast';
import { api } from '../../utils/api';
import { ADAPTER_ICONS, ADAPTER_COLORS, ADAPTER_NAMES } from '../../utils/adapters';
import { cn } from '../../utils/cn';

interface Server {
  id: string;
  name: string;
  adapter: string;
  state: string;
  deploy_method?: string;
  ports: Array<{ internal: number; external: number; protocol: string }>;
  resources: { cpu_cores: number; ram_gb: number; disk_gb: number };
  last_started?: string;
}

const STATE_CLASS: Record<string, string> = {
  running:   'status-running',
  stopped:   'status-stopped',
  starting:  'status-starting',
  stopping:  'status-stopping',
  deploying: 'status-deploying',
  error:     'status-error',
  idle:      'status-idle',
};

const STATE_LABELS: Record<string, string> = {
  running:   'Running',
  stopped:   'Stopped',
  starting:  'Starting',
  stopping:  'Stopping',
  deploying: 'Deploying',
  error:     'Error',
  idle:      'Idle',
};

export function ServerCard({ server }: { server: Server }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const startMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/start`),
    onSuccess: () => {
      toast.success(`Starting ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to start server'),
  });

  const stopMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/stop`),
    onSuccess: () => {
      toast.success(`Stopping ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to stop server'),
  });

  const restartMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/restart`),
    onSuccess: () => {
      toast.success(`Restarting ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to restart server'),
  });

  const isRunning = server.state === 'running';
  const isBusy = ['starting', 'stopping', 'deploying'].includes(server.state);
  const AdapterIcon = ADAPTER_ICONS[server.adapter] ?? ADAPTER_ICONS.default;
  const adapterColor = ADAPTER_COLORS[server.adapter] ?? '#6b7280';
  const adapterName = ADAPTER_NAMES[server.adapter] ?? server.adapter;
  const statusClass = STATE_CLASS[server.state] ?? 'status-stopped';
  const statusLabel = STATE_LABELS[server.state] ?? server.state;

  // Mock CPU/RAM percentages based on resources for display
  const cpuPct = isRunning ? Math.min(95, (server.resources?.cpu_cores ?? 1) * 12 + 15) : 0;
  const ramPct = isRunning ? Math.min(90, (server.resources?.ram_gb ?? 1) * 8 + 20) : 0;

  return (
    <div
      className="card p-5 flex flex-col gap-4 cursor-pointer group"
      style={{ position: 'relative', overflow: 'hidden' }}
      onClick={() => navigate(`/servers/${server.id}`)}
    >
      {/* Orange top border accent on hover */}
      <div
        className="absolute top-0 left-0 right-0 h-[2px] opacity-0 group-hover:opacity-100 transition-opacity duration-200"
        style={{ background: 'linear-gradient(90deg, #f97316, #ea580c)' }}
      />

      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          {/* Game icon */}
          <div
            className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
            style={{
              background: `${adapterColor}22`,
              border: `1px solid ${adapterColor}40`,
            }}
          >
            {/* Wrap in a span so we can set color via CSS without touching the icon type */}
            <span style={{ color: adapterColor, display: 'flex' }}>
              <AdapterIcon className="w-5 h-5" />
            </span>
          </div>
          <div className="min-w-0">
            <div
              className="font-semibold text-sm truncate"
              style={{ color: 'var(--text-primary)' }}
            >
              {server.name}
            </div>
            <div className="text-xs truncate mt-0.5" style={{ color: 'var(--text-secondary)' }}>
              {adapterName}
            </div>
          </div>
        </div>

        {/* Status badge */}
        <span
          className={cn('badge shrink-0', statusClass)}
          style={{ animationPlayState: isBusy ? 'running' : 'paused' }}
        >
          <span
            className={cn(
              'w-1.5 h-1.5 rounded-full',
              isBusy && 'animate-pulse'
            )}
            style={{ background: 'currentColor' }}
          />
          {statusLabel}
        </span>
      </div>

      {/* Deploy method + ports */}
      <div className="flex flex-wrap items-center gap-2">
        {server.deploy_method && (
          <span
            className="text-xs px-2 py-0.5 rounded-md font-medium capitalize"
            style={{
              background: 'var(--bg-elevated)',
              color: 'var(--text-secondary)',
              border: '1px solid var(--border)',
            }}
          >
            {server.deploy_method}
          </span>
        )}
        {server.ports?.slice(0, 3).map((p, i) => (
          <span
            key={i}
            className="text-xs px-2 py-0.5 rounded-md font-mono"
            style={{
              background: 'rgba(59,130,246,0.1)',
              color: '#60a5fa',
              border: '1px solid rgba(59,130,246,0.2)',
            }}
          >
            {p.external}/{p.protocol}
          </span>
        ))}
        {server.ports?.length > 3 && (
          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            +{server.ports.length - 3}
          </span>
        )}
      </div>

      {/* CPU / RAM progress bars */}
      <div className="space-y-2.5">
        <div>
          <div className="flex justify-between items-center mb-1">
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>CPU</span>
            <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              {isRunning ? `${cpuPct}%` : '—'}
            </span>
          </div>
          <div
            className="h-1 rounded-full w-full"
            style={{ background: 'var(--bg-elevated)' }}
          >
            <div
              className="h-full rounded-full transition-all duration-700"
              style={{
                width: `${cpuPct}%`,
                background: 'linear-gradient(90deg, #f97316, #ea580c)',
              }}
            />
          </div>
        </div>
        <div>
          <div className="flex justify-between items-center mb-1">
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>RAM</span>
            <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              {isRunning ? `${ramPct}%` : '—'}
            </span>
          </div>
          <div
            className="h-1 rounded-full w-full"
            style={{ background: 'var(--bg-elevated)' }}
          >
            <div
              className="h-full rounded-full transition-all duration-700"
              style={{
                width: `${ramPct}%`,
                background: 'linear-gradient(90deg, #f97316, #ea580c)',
              }}
            />
          </div>
        </div>
      </div>

      {/* Action buttons footer */}
      <div
        className="flex items-center gap-2 pt-2"
        style={{ borderTop: '1px solid var(--border)' }}
        onClick={e => e.stopPropagation()}
      >
        {isRunning ? (
          <>
            <button
              onClick={() => stopMutation.mutate()}
              disabled={isBusy || stopMutation.isPending}
              className="btn-danger p-2"
              style={{ padding: '6px' }}
              title="Stop server"
            >
              <Square className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => restartMutation.mutate()}
              disabled={isBusy || restartMutation.isPending}
              className="btn-ghost p-2"
              style={{ padding: '6px' }}
              title="Restart server"
            >
              <RotateCcw className="w-3.5 h-3.5" />
            </button>
          </>
        ) : (
          <button
            onClick={() => startMutation.mutate()}
            disabled={isBusy || startMutation.isPending}
            className="btn-primary p-2"
            style={{ padding: '6px', background: 'linear-gradient(135deg, #22c55e, #16a34a)', boxShadow: '0 2px 8px rgba(34,197,94,0.3)' }}
            title="Start server"
          >
            <Play className="w-3.5 h-3.5" />
          </button>
        )}

        <Link
          to={`/servers/${server.id}#console`}
          className="btn-blue ml-auto"
          style={{ padding: '6px 12px' }}
          title="Open console"
        >
          <Terminal className="w-3.5 h-3.5" />
          <span className="text-xs">Console</span>
        </Link>
      </div>
    </div>
  );
}

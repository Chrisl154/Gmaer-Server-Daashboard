import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Play, Square, RotateCcw, Terminal, AlertTriangle, X, HelpCircle } from 'lucide-react';
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
  last_error?: string;
  restart_count?: number;
  last_crash_at?: string;
  disk_pct?: number;
}

const STATE_CLASS: Record<string, string> = {
  running:   'status-running',
  stopped:   'status-stopped',
  starting:  'status-starting',
  stopping:  'status-stopping',
  deploying: 'status-deploying',
  updating:  'status-updating',
  error:     'status-error',
  idle:      'status-idle',
};

const STATE_LABELS: Record<string, string> = {
  running:   'Running',
  stopped:   'Stopped',
  starting:  'Starting',
  stopping:  'Stopping',
  deploying: 'Deploying',
  updating:  'Updating',
  error:     'Error',
  idle:      'Idle',
};

function ErrorHelpModal({ message, onClose }: { message: string; onClose: () => void }) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.6)' }}
      onClick={onClose}
    >
      <div
        className="relative max-w-md w-full rounded-xl p-6 shadow-2xl"
        style={{ background: 'var(--bg-card)', border: '1px solid var(--border)' }}
        onClick={e => e.stopPropagation()}
      >
        <button
          onClick={onClose}
          className="absolute top-4 right-4 btn-ghost p-1"
          style={{ padding: '4px' }}
        >
          <X className="w-4 h-4" />
        </button>
        <div className="flex items-start gap-3 mb-4">
          <AlertTriangle className="w-5 h-5 shrink-0 mt-0.5" style={{ color: '#f87171' }} />
          <h3 className="font-semibold" style={{ color: 'var(--text-primary)' }}>What does this mean?</h3>
        </div>
        <p className="text-sm leading-relaxed mb-4" style={{ color: 'var(--text-secondary)' }}>
          {message}
        </p>
        <div className="text-xs p-3 rounded-lg" style={{ background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
          <strong style={{ color: 'var(--text-secondary)' }}>Next steps:</strong> Check the Console tab for detailed
          output, then try Deploy → Start again. If the problem persists, verify your install directory has enough
          disk space and the correct file permissions.
        </div>
      </div>
    </div>
  );
}

export function ServerCard({ server }: { server: Server }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [showErrorHelp, setShowErrorHelp] = useState(false);

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
  const isBusy = ['starting', 'stopping', 'deploying', 'updating'].includes(server.state);
  const AdapterIcon = ADAPTER_ICONS[server.adapter] ?? ADAPTER_ICONS.default;
  const adapterColor = ADAPTER_COLORS[server.adapter] ?? '#6b7280';
  const adapterName = ADAPTER_NAMES[server.adapter] ?? server.adapter;
  const statusClass = STATE_CLASS[server.state] ?? 'status-stopped';
  const statusLabel = STATE_LABELS[server.state] ?? server.state;

  // Mock CPU/RAM percentages based on resources for display
  const cpuPct = isRunning ? Math.min(95, (server.resources?.cpu_cores ?? 1) * 12 + 15) : 0;
  const ramPct = isRunning ? Math.min(90, (server.resources?.ram_gb ?? 1) * 8 + 20) : 0;
  const diskPct = server.disk_pct ?? 0;
  const diskColor = diskPct >= 95 ? '#ef4444' : diskPct >= 85 ? '#f97316' : diskPct >= 70 ? '#eab308' : '#22c55e';

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

      {/* Error banner — shown only in error state */}
      {server.state === 'error' && server.last_error && (
        <div
          className="flex items-start gap-2 rounded-lg px-3 py-2 text-xs"
          style={{
            background: 'rgba(239,68,68,0.1)',
            border: '1px solid rgba(239,68,68,0.25)',
            color: '#fca5a5',
          }}
        >
          <AlertTriangle className="w-3.5 h-3.5 shrink-0 mt-0.5" />
          <span className="flex-1 line-clamp-2">{server.last_error}</span>
          <button
            onClick={e => { e.stopPropagation(); setShowErrorHelp(true); }}
            title="What does this mean?"
            style={{ color: '#fb923c', flexShrink: 0 }}
          >
            <HelpCircle className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

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

      {/* CPU / RAM / Disk progress bars */}
      <div className="space-y-2.5">
        {[
          { label: 'CPU', pct: cpuPct, show: isRunning, color: 'linear-gradient(90deg, #f97316, #ea580c)' },
          { label: 'RAM', pct: ramPct, show: isRunning, color: 'linear-gradient(90deg, #f97316, #ea580c)' },
          { label: 'Disk', pct: diskPct, show: diskPct > 0, color: diskColor },
        ].map(({ label, pct, show, color }) => (
          <div key={label}>
            <div className="flex justify-between items-center mb-1">
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>{label}</span>
              <span className="text-xs font-medium" style={{ color: label === 'Disk' && diskPct >= 85 ? diskColor : 'var(--text-secondary)' }}>
                {show ? `${Math.round(pct)}%` : '—'}
              </span>
            </div>
            <div className="h-1 rounded-full w-full" style={{ background: 'var(--bg-elevated)' }}>
              <div
                className="h-full rounded-full transition-all duration-700"
                style={{ width: show ? `${pct}%` : '0%', background: color }}
              />
            </div>
          </div>
        ))}
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

      {showErrorHelp && server.last_error && (
        <ErrorHelpModal message={server.last_error} onClose={() => setShowErrorHelp(false)} />
      )}
    </div>
  );
}

import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  HardDrive, Download, RefreshCw, CheckCircle, XCircle, Clock,
  Loader2, ChevronDown, ChevronRight, Trash2, RotateCcw,
} from 'lucide-react';
import { cn } from '../utils/cn';
import { api } from '../utils/api';
import { useTriggerBackup, useRestoreBackup } from '../hooks/useServers';
import type { Backup, Server } from '../types';
import { ADAPTER_COLORS, ADAPTER_ICONS, ADAPTER_NAMES } from '../utils/adapters';

const STATUS_CONFIG: Record<
  string,
  { icon: React.ReactNode; label: string; badgeClass: string }
> = {
  complete: {
    icon: <CheckCircle className="w-3.5 h-3.5" style={{ color: '#4ade80' }} />,
    label: 'Complete',
    badgeClass: 'status-running',
  },
  failed: {
    icon: <XCircle className="w-3.5 h-3.5" style={{ color: '#f87171' }} />,
    label: 'Failed',
    badgeClass: 'status-error',
  },
  running: {
    icon: <Loader2 className="w-3.5 h-3.5 animate-spin" style={{ color: '#60a5fa' }} />,
    label: 'Running',
    badgeClass: 'status-deploying',
  },
  pending: {
    icon: <Clock className="w-3.5 h-3.5" style={{ color: '#94a3b8' }} />,
    label: 'Pending',
    badgeClass: 'status-idle',
  },
};

function formatBytes(bytes: number): string {
  if (!bytes) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
}

function formatDate(iso: string) {
  if (!iso) return '—';
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

function ServerBackupCard({ server }: { server: Server }) {
  const [expanded, setExpanded] = useState(false);

  const { data, isLoading, refetch } = useQuery<{ backups: Backup[] }>({
    queryKey: ['backups', server.id],
    queryFn: () => api.get(`/api/v1/servers/${server.id}/backups`).then(r => r.data),
    enabled: expanded,
  });

  const triggerMutation = useTriggerBackup(server.id);
  const restoreMutation = useRestoreBackup(server.id);

  const backups = data?.backups ?? [];
  const AdapterIcon = ADAPTER_ICONS[(server as any).adapter] ?? ADAPTER_ICONS.default;
  const adapterColor = ADAPTER_COLORS[(server as any).adapter] ?? '#6b7280';
  const adapterName = ADAPTER_NAMES[(server as any).adapter] ?? (server as any).adapter;

  return (
    <div className="card overflow-hidden">
      {/* Accordion Header */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer transition-colors"
        style={{ userSelect: 'none' }}
        onClick={() => setExpanded(e => !e)}
        onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
        onMouseLeave={e => (e.currentTarget.style.background = '')}
      >
        <div className="flex items-center gap-3 min-w-0">
          {/* Chevron */}
          <div style={{ color: 'var(--text-muted)' }}>
            {expanded
              ? <ChevronDown className="w-4 h-4" />
              : <ChevronRight className="w-4 h-4" />}
          </div>

          {/* Adapter icon */}
          <div
            className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
            style={{
              background: `${adapterColor}22`,
              border: `1px solid ${adapterColor}40`,
            }}
          >
            <span style={{ color: adapterColor, display: 'flex' }}>
              <AdapterIcon className="w-4 h-4" />
            </span>
          </div>

          <div className="min-w-0">
            <div className="text-sm font-semibold truncate" style={{ color: 'var(--text-primary)' }}>
              {server.name}
            </div>
            <div className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
              {adapterName}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0 ml-4">
          {expanded && backups.length > 0 && (
            <span
              className="badge"
              style={{
                background: 'rgba(249,115,22,0.1)',
                color: '#fb923c',
                border: '1px solid rgba(249,115,22,0.2)',
              }}
            >
              {backups.length} backup{backups.length !== 1 ? 's' : ''}
            </span>
          )}

          <button
            onClick={e => {
              e.stopPropagation();
              refetch();
            }}
            className="btn-ghost p-2"
            style={{ padding: '5px' }}
            title="Refresh"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>

          <button
            onClick={e => {
              e.stopPropagation();
              triggerMutation.mutate('full');
            }}
            disabled={triggerMutation.isPending}
            className="btn-blue"
            style={{ padding: '6px 12px', fontSize: 12 }}
          >
            {triggerMutation.isPending ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Download className="w-3.5 h-3.5" />
            )}
            Backup Now
          </button>
        </div>
      </div>

      {/* Expanded backup list */}
      {expanded && (
        <div
          style={{ borderTop: '1px solid var(--border)' }}
        >
          {isLoading ? (
            <div
              className="flex items-center gap-3 p-5"
              style={{ color: 'var(--text-secondary)' }}
            >
              <Loader2 className="w-4 h-4 animate-spin" />
              <span className="text-sm">Loading backups...</span>
            </div>
          ) : backups.length === 0 ? (
            <div className="p-6 text-center">
              <div
                className="w-12 h-12 rounded-xl flex items-center justify-center mx-auto mb-3"
                style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
              >
                <HardDrive className="w-6 h-6" style={{ color: 'var(--text-muted)' }} />
              </div>
              <p className="text-sm font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                No backups yet
              </p>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                Click &ldquo;Backup Now&rdquo; to create the first backup for this server.
              </p>
            </div>
          ) : (
            <div className="p-4 space-y-2">
              {/* Table header */}
              <div
                className="grid text-xs font-semibold uppercase tracking-widest px-3 py-2 rounded-lg mb-1"
                style={{
                  color: 'var(--text-muted)',
                  background: 'var(--bg-elevated)',
                  gridTemplateColumns: '1fr 80px 80px 90px 100px',
                }}
              >
                <span>Backup ID</span>
                <span>Type</span>
                <span>Size</span>
                <span>Date</span>
                <span className="text-right">Actions</span>
              </div>

              {backups.map(b => {
                const status = STATUS_CONFIG[b.status] ?? STATUS_CONFIG.pending;
                return (
                  <div
                    key={b.id}
                    className="grid items-center px-3 py-2.5 rounded-xl transition-colors"
                    style={{
                      gridTemplateColumns: '1fr 80px 80px 90px 100px',
                      background: 'var(--bg-elevated)',
                      border: '1px solid var(--border)',
                    }}
                  >
                    {/* ID + status */}
                    <div className="flex items-center gap-2 min-w-0">
                      {status.icon}
                      <span
                        className="text-xs font-mono truncate"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {b.id.slice(0, 12)}…
                      </span>
                    </div>

                    {/* Type */}
                    <span
                      className={cn('badge w-fit', b.type === 'full' ? 'status-deploying' : 'status-idle')}
                      style={{ fontSize: 10 }}
                    >
                      {b.type}
                    </span>

                    {/* Size */}
                    <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {formatBytes(b.size_bytes)}
                    </span>

                    {/* Date */}
                    <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {formatDate(b.created_at)}
                    </span>

                    {/* Actions */}
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => restoreMutation.mutate(b.id)}
                        disabled={restoreMutation.isPending || b.status !== 'complete'}
                        className="btn-ghost p-1.5 disabled:opacity-40 disabled:cursor-not-allowed"
                        style={{ padding: '4px 8px', fontSize: 11, gap: 4 }}
                        title="Restore this backup"
                      >
                        <RotateCcw className="w-3 h-3" />
                        Restore
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function BackupsPage() {
  const { data, isLoading } = useQuery<{ servers: Server[] }>({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });

  const servers = data?.servers ?? [];

  return (
    <div className="p-6 md:p-8 animate-page">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
          Backups
        </h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Manage scheduled and on-demand backups. Click a server to view its backup history.
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-3 max-w-3xl">
          {[1, 2, 3].map(i => (
            <div
              key={i}
              className="card"
              style={{ height: 68, opacity: 0.5, animation: 'pulse 1.5s ease-in-out infinite' }}
            />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <div className="flex flex-col items-center py-20 text-center">
          <div
            className="w-20 h-20 rounded-2xl flex items-center justify-center mb-5"
            style={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border)',
            }}
          >
            <HardDrive className="w-10 h-10" style={{ color: 'var(--text-muted)' }} />
          </div>
          <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
            No servers found
          </h3>
          <p className="text-sm max-w-xs" style={{ color: 'var(--text-secondary)' }}>
            Add a server first to start managing backups.
          </p>
        </div>
      ) : (
        <div className="space-y-3 max-w-3xl">
          {/* Section header */}
          <div
            className="flex items-center gap-3 pb-4 mb-2"
            style={{ borderBottom: '1px solid var(--border)' }}
          >
            <h2 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              Server Backups
            </h2>
            <span
              className="badge"
              style={{
                background: 'rgba(249,115,22,0.1)',
                color: '#fb923c',
                border: '1px solid rgba(249,115,22,0.2)',
              }}
            >
              {servers.length} server{servers.length !== 1 ? 's' : ''}
            </span>
          </div>

          {servers.map(s => (
            <ServerBackupCard key={s.id} server={s} />
          ))}
        </div>
      )}
    </div>
  );
}

import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { HardDrive, Download, RefreshCw, CheckCircle, XCircle, Clock, Loader2 } from 'lucide-react';
import { clsx } from 'clsx';
import { api } from '../utils/api';
import { useTriggerBackup, useRestoreBackup } from '../hooks/useServers';
import type { Backup, Server } from '../types';

const STATUS_ICONS: Record<string, React.ReactNode> = {
  complete:  <CheckCircle className="w-3 h-3 text-green-400" />,
  failed:    <XCircle className="w-3 h-3 text-red-400" />,
  running:   <Loader2 className="w-3 h-3 text-blue-400 animate-spin" />,
  pending:   <Clock className="w-3 h-3 text-gray-400" />,
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

  return (
    <div className="bg-[#141414] border border-[#252525] rounded-xl overflow-hidden">
      {/* Header row */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-[#1a1a1a] transition-colors"
        onClick={() => setExpanded(e => !e)}
      >
        <div className="flex items-center gap-3">
          <HardDrive className="w-5 h-5 text-gray-400 shrink-0" />
          <div>
            <div className="text-sm font-medium text-gray-100">{server.name}</div>
            <div className="text-xs text-gray-500 capitalize">{server.adapter}</div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {expanded && backups.length > 0 && (
            <span className="text-xs text-gray-500 mr-2">
              {backups.length} backup{backups.length !== 1 ? 's' : ''}
            </span>
          )}
          <button
            onClick={e => { e.stopPropagation(); triggerMutation.mutate('full'); }}
            disabled={triggerMutation.isPending}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg transition-colors disabled:opacity-50"
          >
            {triggerMutation.isPending ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : (
              <Download className="w-3 h-3" />
            )}
            Backup Now
          </button>
          <button
            onClick={e => { e.stopPropagation(); refetch(); }}
            className="p-1.5 text-gray-500 hover:text-gray-300 hover:bg-[#252525] rounded-lg transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Expanded backup list */}
      {expanded && (
        <div className="border-t border-[#1a1a1a] px-4 pb-4 pt-3">
          {isLoading ? (
            <div className="flex items-center gap-2 text-gray-500 text-sm py-2">
              <Loader2 className="w-4 h-4 animate-spin" /> Loading backups...
            </div>
          ) : backups.length === 0 ? (
            <p className="text-gray-500 text-sm py-2">
              No backups yet. Click &quot;Backup Now&quot; to create one.
            </p>
          ) : (
            <div className="space-y-2">
              {backups.map(b => (
                <div
                  key={b.id}
                  className="flex items-center justify-between bg-[#0d0d0d] border border-[#1e1e1e] rounded-lg px-3 py-2.5"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    {STATUS_ICONS[b.status] ?? <Clock className="w-3 h-3 text-gray-400" />}
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-mono text-gray-300 truncate">{b.id.slice(0, 8)}</span>
                        <span className={clsx(
                          'text-xs px-1.5 py-0.5 rounded capitalize',
                          b.type === 'full' ? 'bg-blue-500/10 text-blue-400' : 'bg-gray-500/10 text-gray-400'
                        )}>
                          {b.type}
                        </span>
                      </div>
                      <div className="text-xs text-gray-500 mt-0.5">
                        {formatDate(b.created_at)} · {formatBytes(b.size_bytes)}
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => restoreMutation.mutate(b.id)}
                    disabled={restoreMutation.isPending || b.status !== 'complete'}
                    className="text-xs text-blue-400 hover:text-blue-300 disabled:opacity-40 disabled:cursor-not-allowed shrink-0 ml-4"
                  >
                    Restore
                  </button>
                </div>
              ))}
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
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Backups</h1>
        <p className="text-sm text-gray-400 mt-1">
          Manage scheduled and on-demand backups. Click a server to view its backup history.
        </p>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className="h-16 bg-[#141414] border border-[#252525] rounded-xl animate-pulse" />
          ))}
        </div>
      ) : servers.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-center">
          <div className="w-14 h-14 bg-[#1a1a1a] rounded-2xl flex items-center justify-center mb-4">
            <HardDrive className="w-7 h-7 text-gray-500" />
          </div>
          <h3 className="text-gray-200 font-medium mb-2">No servers yet</h3>
          <p className="text-gray-500 text-sm max-w-xs">
            Add a server first to start managing backups.
          </p>
        </div>
      ) : (
        <div className="space-y-3 max-w-3xl">
          {servers.map(s => (
            <ServerBackupCard key={s.id} server={s} />
          ))}
        </div>
      )}
    </div>
  );
}

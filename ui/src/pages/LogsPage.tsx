import React, { useEffect, useMemo, useState } from 'react';
import { Activity, FileText, Filter, RefreshCw, Server, ShieldCheck } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../utils/api';
import { useServers } from '../hooks/useServers';
import { cn } from '../utils/cn';
import type { AuditEntry } from '../types';

const TABS = [
  {
    id: 'server',
    label: 'Server Logs',
    desc: 'Tail the console output from any running game server.',
    icon: Server,
  },
  {
    id: 'events',
    label: 'Events',
    desc: 'Track lifecycle events such as starts, deployments, and backups.',
    icon: Activity,
  },
  {
    id: 'security',
    label: 'Security',
    desc: 'Review authentication events and login attempts.',
    icon: ShieldCheck,
  },
  {
    id: 'audit',
    label: 'Audit Trail',
    desc: 'See everything recorded in the daemon audit log.',
    icon: FileText,
  },
] as const;

type TabId = (typeof TABS)[number]['id'];
const LINE_OPTIONS = [100, 200, 400, 800];

const formatDetails = (details: AuditEntry['details']) => {
  if (!details) return 'No extra details';
  if (typeof details === 'string') return details;
  try {
    return JSON.stringify(details);
  } catch {
    return 'Unable to render details';
  }
};

const StatusChip = ({ success }: { success: boolean }) => (
  <span
    className={cn(
      'text-[11px] font-semibold px-2 py-0.5 rounded-full border',
      success
        ? 'text-emerald-300 border-emerald-500/30 bg-emerald-500/10'
        : 'text-rose-300 border-rose-500/30 bg-rose-500/10',
    )}
  >
    {success ? 'Success' : 'Failed'}
  </span>
);

const AuditEntryRow = ({ entry }: { entry: AuditEntry }) => {
  const timestamp = new Date(entry.timestamp).toLocaleString();
  const details = formatDetails(entry.details);
  return (
    <article className="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[rgba(255,255,255,0.02)] p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-white">{entry.action}</p>
          <p className="text-[11px] text-[rgba(208,208,232,0.9)]">
            {entry.username || 'system'} • {entry.resource || 'system'}
          </p>
        </div>
        <StatusChip success={entry.success} />
      </div>
      <p className="text-[11px] mt-1 text-[rgba(148,163,184,0.95)]">{timestamp} • {entry.ip || '0.0.0.0'}</p>
      <p className="text-[12px] mt-2 text-[rgba(148,163,184,0.75)] font-mono break-words">{details}</p>
    </article>
  );
};

export function LogsPage() {
  const [activeTab, setActiveTab] = useState<TabId>('server');
  const [selectedServer, setSelectedServer] = useState<string | null>(null);
  const [lines, setLines] = useState(LINE_OPTIONS[1]);
  const [eventFilter, setEventFilter] = useState<string>('all');
  const serversQuery = useServers();
  const servers = serversQuery.data?.servers ?? [];

  const auditQuery = useQuery<AuditEntry[]>({
    queryKey: ['auditLog'],
    queryFn: () => api.get('/api/v1/admin/audit').then(res => res.data.audit_log as AuditEntry[]),
    refetchInterval: 20_000,
  });

  const auditEntries = useMemo(() => {
    if (!auditQuery.data) return [];
    return [...auditQuery.data].reverse();
  }, [auditQuery.data]);

  const eventEntries = useMemo(
    () => auditEntries.filter(entry => entry.resource !== 'auth'),
    [auditEntries],
  );
  const securityEntries = useMemo(
    () => auditEntries.filter(entry => entry.resource === 'auth'),
    [auditEntries],
  );

  // Derive subsystem categories from action prefixes (e.g. "server.start" → "server")
  const eventCategories = useMemo(() => {
    const cats = new Set<string>();
    eventEntries.forEach(e => {
      const prefix = e.action?.split('.')[0] ?? e.resource ?? 'other';
      cats.add(prefix);
    });
    return Array.from(cats).sort();
  }, [eventEntries]);

  const filteredEventEntries = useMemo(() => {
    if (eventFilter === 'all') return eventEntries;
    return eventEntries.filter(e => {
      const prefix = e.action?.split('.')[0] ?? e.resource ?? 'other';
      return prefix === eventFilter;
    });
  }, [eventEntries, eventFilter]);

  useEffect(() => {
    if (!selectedServer && servers.length > 0) {
      setSelectedServer(servers[0].id);
    }
  }, [servers, selectedServer]);

  const serverLogsQuery = useQuery<string[]>({
    queryKey: ['serverLogs', selectedServer, lines],
    queryFn: () =>
      api
        .get(`/api/v1/servers/${selectedServer}/logs`, { params: { lines } })
        .then(res => res.data.logs as string[]),
    enabled: !!selectedServer,
    refetchInterval: 5000,
  });

  const activeServer = servers.find(s => s.id === selectedServer);
  const currentTab = TABS.find(tab => tab.id === activeTab)!;

  return (
    <div className="p-6 md:p-8 space-y-6 animate-page">
      <div>
        <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Logs</h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          Live access to console output, lifecycle events, authentication history, and the full audit trail.
        </p>
      </div>

      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          {TABS.map(tab => {
            const Icon = tab.icon;
            const active = tab.id === activeTab;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  'flex items-center gap-2 rounded-2xl px-4 py-2 text-sm font-semibold transition-all duration-150',
                  active
                    ? 'bg-[rgba(249,115,22,0.2)] text-white'
                    : 'bg-[rgba(255,255,255,0.03)] text-[rgba(208,208,232,0.9)] hover:bg-[rgba(255,255,255,0.08)]',
                )}
              >
                <Icon size={14} />
                {tab.label}
              </button>
            );
          })}
        </div>
        <p className="text-xs text-[rgba(148,163,184,0.85)]">{currentTab.desc}</p>
      </div>

      {activeTab === 'server' && (
        <div className="grid gap-4 lg:grid-cols-[280px,1fr]">
          <div className="rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.01)] p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wide text-[rgba(208,208,232,0.7)]">Servers</p>
                <p className="text-sm text-white">{servers.length} available</p>
              </div>
            </div>
            <div className="mt-4 space-y-2">
              {serversQuery.isLoading && <p className="text-xs text-[rgba(148,163,184,0.8)]">Loading servers...</p>}
              {!serversQuery.isLoading && servers.length === 0 && (
                <p className="text-xs text-[rgba(148,163,184,0.8)]">No servers configured yet.</p>
              )}
              {servers.map(server => {
                const selected = server.id === selectedServer;
                return (
                  <button
                    key={server.id}
                    type="button"
                    onClick={() => setSelectedServer(server.id)}
                    className={cn(
                      'w-full text-left rounded-xl px-3 py-2 transition-all duration-150',
                      selected
                        ? 'bg-[rgba(249,115,22,0.15)] border border-[rgba(249,115,22,0.4)]'
                        : 'bg-[rgba(255,255,255,0.02)] hover:bg-[rgba(255,255,255,0.05)]',
                    )}
                  >
                    <p className="text-sm font-semibold text-white">{server.name}</p>
                    <p className="text-[11px] text-[rgba(148,163,184,0.9)]">{server.adapter} • {server.state}</p>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="rounded-2xl border border-[rgba(255,255,255,0.06)] bg-[rgba(255,255,255,0.01)] p-5 flex flex-col gap-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-white">{activeServer?.name ?? 'Select a server'}</p>
                <p className="text-xs text-[rgba(148,163,184,0.8)]">{activeServer ? `${activeServer.adapter} • ${activeServer.install_dir}` : 'Choose a server to tail logs.'}</p>
              </div>
              <div className="flex items-center gap-2">
                <label className="text-[11px] text-[rgba(148,163,184,0.9)]">Lines</label>
                <select
                  className="input text-xs w-[100px]"
                  value={lines}
                  onChange={e => setLines(Number(e.target.value))}
                >
                  {LINE_OPTIONS.map(option => (
                    <option key={option} value={option}>{option}</option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={() => serverLogsQuery.refetch()}
                  disabled={!selectedServer}
                  className={cn(
                    'btn-ghost flex items-center gap-1',
                    !selectedServer ? 'opacity-40 cursor-not-allowed' : '',
                  )}
                >
                  <RefreshCw size={14} />
                  <span>Refresh</span>
                </button>
              </div>
            </div>

            <div className="min-h-[260px] overflow-y-auto rounded-2xl border border-[rgba(255,255,255,0.05)] bg-[rgba(0,0,0,0.35)] p-4 font-mono text-[12px] leading-relaxed">
              {!selectedServer && (
                <p className="text-[10px] text-[rgba(148,163,184,0.8)]">Select a server from the list to begin tailing its log.</p>
              )}
              {serverLogsQuery.isLoading && <p className="text-[10px] text-[rgba(148,163,184,0.8)]">Fetching logs…</p>}
              {!serverLogsQuery.isLoading && serverLogsQuery.data?.length === 0 && (
                <p className="text-[10px] text-[rgba(148,163,184,0.8)]">No log lines returned yet.</p>
              )}
              {serverLogsQuery.data?.map((line, index) => (
                <p key={`${line}-${index}`} className="text-[12px] text-[rgba(239,239,255,0.9)]">{line}</p>
              ))}
              {serverLogsQuery.isFetching && !serverLogsQuery.isLoading && (
                <p className="text-[10px] text-[rgba(59,130,246,0.9)]">Refreshing…</p>
              )}
            </div>
          </div>
        </div>
      )}

      {activeTab !== 'server' && (
        <div className="rounded-2xl border border-[rgba(255,255,255,0.08)] bg-[rgba(255,255,255,0.01)] p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold text-white">{currentTab.label}</p>
              <p className="text-xs text-[rgba(148,163,184,0.75)]">{currentTab.desc}</p>
            </div>
            <button
              type="button"
              onClick={() => auditQuery.refetch()}
              className="btn-ghost flex items-center gap-1 text-xs"
            >
              <RefreshCw size={14} />
              Refresh
            </button>
          </div>
          {/* Subsystem filter — only shown on Events tab */}
          {activeTab === 'events' && eventCategories.length > 1 && (
            <div className="flex items-center gap-2 flex-wrap">
              <Filter className="w-3 h-3 text-[rgba(148,163,184,0.7)] shrink-0" />
              {['all', ...eventCategories].map(cat => (
                <button
                  key={cat}
                  type="button"
                  onClick={() => setEventFilter(cat)}
                  className={cn(
                    'rounded-xl px-2.5 py-1 text-[11px] font-semibold capitalize transition-all duration-150',
                    eventFilter === cat
                      ? 'bg-[rgba(249,115,22,0.2)] text-white'
                      : 'bg-[rgba(255,255,255,0.04)] text-[rgba(208,208,232,0.8)] hover:bg-[rgba(255,255,255,0.08)]',
                  )}
                >
                  {cat === 'all' ? 'All' : cat}
                  {cat !== 'all' && (
                    <span className="ml-1 opacity-60">
                      ({eventEntries.filter(e => (e.action?.split('.')[0] ?? e.resource ?? 'other') === cat).length})
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}

          <div className="max-h-[520px] overflow-y-auto space-y-3">
            {activeTab === 'events' && (filteredEventEntries.length > 0 ? (
              filteredEventEntries.map(entry => <AuditEntryRow key={entry.id} entry={entry} />)
            ) : (
              <p className="text-xs text-[rgba(148,163,184,0.8)]">No recent events.</p>
            ))}
            {activeTab === 'security' && (securityEntries.length > 0 ? (
              securityEntries.map(entry => <AuditEntryRow key={entry.id} entry={entry} />)
            ) : (
              <p className="text-xs text-[rgba(148,163,184,0.8)]">No authentication events recorded yet.</p>
            ))}
            {activeTab === 'audit' && (auditEntries.length > 0 ? (
              auditEntries.map(entry => <AuditEntryRow key={entry.id} entry={entry} />)
            ) : (
              <p className="text-xs text-[rgba(148,163,184,0.8)]">Audit log is empty.</p>
            ))}
            {auditQuery.isFetching && (
              <p className="text-[10px] text-[rgba(59,130,246,0.9)]">Updating audit trail…</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// ServersPage.tsx
import React, { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Plus, Search, Server, X, Play, Square, RotateCcw } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { useNavigate } from 'react-router-dom';
import { api } from '../utils/api';
import { cn } from '../utils/cn';
import { ADAPTER_ICONS, ADAPTER_NAMES, ADAPTER_COLORS } from '../utils/adapters';
import { useServers, useStartServer, useStopServer, useRestartServer } from '../hooks/useServers';

// ── Noise texture data-URI (subtle grain overlay) ─────────────────────────────
const NOISE_BG =
  "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.04'/%3E%3C/svg%3E\")";

// ── Status helpers ────────────────────────────────────────────────────────────
const STATUS_DOT: Record<string, string> = {
  running:   '#22c55e',
  stopped:   '#6b7280',
  starting:  '#f97316',
  stopping:  '#f97316',
  deploying: '#f97316',
  error:     '#ef4444',
  idle:      '#6b7280',
};

const STATUS_LABELS: Record<string, string> = {
  running:   'Running',
  stopped:   'Stopped',
  starting:  'Starting',
  stopping:  'Stopping',
  deploying: 'Deploying',
  error:     'Error',
  idle:      'Idle',
};

// ── Skeleton poster card ──────────────────────────────────────────────────────
function SkeletonPosterCard() {
  return (
    <div
      className="rounded-2xl shrink-0"
      style={{
        width: 168,
        height: 224,
        background: 'linear-gradient(160deg, #1e1e2e 0%, #13131f 100%)',
        border: '1px solid rgba(255,255,255,0.06)',
        animation: 'pulse 1.6s ease-in-out infinite',
      }}
    />
  );
}

// ── Action buttons shown on hover overlay ────────────────────────────────────
interface ActionOverlayProps {
  serverId: string;
  serverName: string;
  state: string;
}

function ActionOverlay({ serverId, serverName, state }: ActionOverlayProps) {
  const startM  = useStartServer(serverId);
  const stopM   = useStopServer(serverId);
  const restartM = useRestartServer(serverId);

  const isRunning = state === 'running';
  const isBusy    = ['starting', 'stopping', 'deploying'].includes(state);

  const iconBtn =
    'flex items-center justify-center w-9 h-9 rounded-full bg-white/15 hover:bg-white/30 backdrop-blur-sm transition-all duration-150 disabled:opacity-40 disabled:cursor-not-allowed text-white';

  return (
    <div
      className="absolute inset-0 flex flex-col items-center justify-center gap-3 rounded-2xl"
      style={{ background: 'rgba(0,0,0,0.52)', backdropFilter: 'blur(2px)' }}
      onClick={e => e.stopPropagation()}
    >
      <div className="flex items-center gap-2">
        {/* Play — only when not running */}
        {!isRunning && (
          <button
            className={iconBtn}
            title={`Start ${serverName}`}
            disabled={isBusy || startM.isPending}
            onClick={() => startM.mutate()}
          >
            <Play className="w-4 h-4" />
          </button>
        )}

        {/* Stop — only when running */}
        {isRunning && (
          <button
            className={iconBtn}
            title={`Stop ${serverName}`}
            disabled={isBusy || stopM.isPending}
            onClick={() => stopM.mutate()}
          >
            <Square className="w-4 h-4" />
          </button>
        )}

        {/* Restart — always available */}
        <button
          className={iconBtn}
          title={`Restart ${serverName}`}
          disabled={isBusy || restartM.isPending}
          onClick={() => restartM.mutate()}
        >
          <RotateCcw className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}

// ── GamePosterCard ─────────────────────────────────────────────────────────────
interface GamePosterCardProps {
  server: {
    id: string;
    name: string;
    adapter: string;
    state: string;
  };
}

function GamePosterCard({ server }: GamePosterCardProps) {
  const navigate = useNavigate();
  const [hovered, setHovered] = useState(false);

  const AdapterIcon = ADAPTER_ICONS[server.adapter] ?? ADAPTER_ICONS.default;
  const color       = ADAPTER_COLORS[server.adapter] ?? '#6b7280';
  const gameName    = ADAPTER_NAMES[server.adapter] ?? server.adapter;
  const dotColor    = STATUS_DOT[server.state]   ?? '#6b7280';
  const stateLabel  = STATUS_LABELS[server.state] ?? server.state;

  const cardStyle: React.CSSProperties = {
    width: 168,
    height: 224,
    background: `linear-gradient(160deg, ${color}cc 0%, ${color}66 40%, #0b0b16 100%)`,
    backgroundImage: `${NOISE_BG}, linear-gradient(160deg, ${color}cc 0%, ${color}66 40%, #0b0b16 100%)`,
    border: hovered ? `1px solid ${color}55` : '1px solid rgba(255,255,255,0.08)',
    borderRadius: 16,
    boxShadow: hovered
      ? `0 8px 32px rgba(0,0,0,0.7), 0 0 0 1px ${color}33`
      : '0 4px 20px rgba(0,0,0,0.5)',
    transform: hovered ? 'scale(1.04)' : 'scale(1)',
    transition: 'all 0.2s ease',
    cursor: 'pointer',
    position: 'relative',
    overflow: 'hidden',
    flexShrink: 0,
    display: 'flex',
    flexDirection: 'column',
  };

  return (
    <div
      style={cardStyle}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onClick={() => navigate(`/servers/${server.id}`)}
      role="button"
      aria-label={`Open ${server.name}`}
    >
      {/* Upper portion — icon centered in frosted circle */}
      <div className="flex flex-col items-center justify-center flex-1 gap-3 px-2 pt-5">
        {/* Frosted icon circle */}
        <div
          className="flex items-center justify-center rounded-full backdrop-blur-sm"
          style={{
            width: 72,
            height: 72,
            background: 'rgba(255,255,255,0.10)',
            border: '1px solid rgba(255,255,255,0.15)',
            flexShrink: 0,
          }}
        >
          <span style={{ color: '#ffffff', display: 'flex' }}>
            <AdapterIcon className="w-12 h-12" />
          </span>
        </div>

        {/* Game name */}
        <p
          className="text-xs font-bold text-white text-center px-2 leading-tight"
          style={{
            display: '-webkit-box',
            WebkitLineClamp: 2,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
            wordBreak: 'break-word',
          }}
        >
          {gameName}
        </p>
      </div>

      {/* Bottom strip — server name + status */}
      <div
        className="px-3 py-2.5 flex flex-col gap-1"
        style={{
          background: 'rgba(0,0,0,0.40)',
          borderTop: '1px solid rgba(255,255,255,0.07)',
          minHeight: 56,
        }}
      >
        <p
          className="text-white font-semibold leading-tight"
          style={{
            fontSize: 11,
            display: '-webkit-box',
            WebkitLineClamp: 1,
            WebkitBoxOrient: 'vertical',
            overflow: 'hidden',
          }}
        >
          {server.name}
        </p>
        <div className="flex items-center gap-1.5">
          <span
            className="w-2 h-2 rounded-full shrink-0"
            style={{ background: dotColor }}
          />
          <span
            className="font-medium capitalize"
            style={{ fontSize: 10, color: 'rgba(255,255,255,0.6)' }}
          >
            {stateLabel}
          </span>
        </div>
      </div>

      {/* Hover action overlay */}
      {hovered && (
        <ActionOverlay
          serverId={server.id}
          serverName={server.name}
          state={server.state}
        />
      )}
    </div>
  );
}

// ── ServersPage ────────────────────────────────────────────────────────────────
export function ServersPage() {
  const [search, setSearch]       = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const queryClient               = useQueryClient();

  const { data, isLoading } = useServers(15_000);

  const allServers = data?.servers ?? [];
  const servers = allServers.filter((s: any) =>
    s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.adapter.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="p-6 md:p-8 animate-page">
      {/* Page Header */}
      <div className="flex items-center justify-between gap-4 mb-8">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              Servers
            </h1>
            {allServers.length > 0 && (
              <span
                className="badge"
                style={{
                  background: 'rgba(249,115,22,0.1)',
                  color: '#fb923c',
                  border: '1px solid rgba(249,115,22,0.2)',
                }}
              >
                {allServers.length}
              </span>
            )}
          </div>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            Manage and monitor your game servers
          </p>
        </div>
        <button onClick={() => setShowCreate(true)} className="btn-primary">
          <Plus className="w-4 h-4" />
          New Server
        </button>
      </div>

      {/* Search */}
      <div className="relative mb-6 max-w-md">
        <Search
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none"
          style={{ color: 'var(--text-muted)' }}
        />
        <input
          type="text"
          placeholder="Search servers by name or game..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="input pl-9"
        />
        {search && (
          <button
            onClick={() => setSearch('')}
            className="absolute right-3 top-1/2 -translate-y-1/2 transition-colors"
            style={{ color: 'var(--text-muted)' }}
          >
            <X className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* Content */}
      {isLoading ? (
        /* Skeleton grid */
        <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7 2xl:grid-cols-8 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <SkeletonPosterCard key={i} />
          ))}
        </div>
      ) : servers.length === 0 ? (
        search ? (
          /* No search results */
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4"
              style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
            >
              <Search className="w-8 h-8" style={{ color: 'var(--text-muted)' }} />
            </div>
            <h3 className="text-base font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
              No results for &ldquo;{search}&rdquo;
            </h3>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Try a different name or game type.
            </p>
            <button onClick={() => setSearch('')} className="btn-ghost mt-4">
              Clear search
            </button>
          </div>
        ) : (
          /* Empty state */
          <div className="flex flex-col items-center justify-center py-20 text-center">
            <div
              className="w-20 h-20 rounded-2xl flex items-center justify-center mb-5"
              style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
            >
              <Server className="w-10 h-10" style={{ color: 'var(--text-muted)' }} />
            </div>
            <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
              No servers yet
            </h3>
            <p className="text-sm max-w-xs mb-6" style={{ color: 'var(--text-secondary)' }}>
              Create your first game server to get started. Supports Valheim, Minecraft,
              Satisfactory and 20+ more games.
            </p>
            <button onClick={() => setShowCreate(true)} className="btn-primary">
              <Plus className="w-4 h-4" />
              Create Server
            </button>
          </div>
        )
      ) : (
        /* Poster grid */
        <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7 2xl:grid-cols-8 gap-4">
          {servers.map((s: any) => (
            <GamePosterCard key={s.id} server={s} />
          ))}
        </div>
      )}

      {showCreate && (
        <CreateServerModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            queryClient.invalidateQueries({ queryKey: ['servers'] });
          }}
        />
      )}
    </div>
  );
}

// ── CreateServerModal (unchanged) ─────────────────────────────────────────────
function CreateServerModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState({
    id: '',
    name: '',
    adapter: 'minecraft',
    deployMethod: 'manual',
    installDir: '/opt/games',
    dockerImage: '',
  });
  const [loading, setLoading] = useState(false);

  const ADAPTERS = [
    'valheim', 'minecraft', 'satisfactory', 'palworld', 'eco', 'enshrouded', 'riftbreaker',
  ];
  const DEPLOY_METHODS = [
    { value: 'manual',   label: 'Manual (archive)' },
    { value: 'steamcmd', label: 'SteamCMD' },
    { value: 'docker',   label: 'Docker' },
  ];

  const handleCreate = async () => {
    setLoading(true);
    try {
      const config =
        form.deployMethod === 'docker' && form.dockerImage
          ? { docker_image: form.dockerImage }
          : undefined;
      await api.post('/api/v1/servers', {
        id: form.id,
        name: form.name,
        adapter: form.adapter,
        deploy_method: form.deployMethod,
        install_dir: form.installDir,
        ...(config ? { config } : {}),
      });
      toast.success('Server created!');
      onCreated();
    } catch (e: any) {
      toast.error(e.response?.data?.error ?? 'Create failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50 p-4"
      style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}
    >
      <div
        className="card w-full max-w-md p-6 space-y-5"
        style={{ background: 'var(--bg-card)', maxHeight: '90vh', overflowY: 'auto' }}
      >
        {/* Modal header */}
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
              New Server
            </h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
              Configure your new game server
            </p>
          </div>
          <button onClick={onClose} className="btn-ghost p-2" style={{ padding: '6px' }}>
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Divider */}
        <div style={{ height: 1, background: 'var(--border)' }} />

        {/* Fields */}
        <div>
          <label className="label">Server ID</label>
          <input
            type="text"
            value={form.id}
            onChange={e => setForm(p => ({ ...p, id: e.target.value }))}
            placeholder="e.g. my-minecraft-1"
            className="input"
          />
        </div>

        <div>
          <label className="label">Server Name</label>
          <input
            type="text"
            value={form.name}
            onChange={e => setForm(p => ({ ...p, name: e.target.value }))}
            placeholder="e.g. Survival World"
            className="input"
          />
        </div>

        <div>
          <label className="label">Adapter</label>
          <select
            value={form.adapter}
            onChange={e => setForm(p => ({ ...p, adapter: e.target.value }))}
            className="input"
          >
            {ADAPTERS.map(a => (
              <option key={a} value={a} style={{ background: 'var(--bg-input)' }}>
                {a.charAt(0).toUpperCase() + a.slice(1)}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="label">Deploy Method</label>
          <select
            value={form.deployMethod}
            onChange={e => setForm(p => ({ ...p, deployMethod: e.target.value }))}
            className="input"
          >
            {DEPLOY_METHODS.map(m => (
              <option key={m.value} value={m.value} style={{ background: 'var(--bg-input)' }}>
                {m.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="label">Install Directory</label>
          <input
            type="text"
            value={form.installDir}
            onChange={e => setForm(p => ({ ...p, installDir: e.target.value }))}
            placeholder="/opt/games"
            className="input"
          />
        </div>

        {form.deployMethod === 'docker' && (
          <div>
            <label className="label">
              Docker Image{' '}
              <span style={{ color: 'var(--text-muted)', textTransform: 'none', letterSpacing: 0 }}>
                (optional — leave blank for adapter default)
              </span>
            </label>
            <input
              type="text"
              value={form.dockerImage}
              onChange={e => setForm(p => ({ ...p, dockerImage: e.target.value }))}
              placeholder="e.g. itzg/minecraft-server:latest"
              className="input"
            />
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-3 pt-1">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">
            Cancel
          </button>
          <button
            onClick={handleCreate}
            disabled={loading || !form.id || !form.name}
            className="btn-primary flex-1 justify-center"
          >
            {loading ? 'Creating...' : 'Create Server'}
          </button>
        </div>
      </div>
    </div>
  );
}

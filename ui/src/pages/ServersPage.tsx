// ServersPage.tsx
import React, { useState } from 'react';
import { useQueryClient, useMutation } from '@tanstack/react-query';
import { Plus, Search, Server, X, Play, Square, RotateCcw, Trash2, ChevronLeft, ExternalLink, Download } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { useNavigate } from 'react-router-dom';
import { api } from '../utils/api';
import { cn } from '../utils/cn';
import { ADAPTER_ICONS, ADAPTER_NAMES, ADAPTER_COLORS } from '../utils/adapters';
import { useServers, useStartServer, useStopServer, useRestartServer, useDeleteServer } from '../hooks/useServers';
import { ADAPTER_DEPLOY_METHODS } from '../utils/adapters';

// ── Noise texture data-URI (subtle grain overlay) ─────────────────────────────
const NOISE_BG =
  "url(\"data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.04'/%3E%3C/svg%3E\")";

// All supported adapters derived from the canonical map
const ALL_ADAPTERS = Object.keys(ADAPTER_NAMES) as string[];

// ── Status helpers ────────────────────────────────────────────────────────────
const STATUS_DOT: Record<string, string> = {
  running:   '#22c55e',
  stopped:   '#6b7280',
  starting:  '#f97316',
  stopping:  '#f97316',
  deploying: '#3b82f6',
  updating:  '#06b6d4',
  error:     '#ef4444',
  idle:      '#6b7280',
};

const STATUS_LABELS: Record<string, string> = {
  running:   'Running',
  stopped:   'Stopped',
  starting:  'Starting',
  stopping:  'Stopping',
  deploying: 'Deploying',
  updating:  'Updating',
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

// ── Delete confirmation modal ─────────────────────────────────────────────────
interface DeleteConfirmProps {
  serverName: string;
  onConfirm: () => void;
  onCancel: () => void;
  isDeleting: boolean;
}

function DeleteConfirmModal({ serverName, onConfirm, onCancel, isDeleting }: DeleteConfirmProps) {
  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50 p-4"
      style={{ background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(4px)' }}
      onClick={onCancel}
    >
      <div
        className="card w-full max-w-sm p-6 space-y-4"
        style={{ background: 'var(--bg-card)' }}
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3">
          <div
            className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
            style={{ background: 'rgba(239,68,68,0.15)', border: '1px solid rgba(239,68,68,0.3)' }}
          >
            <Trash2 className="w-5 h-5" style={{ color: '#ef4444' }} />
          </div>
          <div>
            <h2 className="text-base font-bold" style={{ color: 'var(--text-primary)' }}>Delete Server</h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>This action cannot be undone.</p>
          </div>
        </div>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Are you sure you want to delete <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>{serverName}</span>?
          The server configuration will be removed. Game files on disk are not deleted.
        </p>
        <div className="flex gap-3 pt-1">
          <button onClick={onCancel} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            onClick={onConfirm}
            disabled={isDeleting}
            className="flex-1 flex items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-150"
            style={{
              background: isDeleting ? 'rgba(239,68,68,0.4)' : 'rgba(239,68,68,0.85)',
              color: '#fff',
              border: '1px solid rgba(239,68,68,0.3)',
              cursor: isDeleting ? 'not-allowed' : 'pointer',
            }}
          >
            <Trash2 className="w-4 h-4" />
            {isDeleting ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Action buttons shown on hover overlay ────────────────────────────────────
interface ActionOverlayProps {
  serverId: string;
  serverName: string;
  state: string;
  onDeleteRequest: () => void;
  onNavigate: () => void;
}

function ActionOverlay({ serverId, serverName, state, onDeleteRequest, onNavigate }: ActionOverlayProps) {
  const qc       = useQueryClient();
  const startM   = useStartServer(serverId);
  const stopM    = useStopServer(serverId);
  const restartM = useRestartServer(serverId);
  const deployM  = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${serverId}/deploy`),
    onSuccess: () => { toast.success('Deploy started — downloading game files…'); qc.invalidateQueries({ queryKey: ['servers'] }); },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Deploy failed'),
  });

  const isRunning = state === 'running';
  const needsDeploy = state === 'idle';
  const isBusy    = ['starting', 'stopping', 'deploying', 'updating'].includes(state);

  const iconBtn =
    'flex items-center justify-center w-9 h-9 rounded-full bg-white/15 hover:bg-white/30 backdrop-blur-sm transition-all duration-150 disabled:opacity-40 disabled:cursor-not-allowed text-white';

  return (
    <div
      className="absolute inset-0 flex flex-col items-center justify-between rounded-2xl py-3"
      style={{ background: 'rgba(0,0,0,0.52)', backdropFilter: 'blur(2px)' }}
      onClick={e => e.stopPropagation()}
    >
      {/* Top row: open + delete */}
      <div className="w-full flex justify-between px-2">
        <button
          className="flex items-center justify-center w-7 h-7 rounded-full bg-white/15 hover:bg-white/30 backdrop-blur-sm transition-all duration-150 text-white"
          title={`Open ${serverName}`}
          onClick={onNavigate}
        >
          <ExternalLink className="w-3.5 h-3.5" />
        </button>
        <button
          className="flex items-center justify-center w-7 h-7 rounded-full bg-rose-500/20 hover:bg-rose-500/50 backdrop-blur-sm transition-all duration-150 text-rose-300"
          title={`Delete ${serverName}`}
          onClick={onDeleteRequest}
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Center action buttons */}
      <div className="flex items-center gap-2">
        {needsDeploy && (
          <button
            className={iconBtn}
            title={`Deploy ${serverName}`}
            disabled={isBusy || deployM.isPending}
            onClick={() => deployM.mutate()}
          >
            <Download className="w-4 h-4" />
          </button>
        )}
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
        <button
          className={iconBtn}
          title={`Restart ${serverName}`}
          disabled={isBusy || restartM.isPending}
          onClick={() => restartM.mutate()}
        >
          <RotateCcw className="w-4 h-4" />
        </button>
      </div>

      {/* Bottom spacer */}
      <div />
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
  const deleteM  = useDeleteServer();
  const [hovered, setHovered] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

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
    <>
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
            <span className="w-2 h-2 rounded-full shrink-0" style={{ background: dotColor }} />
            <span className="font-medium capitalize" style={{ fontSize: 10, color: 'rgba(255,255,255,0.6)' }}>
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
            onDeleteRequest={() => setConfirmDelete(true)}
            onNavigate={() => navigate(`/servers/${server.id}`)}
          />
        )}
      </div>

      {confirmDelete && (
        <DeleteConfirmModal
          serverName={server.name}
          isDeleting={deleteM.isPending}
          onCancel={() => setConfirmDelete(false)}
          onConfirm={() =>
            deleteM.mutate(server.id, { onSuccess: () => setConfirmDelete(false) })
          }
        />
      )}
    </>
  );
}

// ── Mini adapter poster for the game picker ───────────────────────────────────
interface AdapterCardProps {
  adapterId: string;
  selected: boolean;
  onClick: () => void;
}

function AdapterCard({ adapterId, selected, onClick }: AdapterCardProps) {
  const AdapterIcon = ADAPTER_ICONS[adapterId] ?? ADAPTER_ICONS.default;
  const color       = ADAPTER_COLORS[adapterId] ?? '#6b7280';
  const gameName    = ADAPTER_NAMES[adapterId] ?? adapterId;

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col items-center gap-1.5 rounded-xl p-2.5 text-center transition-all duration-150"
      style={{
        background: selected
          ? `linear-gradient(160deg, ${color}99 0%, ${color}44 100%)`
          : 'rgba(255,255,255,0.03)',
        border: selected
          ? `1.5px solid ${color}88`
          : '1.5px solid rgba(255,255,255,0.07)',
        boxShadow: selected ? `0 0 0 2px ${color}33` : 'none',
        cursor: 'pointer',
      }}
    >
      <div
        className="flex items-center justify-center rounded-full"
        style={{
          width: 48,
          height: 48,
          background: selected ? `${color}55` : 'rgba(255,255,255,0.07)',
          border: `1px solid ${selected ? color + '66' : 'rgba(255,255,255,0.10)'}`,
          flexShrink: 0,
        }}
      >
        <span style={{ color: selected ? '#fff' : 'rgba(255,255,255,0.7)', display: 'flex' }}>
          <AdapterIcon className="w-7 h-7" />
        </span>
      </div>
      <span
        className="font-semibold leading-tight text-center"
        style={{
          fontSize: 10,
          color: selected ? '#fff' : 'rgba(208,208,232,0.85)',
          display: '-webkit-box',
          WebkitLineClamp: 2,
          WebkitBoxOrient: 'vertical',
          overflow: 'hidden',
          wordBreak: 'break-word',
          maxWidth: 80,
        }}
      >
        {gameName}
      </span>
    </button>
  );
}

const DEPLOY_METHOD_META: { value: string; label: string; desc: string }[] = [
  { value: 'any',      label: 'All Games',  desc: 'Show all supported games' },
  { value: 'steamcmd', label: 'SteamCMD',   desc: 'Install via Steam (requires SteamCMD)' },
  { value: 'manual',   label: 'Manual',     desc: 'Upload / extract an archive yourself' },
  { value: 'docker',   label: 'Docker',     desc: 'Run in a Docker container (requires Docker)' },
];

// ── CreateServerModal — 2-step: game picker → config ──────────────────────────
function CreateServerModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [step, setStep] = useState<'pick' | 'config'>('pick');
  const [adapter, setAdapter] = useState('');
  const [deployFilter, setDeployFilter] = useState('any');
  const [form, setForm] = useState({
    id: '',
    name: '',
    deployMethod: 'manual',
    installDir: '/opt/games',
    dockerImage: '',
  });
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');

  const filteredAdapters = ALL_ADAPTERS.filter(a => {
    const nameMatch =
      ADAPTER_NAMES[a]?.toLowerCase().includes(search.toLowerCase()) ||
      a.toLowerCase().includes(search.toLowerCase());
    const methodMatch =
      deployFilter === 'any' ||
      (ADAPTER_DEPLOY_METHODS[a] ?? ['manual']).includes(deployFilter);
    return nameMatch && methodMatch;
  });

  const handlePickGame = (a: string) => {
    setAdapter(a);
    // Pre-select the active deploy filter as the default method (if supported)
    const supported = ADAPTER_DEPLOY_METHODS[a] ?? ['manual'];
    const defaultMethod =
      deployFilter !== 'any' && supported.includes(deployFilter)
        ? deployFilter
        : supported[0] ?? 'manual';
    setForm(p => ({
      ...p,
      deployMethod: defaultMethod,
      installDir: `/opt/games/${a}`,
    }));
    setStep('config');
  };

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
        adapter,
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

  // Only show deploy methods the selected game actually supports
  const supportedMethods = ADAPTER_DEPLOY_METHODS[adapter] ?? ['manual'];
  const DEPLOY_METHODS = [
    { value: 'manual',   label: 'Manual (archive)' },
    { value: 'steamcmd', label: 'SteamCMD' },
    { value: 'docker',   label: 'Docker' },
  ].filter(m => supportedMethods.includes(m.value));

  const selectedColor   = adapter ? (ADAPTER_COLORS[adapter] ?? '#6b7280') : '#6b7280';
  const selectedName    = adapter ? (ADAPTER_NAMES[adapter] ?? adapter) : '';
  const SelectedIcon    = adapter ? (ADAPTER_ICONS[adapter] ?? ADAPTER_ICONS.default) : ADAPTER_ICONS.default;

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50 p-4"
      style={{ background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(6px)' }}
    >
      <div
        className="card w-full flex flex-col"
        style={{
          background: 'var(--bg-card)',
          maxHeight: '90vh',
          width: step === 'pick' ? 680 : 460,
          transition: 'width 0.2s ease',
        }}
      >
        {/* Modal header */}
        <div className="flex items-center justify-between p-6 pb-4 shrink-0">
          <div className="flex items-center gap-3">
            {step === 'config' && (
              <button
                onClick={() => setStep('pick')}
                className="btn-ghost p-1.5 mr-1"
                title="Back to game picker"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
            )}
            {step === 'config' && adapter && (
              <div
                className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                style={{ background: `${selectedColor}33`, border: `1px solid ${selectedColor}55` }}
              >
                <span style={{ color: selectedColor, display: 'flex' }}>
                  <SelectedIcon className="w-5 h-5" />
                </span>
              </div>
            )}
            <div>
              <h2 className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
                {step === 'pick' ? 'Choose a Game' : `New ${selectedName} Server`}
              </h2>
              <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                {step === 'pick'
                  ? `${ALL_ADAPTERS.length} supported games`
                  : 'Configure your server settings'}
              </p>
            </div>
          </div>
          <button onClick={onClose} className="btn-ghost p-2">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div style={{ height: 1, background: 'var(--border)', flexShrink: 0 }} />

        {/* ── Step 1: Game picker ── */}
        {step === 'pick' && (
          <div className="flex flex-col flex-1 overflow-hidden p-5 gap-3">
            {/* Deploy method filter tabs */}
            <div className="flex items-center gap-1.5 flex-wrap shrink-0">
              {DEPLOY_METHOD_META.map(m => (
                <button
                  key={m.value}
                  type="button"
                  title={m.desc}
                  onClick={() => setDeployFilter(m.value)}
                  className="rounded-xl px-3 py-1 text-xs font-semibold transition-all duration-150"
                  style={{
                    background: deployFilter === m.value
                      ? 'rgba(249,115,22,0.2)'
                      : 'rgba(255,255,255,0.04)',
                    color: deployFilter === m.value
                      ? '#fb923c'
                      : 'rgba(208,208,232,0.8)',
                    border: deployFilter === m.value
                      ? '1px solid rgba(249,115,22,0.35)'
                      : '1px solid rgba(255,255,255,0.07)',
                  }}
                >
                  {m.label}
                </button>
              ))}
              {deployFilter !== 'any' && (
                <span className="text-[11px] ml-1" style={{ color: 'var(--text-muted)' }}>
                  {filteredAdapters.length} game{filteredAdapters.length !== 1 ? 's' : ''}
                </span>
              )}
            </div>

            {/* Search */}
            <div className="relative shrink-0">
              <Search
                className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 pointer-events-none"
                style={{ color: 'var(--text-muted)' }}
              />
              <input
                type="text"
                placeholder="Search games…"
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="input pl-9 text-sm"
                autoFocus
              />
              {search && (
                <button
                  onClick={() => setSearch('')}
                  className="absolute right-3 top-1/2 -translate-y-1/2"
                  style={{ color: 'var(--text-muted)' }}
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </div>

            {/* Game grid */}
            <div
              className="grid gap-2 overflow-y-auto flex-1 pr-1"
              style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(100px, 1fr))' }}
            >
              {filteredAdapters.map(a => (
                <AdapterCard
                  key={a}
                  adapterId={a}
                  selected={adapter === a}
                  onClick={() => handlePickGame(a)}
                />
              ))}
              {filteredAdapters.length === 0 && (
                <p
                  className="col-span-full text-center text-sm py-8"
                  style={{ color: 'var(--text-muted)' }}
                >
                  {search
                    ? `No ${deployFilter !== 'any' ? DEPLOY_METHOD_META.find(m => m.value === deployFilter)?.label + ' ' : ''}games match "${search}"`
                    : `No games support the ${DEPLOY_METHOD_META.find(m => m.value === deployFilter)?.label} deploy method`}
                </p>
              )}
            </div>
          </div>
        )}

        {/* ── Step 2: Config form ── */}
        {step === 'config' && (
          <div className="flex flex-col flex-1 overflow-y-auto p-6 gap-5">
            <div>
              <label className="label">Server ID</label>
              <input
                type="text"
                value={form.id}
                onChange={e => setForm(p => ({ ...p, id: e.target.value }))}
                placeholder={`e.g. my-${adapter}-1`}
                className="input"
                autoFocus
              />
              <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>
                Unique identifier — lowercase letters, digits, and hyphens only.
              </p>
            </div>

            <div>
              <label className="label">Server Name</label>
              <input
                type="text"
                value={form.name}
                onChange={e => setForm(p => ({ ...p, name: e.target.value }))}
                placeholder={`e.g. My ${selectedName} Server`}
                className="input"
              />
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
              <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
              <button
                onClick={handleCreate}
                disabled={loading || !form.id || !form.name}
                className="btn-primary flex-1 justify-center"
              >
                {loading ? 'Creating…' : 'Create Server'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ── ServersPage ────────────────────────────────────────────────────────────────
export function ServersPage() {
  const [search, setSearch]         = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const queryClient                 = useQueryClient();

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
        <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7 2xl:grid-cols-8 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <SkeletonPosterCard key={i} />
          ))}
        </div>
      ) : servers.length === 0 ? (
        search ? (
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
              Satisfactory and {ALL_ADAPTERS.length - 3}+ more games.
            </p>
            <button onClick={() => setShowCreate(true)} className="btn-primary">
              <Plus className="w-4 h-4" />
              Create Server
            </button>
          </div>
        )
      ) : (
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

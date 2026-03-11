// ServersPage.tsx
import React, { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Search, Server, X } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { ServerCard } from '../components/Dashboard/ServerCard';
import { api } from '../utils/api';
import { cn } from '../utils/cn';

export function ServersPage() {
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
    refetchInterval: 15_000,
  });

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
          placeholder="Search servers by name or adapter..."
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
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => (
            <div
              key={i}
              className="card"
              style={{ height: 220, opacity: 0.5, animation: 'pulse 1.5s ease-in-out infinite' }}
            />
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
              Try a different name or adapter type.
            </p>
            <button
              onClick={() => setSearch('')}
              className="btn-ghost mt-4"
            >
              Clear search
            </button>
          </div>
        ) : (
          /* Empty state */
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
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {servers.map((s: any) => (
            <ServerCard key={s.id} server={s} />
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
    <div className="fixed inset-0 flex items-center justify-center z-50 p-4" style={{ background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(4px)' }}>
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

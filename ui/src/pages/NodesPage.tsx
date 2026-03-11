import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Server,
  Plus,
  Trash2,
  Wifi,
  WifiOff,
  Activity,
  Cpu,
  MemoryStick,
  HardDrive,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { cn } from '../utils/cn';
import { api } from '../utils/api';
import type { Node, NodeCapacity, RegisterNodeRequest } from '../types';

// ── Helpers ───────────────────────────────────────────────────────────────────

function pct(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(100, Math.round((used / total) * 100));
}

function UtilBar({ used, total, label }: { used: number; total: number; label: string }) {
  const p = pct(used, total);
  const barColor = p >= 90 ? '#ef4444' : p >= 70 ? '#f59e0b' : 'var(--primary)';
  return (
    <div className="space-y-1.5">
      <div className="flex justify-between text-xs">
        <span style={{ color: 'var(--text-secondary)' }}>{label}</span>
        <span style={{ color: 'var(--text-primary)' }}>{p}%</span>
      </div>
      <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
        <div
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${p}%`, background: barColor }}
        />
      </div>
    </div>
  );
}

const STATUS_CLASSES: Record<string, string> = {
  online:   'status-running',
  offline:  'status-error',
  draining: 'status-starting',
};

// ── Add-node dialog ───────────────────────────────────────────────────────────

interface AddNodeDialogProps {
  onClose: () => void;
  onAdd: (req: RegisterNodeRequest) => void;
  loading: boolean;
}

function AddNodeDialog({ onClose, onAdd, loading }: AddNodeDialogProps) {
  const [form, setForm] = useState<RegisterNodeRequest>({
    hostname: '',
    address: '',
    capacity: { cpu_cores: 4, memory_gb: 8, disk_gb: 100 },
  });

  const set = (field: string, value: unknown) =>
    setForm(prev => ({ ...prev, [field]: value }));

  const setCap = (field: keyof NodeCapacity, value: number) =>
    setForm(prev => ({ ...prev, capacity: { ...prev.capacity, [field]: value } }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
      <div className="w-full max-w-md rounded-2xl p-6 space-y-5"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}>
        <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>Register Node</h2>

        <div className="space-y-4">
          <div>
            <label className="label">Hostname</label>
            <input
              className="input"
              placeholder="node-2.example.com"
              value={form.hostname}
              onChange={e => set('hostname', e.target.value)}
            />
          </div>
          <div>
            <label className="label">Agent address (host:port)</label>
            <input
              className="input"
              placeholder="192.168.1.20:9090"
              value={form.address}
              onChange={e => set('address', e.target.value)}
            />
          </div>
          <div className="grid grid-cols-3 gap-3">
            {(
              [
                { key: 'cpu_cores',  label: 'CPU cores' },
                { key: 'memory_gb', label: 'RAM (GB)'   },
                { key: 'disk_gb',   label: 'Disk (GB)'  },
              ] as { key: keyof NodeCapacity; label: string }[]
            ).map(({ key, label }) => (
              <div key={key}>
                <label className="label">{label}</label>
                <input
                  type="number"
                  min={0}
                  className="input"
                  value={form.capacity[key]}
                  onChange={e => setCap(key, parseFloat(e.target.value) || 0)}
                />
              </div>
            ))}
          </div>
          <div>
            <label className="label">Version (optional)</label>
            <input
              className="input"
              placeholder="1.0.0"
              value={form.version ?? ''}
              onChange={e => set('version', e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-3">
          <button onClick={onClose} className="btn-ghost flex-1 justify-center">Cancel</button>
          <button
            disabled={!form.hostname || !form.address || loading}
            onClick={() => onAdd(form)}
            className="btn-primary flex-1 justify-center"
          >
            {loading ? 'Registering…' : 'Register Node'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Node card ─────────────────────────────────────────────────────────────────

interface NodeCardProps {
  node: Node;
  onRemove: (id: string) => void;
}

function NodeCard({ node, onRemove }: NodeCardProps) {
  const isOnline = node.status === 'online';

  return (
    <div className="card p-5 space-y-4 group">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className={cn(
            'w-9 h-9 rounded-xl flex items-center justify-center shrink-0',
          )} style={{
            background: isOnline ? 'rgba(249,115,22,0.12)' : 'rgba(128,128,168,0.1)',
          }}>
            <Server className="w-4 h-4" style={{ color: isOnline ? 'var(--primary)' : 'var(--text-muted)' }} />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-semibold truncate" style={{ color: 'var(--text-primary)' }}>{node.hostname}</p>
            <p className="text-xs truncate font-mono mt-0.5" style={{ color: 'var(--text-muted)' }}>{node.address}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className={cn('badge capitalize', STATUS_CLASSES[node.status] ?? 'status-stopped')}>
            {node.status}
          </span>
          <button
            onClick={() => onRemove(node.id)}
            className="opacity-0 group-hover:opacity-100 p-1.5 rounded-lg transition-all"
            style={{ color: 'var(--text-muted)' }}
            onMouseEnter={e => (e.currentTarget.style.color = '#f87171')}
            onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
            title="Remove node"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Utilisation bars */}
      <div className="space-y-2.5">
        <UtilBar
          used={node.allocated.cpu_cores}
          total={node.capacity.cpu_cores}
          label={`CPU  ${node.allocated.cpu_cores} / ${node.capacity.cpu_cores} cores`}
        />
        <UtilBar
          used={node.allocated.memory_gb}
          total={node.capacity.memory_gb}
          label={`RAM  ${node.allocated.memory_gb} / ${node.capacity.memory_gb} GB`}
        />
        <UtilBar
          used={node.allocated.disk_gb}
          total={node.capacity.disk_gb}
          label={`Disk  ${node.allocated.disk_gb} / ${node.capacity.disk_gb} GB`}
        />
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between text-xs pt-1"
        style={{ borderTop: '1px solid var(--border)', color: 'var(--text-muted)' }}>
        <span className="flex items-center gap-1">
          <Activity className="w-3 h-3" />
          {node.server_count} server{node.server_count !== 1 ? 's' : ''}
        </span>
        {node.version && <span>v{node.version}</span>}
        <span title={new Date(node.last_seen).toLocaleString()}>
          {isOnline
            ? <Wifi className="w-3 h-3 text-green-400" />
            : <WifiOff className="w-3 h-3 text-red-400" />
          }
        </span>
      </div>
    </div>
  );
}

// ── Page ──────────────────────────────────────────────────────────────────────

export function NodesPage() {
  const [showAdd, setShowAdd] = useState(false);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['nodes'],
    queryFn: () => api.get('/api/v1/nodes').then(r => r.data),
    refetchInterval: 15_000,
  });

  const nodes: Node[] = data?.nodes ?? [];
  const online = nodes.filter(n => n.status === 'online').length;
  const offline = nodes.filter(n => n.status !== 'online').length;

  const registerMutation = useMutation({
    mutationFn: (req: RegisterNodeRequest) =>
      api.post('/api/v1/nodes', req).then(r => r.data),
    onSuccess: () => {
      toast.success('Node registered');
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
      setShowAdd(false);
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Failed to register node'),
  });

  const removeMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/nodes/${id}`),
    onSuccess: () => {
      toast.success('Node removed');
      queryClient.invalidateQueries({ queryKey: ['nodes'] });
    },
    onError: (e: any) => toast.error(e.response?.data?.error ?? 'Failed to remove node'),
  });

  return (
    <div className="p-6 md:p-8 animate-page space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4 mb-6">
        <div>
          <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Cluster Nodes</h1>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            {nodes.length} node{nodes.length !== 1 ? 's' : ''} —{' '}
            <span className="text-green-400">{online} online</span>
            {offline > 0 && <>, <span className="text-red-400">{offline} offline</span></>}
          </p>
        </div>
        <button
          onClick={() => setShowAdd(true)}
          className="btn-primary"
        >
          <Plus className="w-4 h-4" /> Register Node
        </button>
      </div>

      {/* Cluster summary stats */}
      {nodes.length > 0 && (() => {
        const totCPU  = nodes.reduce((s, n) => s + n.capacity.cpu_cores, 0);
        const usedCPU = nodes.reduce((s, n) => s + n.allocated.cpu_cores, 0);
        const totMem  = nodes.reduce((s, n) => s + n.capacity.memory_gb, 0);
        const usedMem = nodes.reduce((s, n) => s + n.allocated.memory_gb, 0);
        const totDisk = nodes.reduce((s, n) => s + n.capacity.disk_gb, 0);
        const usedDisk = nodes.reduce((s, n) => s + n.allocated.disk_gb, 0);
        return (
          <div className="grid grid-cols-3 gap-4">
            {[
              { icon: Cpu,         label: 'CPU',  used: usedCPU,  total: totCPU,  unit: 'cores' },
              { icon: MemoryStick, label: 'RAM',  used: usedMem,  total: totMem,  unit: 'GB'    },
              { icon: HardDrive,   label: 'Disk', used: usedDisk, total: totDisk, unit: 'GB'    },
            ].map(({ icon: Icon, label, used, total, unit }) => (
              <div key={label} className="card p-5">
                <div className="flex items-center gap-2 mb-3">
                  <Icon className="w-4 h-4" style={{ color: 'var(--primary)' }} />
                  <span className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{label}</span>
                </div>
                <p className="text-2xl font-bold mb-2" style={{ color: 'var(--text-primary)' }}>
                  {used}
                  <span className="text-sm font-normal ml-1" style={{ color: 'var(--text-muted)' }}>/ {total} {unit}</span>
                </p>
                <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'rgba(255,255,255,0.06)' }}>
                  <div
                    className="h-full rounded-full transition-all duration-500"
                    style={{
                      width: `${pct(used, total)}%`,
                      background: pct(used, total) >= 90 ? '#ef4444' :
                                  pct(used, total) >= 70 ? '#f59e0b' :
                                  'var(--primary)',
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        );
      })()}

      {/* Node grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => (
            <div key={i} className="card h-52 animate-pulse" />
          ))}
        </div>
      ) : nodes.length === 0 ? (
        <div className="card p-12 flex flex-col items-center text-center">
          <div className="w-14 h-14 rounded-2xl flex items-center justify-center mb-4"
            style={{ background: 'var(--bg-elevated)' }}>
            <Server className="w-7 h-7" style={{ color: 'var(--text-muted)' }} />
          </div>
          <h3 className="font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>No nodes registered</h3>
          <p className="text-sm max-w-xs" style={{ color: 'var(--text-secondary)' }}>
            Add a node to distribute game servers across multiple hosts.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {nodes.map(node => (
            <NodeCard
              key={node.id}
              node={node}
              onRemove={id => removeMutation.mutate(id)}
            />
          ))}
        </div>
      )}

      {showAdd && (
        <AddNodeDialog
          onClose={() => setShowAdd(false)}
          onAdd={req => registerMutation.mutate(req)}
          loading={registerMutation.isPending}
        />
      )}
    </div>
  );
}

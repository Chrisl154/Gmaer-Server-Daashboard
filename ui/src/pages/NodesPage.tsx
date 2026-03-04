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
import { clsx } from 'clsx';
import { api } from '../utils/api';
import type { Node, NodeCapacity, RegisterNodeRequest } from '../types';

// ── Helpers ───────────────────────────────────────────────────────────────────

function pct(used: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(100, Math.round((used / total) * 100));
}

function UtilBar({ used, total, label }: { used: number; total: number; label: string }) {
  const p = pct(used, total);
  const color = p >= 90 ? 'bg-red-500' : p >= 70 ? 'bg-yellow-500' : 'bg-blue-500';
  return (
    <div className="space-y-1">
      <div className="flex justify-between text-xs text-gray-400">
        <span>{label}</span>
        <span>{p}%</span>
      </div>
      <div className="h-1.5 bg-[#1f1f1f] rounded-full overflow-hidden">
        <div className={clsx('h-full rounded-full transition-all', color)} style={{ width: `${p}%` }} />
      </div>
    </div>
  );
}

const STATUS_STYLES: Record<string, string> = {
  online:   'bg-green-500/10 text-green-400 border border-green-500/20',
  offline:  'bg-red-500/10  text-red-400  border border-red-500/20',
  draining: 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20',
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-6 w-full max-w-md space-y-4">
        <h2 className="text-lg font-semibold text-gray-100">Register Node</h2>

        <div className="space-y-3">
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Hostname</label>
            <input
              className="w-full bg-[#0a0a0a] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
              placeholder="node-2.example.com"
              value={form.hostname}
              onChange={e => set('hostname', e.target.value)}
            />
          </div>
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Agent address (host:port)</label>
            <input
              className="w-full bg-[#0a0a0a] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
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
                <label className="text-xs text-gray-400 mb-1 block">{label}</label>
                <input
                  type="number"
                  min={0}
                  className="w-full bg-[#0a0a0a] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
                  value={form.capacity[key]}
                  onChange={e => setCap(key, parseFloat(e.target.value) || 0)}
                />
              </div>
            ))}
          </div>
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Version (optional)</label>
            <input
              className="w-full bg-[#0a0a0a] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
              placeholder="1.0.0"
              value={form.version ?? ''}
              onChange={e => set('version', e.target.value)}
            />
          </div>
        </div>

        <div className="flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-400 hover:text-gray-100 transition-colors"
          >
            Cancel
          </button>
          <button
            disabled={!form.hostname || !form.address || loading}
            onClick={() => onAdd(form)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition-colors"
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
    <div className="bg-[#111] border border-[#1f1f1f] rounded-xl p-5 space-y-4 hover:border-[#2a2a2a] transition-colors">
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className={clsx('w-8 h-8 rounded-lg flex items-center justify-center shrink-0',
            isOnline ? 'bg-blue-600/15' : 'bg-gray-700/30')}>
            <Server className={clsx('w-4 h-4', isOnline ? 'text-blue-400' : 'text-gray-500')} />
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium text-gray-100 truncate">{node.hostname}</p>
            <p className="text-xs text-gray-500 truncate">{node.address}</p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className={clsx('text-xs px-2 py-0.5 rounded-full', STATUS_STYLES[node.status])}>
            {node.status}
          </span>
          <button
            onClick={() => onRemove(node.id)}
            className="text-gray-600 hover:text-red-400 transition-colors"
            title="Remove node"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Utilisation bars */}
      <div className="space-y-2">
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
      <div className="flex items-center justify-between text-xs text-gray-500 pt-1 border-t border-[#1a1a1a]">
        <span className="flex items-center gap-1">
          <Activity className="w-3 h-3" />
          {node.server_count} server{node.server_count !== 1 ? 's' : ''}
        </span>
        {node.version && <span>v{node.version}</span>}
        <span title={new Date(node.last_seen).toLocaleString()}>
          {isOnline ? (
            <Wifi className="w-3 h-3 text-green-500" />
          ) : (
            <WifiOff className="w-3 h-3 text-red-500" />
          )}
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
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-gray-100">Cluster Nodes</h1>
          <p className="text-sm text-gray-400 mt-1">
            {nodes.length} node{nodes.length !== 1 ? 's' : ''} —{' '}
            <span className="text-green-400">{online} online</span>
            {offline > 0 && <>, <span className="text-red-400">{offline} offline</span></>}
          </p>
        </div>
        <button
          onClick={() => setShowAdd(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
        >
          <Plus className="w-4 h-4" /> Add Node
        </button>
      </div>

      {/* Summary stats */}
      {nodes.length > 0 && (() => {
        const totCPU = nodes.reduce((s, n) => s + n.capacity.cpu_cores, 0);
        const usedCPU = nodes.reduce((s, n) => s + n.allocated.cpu_cores, 0);
        const totMem = nodes.reduce((s, n) => s + n.capacity.memory_gb, 0);
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
              <div key={label} className="bg-[#111] border border-[#1f1f1f] rounded-xl p-4">
                <div className="flex items-center gap-2 mb-3">
                  <Icon className="w-4 h-4 text-blue-400" />
                  <span className="text-sm text-gray-400">{label}</span>
                </div>
                <p className="text-xl font-semibold text-gray-100">
                  {used} <span className="text-sm font-normal text-gray-500">/ {total} {unit}</span>
                </p>
                <div className="mt-2 h-1.5 bg-[#1f1f1f] rounded-full overflow-hidden">
                  <div
                    className={clsx('h-full rounded-full',
                      pct(used, total) >= 90 ? 'bg-red-500' :
                      pct(used, total) >= 70 ? 'bg-yellow-500' : 'bg-blue-500')}
                    style={{ width: `${pct(used, total)}%` }}
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
            <div key={i} className="h-52 bg-[#111] border border-[#1f1f1f] rounded-xl animate-pulse" />
          ))}
        </div>
      ) : nodes.length === 0 ? (
        <div className="text-center py-20 text-gray-500 text-sm">
          <Server className="w-10 h-10 mx-auto mb-3 text-gray-700" />
          <p className="font-medium text-gray-400 mb-1">No nodes registered</p>
          <p>Add a node to distribute game servers across multiple hosts.</p>
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

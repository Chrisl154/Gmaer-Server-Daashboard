// ServersPage.tsx
import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Search } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { ServerCard } from '../components/Dashboard/ServerCard';
import { api } from '../utils/api';

export function ServersPage() {
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
    refetchInterval: 15_000,
  });

  const servers = (data?.servers ?? []).filter((s: any) =>
    s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.adapter.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-gray-100">Servers</h1>
          <p className="text-sm text-gray-400 mt-1">{data?.count ?? 0} servers total</p>
        </div>
        <button onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium">
          <Plus className="w-4 h-4" /> Add Server
        </button>
      </div>

      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
        <input type="text" placeholder="Search servers..." value={search}
          onChange={e => setSearch(e.target.value)}
          className="w-full max-w-sm bg-[#141414] border border-[#252525] rounded-lg pl-9 pr-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500" />
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1,2,3].map(i => <div key={i} className="h-48 bg-gray-800 rounded-xl animate-pulse" />)}
        </div>
      ) : servers.length === 0 ? (
        <div className="text-center py-16 text-gray-500 text-sm">
          {search ? `No servers matching "${search}"` : 'No servers yet. Add your first server.'}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {servers.map((s: any) => <ServerCard key={s.id} server={s} />)}
        </div>
      )}

      {showCreate && <CreateServerModal onClose={() => setShowCreate(false)} onCreated={() => { setShowCreate(false); queryClient.invalidateQueries({ queryKey: ['servers'] }); }} />}
    </div>
  );
}

function CreateServerModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [form, setForm] = useState({ id: '', name: '', adapter: 'minecraft', deployMethod: 'manual', installDir: '/opt/games', dockerImage: '' });
  const [loading, setLoading] = useState(false);
  const ADAPTERS = ['valheim','minecraft','satisfactory','palworld','eco','enshrouded','riftbreaker'];
  const DEPLOY_METHODS = [
    { value: 'manual',   label: 'Manual (archive)' },
    { value: 'steamcmd', label: 'SteamCMD' },
    { value: 'docker',   label: 'Docker' },
  ];

  const handleCreate = async () => {
    setLoading(true);
    try {
      const config = form.deployMethod === 'docker' && form.dockerImage
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
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
      <div className="bg-[#141414] border border-[#252525] rounded-xl p-6 w-full max-w-md space-y-4">
        <h2 className="text-lg font-semibold text-gray-100">Add Server</h2>
        {(['id','name','installDir'] as const).map(field => (
          <div key={field}>
            <label className="block text-xs text-gray-400 mb-1 capitalize">{field === 'installDir' ? 'Install Directory' : field}</label>
            <input type="text" value={form[field]} onChange={e => setForm(p => ({...p, [field]: e.target.value}))}
              className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500" />
          </div>
        ))}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Adapter</label>
          <select value={form.adapter} onChange={e => setForm(p => ({...p, adapter: e.target.value}))}
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500">
            {ADAPTERS.map(a => <option key={a} value={a}>{a}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Deploy Method</label>
          <select value={form.deployMethod} onChange={e => setForm(p => ({...p, deployMethod: e.target.value}))}
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500">
            {DEPLOY_METHODS.map(m => <option key={m.value} value={m.value}>{m.label}</option>)}
          </select>
        </div>
        {form.deployMethod === 'docker' && (
          <div>
            <label className="block text-xs text-gray-400 mb-1">Docker Image <span className="text-gray-600">(leave blank to use adapter default)</span></label>
            <input type="text" value={form.dockerImage} onChange={e => setForm(p => ({...p, dockerImage: e.target.value}))}
              placeholder="e.g. itzg/minecraft-server:latest"
              className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 placeholder-gray-600 focus:outline-none focus:border-blue-500" />
          </div>
        )}
        <div className="flex gap-3 pt-2">
          <button onClick={onClose} className="flex-1 px-4 py-2 text-sm text-gray-300 bg-[#1a1a1a] hover:bg-[#252525] rounded-lg">Cancel</button>
          <button onClick={handleCreate} disabled={loading || !form.id || !form.name}
            className="flex-1 px-4 py-2 text-sm text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50">
            {loading ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  );
}

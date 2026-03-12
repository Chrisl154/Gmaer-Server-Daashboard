// ServersPage.tsx
import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Plus, Search } from 'lucide-react';
import { ServerCard } from '../components/Dashboard/ServerCard';
import { CreateServerWizard } from '../components/CreateServerWizard';
import { api } from '../utils/api';

export function ServersPage() {
  const [search, setSearch] = useState('');
  const [showCreate, setShowCreate] = useState(false);

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

      {showCreate && (
        <CreateServerWizard
          onClose={() => setShowCreate(false)}
          onCreated={() => setShowCreate(false)}
        />
      )}
    </div>
  );
}

// ModsPage.tsx
import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Package } from 'lucide-react';
import { api } from '../utils/api';

export function ModsPage() {
  const { data } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });
  const servers = (data?.servers ?? []).filter((s: any) => s.mod_manifest?.mods?.length > 0);

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Mods</h1>
        <p className="text-sm text-gray-400 mt-1">Manage mods across all servers. Supports Steam Workshop, CurseForge, Git, and local uploads.</p>
      </div>
      {servers.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-center">
          <div className="w-14 h-14 bg-[#1a1a1a] rounded-2xl flex items-center justify-center mb-4">
            <Package className="w-7 h-7 text-gray-500" />
          </div>
          <p className="text-gray-400 text-sm">No mods installed. Open a server and add mods from its detail page.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {servers.map((s: any) => (
            <div key={s.id} className="bg-[#141414] border border-[#252525] rounded-xl p-4">
              <h3 className="text-sm font-medium text-gray-200 mb-3">{s.name}</h3>
              <div className="space-y-2">
                {s.mod_manifest.mods.map((m: any) => (
                  <div key={m.id} className="flex items-center justify-between text-sm">
                    <div><span className="text-gray-200">{m.name}</span><span className="ml-2 text-xs text-gray-500 font-mono">{m.version}</span></div>
                    <span className="text-xs text-gray-500 capitalize">{m.source}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// PortsPage.tsx
import React, { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { Network, CheckCircle, XCircle, AlertTriangle } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';

export function PortsPage() {
  const { data } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });
  const servers = data?.servers ?? [];

  const validateMutation = useMutation({
    mutationFn: (ports: any[]) => api.post('/api/v1/ports/validate', { ports }),
    onSuccess: (r) => {
      const results = r.data?.results ?? [];
      const conflicts = results.filter((p: any) => !p.available);
      if (conflicts.length === 0) toast.success('All ports available!');
      else toast.error(`${conflicts.length} port conflict(s) detected`);
    },
  });

  const allPorts = servers.flatMap((s: any) =>
    (s.ports ?? []).map((p: any) => ({ ...p, server: s.name }))
  );

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-100">Ports</h1>
          <p className="text-sm text-gray-400 mt-1">View and validate port mappings across all servers</p>
        </div>
        <button
          onClick={() => validateMutation.mutate(allPorts)}
          disabled={validateMutation.isPending || allPorts.length === 0}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg text-sm disabled:opacity-50">
          <Network className="w-4 h-4" />
          {validateMutation.isPending ? 'Checking...' : 'Validate All'}
        </button>
      </div>

      {allPorts.length === 0 ? (
        <div className="text-center py-16 text-gray-500 text-sm">No ports configured.</div>
      ) : (
        <div className="bg-[#141414] border border-[#252525] rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#252525] text-xs text-gray-400">
                <th className="text-left px-4 py-3">Server</th>
                <th className="text-left px-4 py-3">Internal</th>
                <th className="text-left px-4 py-3">External</th>
                <th className="text-left px-4 py-3">Protocol</th>
                <th className="text-left px-4 py-3">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#1a1a1a]">
              {allPorts.map((p: any, i: number) => (
                <tr key={i} className="hover:bg-[#1a1a1a]/50 transition-colors">
                  <td className="px-4 py-3 text-gray-300">{p.server}</td>
                  <td className="px-4 py-3 font-mono text-gray-200">{p.internal}</td>
                  <td className="px-4 py-3 font-mono text-gray-200">{p.external}</td>
                  <td className="px-4 py-3 text-gray-400 uppercase text-xs">{p.protocol}</td>
                  <td className="px-4 py-3">
                    {p.exposed ? (
                      <span className="flex items-center gap-1 text-green-400 text-xs"><CheckCircle className="w-3.5 h-3.5" />Exposed</span>
                    ) : (
                      <span className="flex items-center gap-1 text-gray-500 text-xs"><XCircle className="w-3.5 h-3.5" />Internal</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

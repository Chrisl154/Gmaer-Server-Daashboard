// BackupsPage.tsx
import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { HardDrive, Download } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';

export function BackupsPage() {
  const { data } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });
  const servers = data?.servers ?? [];

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Backups</h1>
        <p className="text-sm text-gray-400 mt-1">Manage scheduled and manual backups</p>
      </div>
      {servers.length === 0 ? (
        <p className="text-gray-500 text-sm">No servers configured.</p>
      ) : (
        <div className="space-y-3">
          {servers.map((s: any) => (
            <div key={s.id} className="bg-[#141414] border border-[#252525] rounded-xl p-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <HardDrive className="w-5 h-5 text-gray-400" />
                <div>
                  <div className="text-sm text-gray-200">{s.name}</div>
                  <div className="text-xs text-gray-500">{s.backup_config?.schedule ?? 'No schedule'} · {s.backup_config?.target ?? 'No target'}</div>
                </div>
              </div>
              <button
                onClick={() => api.post(`/api/v1/servers/${s.id}/backup`, { type: 'full' }).then(() => toast.success('Backup started for ' + s.name))}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg">
                <Download className="w-3 h-3" /> Backup Now
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

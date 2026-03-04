import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { Play, Square, RotateCcw, Terminal, MoreVertical, Circle } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'react-hot-toast';
import { api } from '../../utils/api';
import { ADAPTER_ICONS } from '../../utils/adapters';
import { clsx } from 'clsx';

interface Server {
  id: string;
  name: string;
  adapter: string;
  state: string;
  ports: Array<{ internal: number; external: number; protocol: string }>;
  resources: { cpu_cores: number; ram_gb: number; disk_gb: number };
  last_started?: string;
}

const STATE_COLORS: Record<string, string> = {
  running:   'text-green-400',
  stopped:   'text-gray-400',
  starting:  'text-yellow-400',
  stopping:  'text-orange-400',
  deploying: 'text-blue-400',
  error:     'text-red-400',
  idle:      'text-gray-500',
};

const STATE_LABELS: Record<string, string> = {
  running:   'Running',
  stopped:   'Stopped',
  starting:  'Starting',
  stopping:  'Stopping',
  deploying: 'Deploying',
  error:     'Error',
  idle:      'Idle',
};

export function ServerCard({ server }: { server: Server }) {
  const [showMenu, setShowMenu] = useState(false);
  const queryClient = useQueryClient();

  const startMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/start`),
    onSuccess: () => {
      toast.success(`Starting ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to start server'),
  });

  const stopMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/stop`),
    onSuccess: () => {
      toast.success(`Stopping ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to stop server'),
  });

  const restartMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${server.id}/restart`),
    onSuccess: () => {
      toast.success(`Restarting ${server.name}...`);
      queryClient.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: () => toast.error('Failed to restart server'),
  });

  const isRunning = server.state === 'running';
  const isBusy = ['starting', 'stopping', 'deploying'].includes(server.state);
  const stateColor = STATE_COLORS[server.state] ?? 'text-gray-400';
  const AdapterIcon = ADAPTER_ICONS[server.adapter] ?? ADAPTER_ICONS.default;

  return (
    <div className="bg-[#141414] border border-[#252525] rounded-xl p-4 flex flex-col gap-3 hover:border-[#333] transition-colors group">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-10 h-10 bg-[#1e1e1e] rounded-lg flex items-center justify-center shrink-0">
            <AdapterIcon className="w-5 h-5 text-gray-300" />
          </div>
          <div className="min-w-0">
            <Link
              to={`/servers/${server.id}`}
              className="text-gray-100 font-medium text-sm hover:text-white truncate block"
            >
              {server.name}
            </Link>
            <span className="text-xs text-gray-500 capitalize">{server.adapter}</span>
          </div>
        </div>

        {/* Status badge */}
        <div className="flex items-center gap-1.5 shrink-0">
          <Circle
            className={clsx('w-2 h-2 fill-current', stateColor, isBusy && 'animate-pulse')}
          />
          <span className={clsx('text-xs', stateColor)}>{STATE_LABELS[server.state]}</span>
        </div>
      </div>

      {/* Ports */}
      {server.ports?.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {server.ports.slice(0, 4).map((p, i) => (
            <span
              key={i}
              className="text-xs bg-[#1e1e1e] text-gray-400 px-2 py-0.5 rounded-md font-mono"
            >
              {p.external}/{p.protocol}
            </span>
          ))}
        </div>
      )}

      {/* Resources */}
      {server.resources && (
        <div className="flex gap-3 text-xs text-gray-500">
          <span>{server.resources.cpu_cores} CPU</span>
          <span>{server.resources.ram_gb}GB RAM</span>
          <span>{server.resources.disk_gb}GB</span>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center gap-2 mt-1">
        {isRunning ? (
          <>
            <button
              onClick={() => stopMutation.mutate()}
              disabled={isBusy || stopMutation.isPending}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-red-900/30 hover:bg-red-900/50 text-red-400 rounded-lg transition-colors disabled:opacity-50"
            >
              <Square className="w-3 h-3" />
              Stop
            </button>
            <button
              onClick={() => restartMutation.mutate()}
              disabled={isBusy || restartMutation.isPending}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg transition-colors disabled:opacity-50"
            >
              <RotateCcw className="w-3 h-3" />
              Restart
            </button>
          </>
        ) : (
          <button
            onClick={() => startMutation.mutate()}
            disabled={isBusy || startMutation.isPending}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-green-900/30 hover:bg-green-900/50 text-green-400 rounded-lg transition-colors disabled:opacity-50"
          >
            <Play className="w-3 h-3" />
            Start
          </button>
        )}

        <Link
          to={`/servers/${server.id}#console`}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#1e1e1e] hover:bg-[#252525] text-gray-300 rounded-lg transition-colors"
        >
          <Terminal className="w-3 h-3" />
          Console
        </Link>

        <button
          className="ml-auto p-1.5 text-gray-500 hover:text-gray-300 hover:bg-[#1e1e1e] rounded-lg transition-colors"
          onClick={() => setShowMenu(!showMenu)}
        >
          <MoreVertical className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}

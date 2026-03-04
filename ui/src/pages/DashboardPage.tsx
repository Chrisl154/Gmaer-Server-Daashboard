import React, { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, Server, HardDrive, Shield, Package } from 'lucide-react';
import { ServerCard } from '../components/Dashboard/ServerCard';
import { StatsCard } from '../components/Dashboard/StatsCard';
import { api } from '../utils/api';
import {
  AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer,
} from 'recharts';

interface TrendPoint { time: string; running: number; }

export function DashboardPage() {
  const { data: serversData, isLoading } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
    refetchInterval: 15_000,
  });

  const { data: statusData } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => api.get('/api/v1/status').then(r => r.data),
    refetchInterval: 30_000,
  });

  const servers = serversData?.servers ?? [];
  const running = servers.filter((s: any) => s.state === 'running').length;
  const stopped = servers.filter((s: any) => s.state === 'stopped').length;

  // Accumulate running-server count history locally (no extra API call).
  const historyRef = useRef<TrendPoint[]>([]);
  const [, forceUpdate] = useState(0);
  useEffect(() => {
    if (!serversData) return;
    const point: TrendPoint = {
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      running,
    };
    historyRef.current = [...historyRef.current.slice(-19), point];
    forceUpdate(n => n + 1);
  }, [serversData]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Dashboard</h1>
        <p className="text-sm text-gray-400 mt-1">
          {servers.length} server{servers.length !== 1 ? 's' : ''} managed
        </p>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatsCard
          icon={<Server className="w-5 h-5" />}
          label="Total Servers"
          value={servers.length}
          color="blue"
        />
        <StatsCard
          icon={<Activity className="w-5 h-5" />}
          label="Running"
          value={running}
          color="green"
        />
        <StatsCard
          icon={<HardDrive className="w-5 h-5" />}
          label="Stopped"
          value={stopped}
          color="gray"
        />
        <StatsCard
          icon={<Shield className="w-5 h-5" />}
          label="System"
          value={statusData?.healthy ? 'Healthy' : 'Degraded'}
          color={statusData?.healthy ? 'green' : 'red'}
        />
      </div>

      {/* Running servers trend */}
      {historyRef.current.length > 1 && (
        <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
          <h2 className="text-xs font-medium text-gray-400 uppercase tracking-wider mb-3">
            Running Servers — Last {historyRef.current.length} polls
          </h2>
          <ResponsiveContainer width="100%" height={120}>
            <AreaChart data={historyRef.current} margin={{ top: 4, right: 8, bottom: 0, left: -20 }}>
              <defs>
                <linearGradient id="runningGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#6b7280' }} interval="preserveStartEnd" />
              <YAxis allowDecimals={false} tick={{ fontSize: 10, fill: '#6b7280' }} width={24} />
              <Tooltip
                contentStyle={{ background: '#141414', border: '1px solid #252525', borderRadius: 8, fontSize: 12 }}
                labelStyle={{ color: '#9ca3af' }}
              />
              <Area
                type="monotone"
                dataKey="running"
                stroke="#22c55e"
                strokeWidth={1.5}
                fill="url(#runningGrad)"
                dot={false}
                name="Running"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Server Cards */}
      <div>
        <h2 className="text-lg font-medium text-gray-200 mb-4">Servers</h2>
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-48 bg-gray-800 rounded-xl animate-pulse" />
            ))}
          </div>
        ) : servers.length === 0 ? (
          <EmptyServers />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {servers.map((server: any) => (
              <ServerCard key={server.id} server={server} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function EmptyServers() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="w-16 h-16 bg-gray-800 rounded-2xl flex items-center justify-center mb-4">
        <Server className="w-8 h-8 text-gray-500" />
      </div>
      <h3 className="text-gray-200 font-medium mb-2">No servers yet</h3>
      <p className="text-gray-400 text-sm max-w-xs">
        Add your first game server to get started. Supports Valheim, Minecraft, Satisfactory and more.
      </p>
      <button className="mt-6 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors">
        Add Server
      </button>
    </div>
  );
}

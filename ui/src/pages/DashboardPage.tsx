import React, { useEffect, useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, Server, HardDrive, Shield } from 'lucide-react';
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
  const healthy = statusData?.healthy;

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
    <div className="p-6 md:p-8 animate-page">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
          Dashboard
        </h1>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          {servers.length} server{servers.length !== 1 ? 's' : ''} managed &middot; System{' '}
          {healthy == null ? 'checking...' : healthy ? 'healthy' : 'degraded'}
        </p>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatsCard
          icon={Server}
          title="Total Servers"
          value={servers.length}
          color="blue"
        />
        <StatsCard
          icon={Activity}
          title="Running"
          value={running}
          color="green"
          trend={running > 0 ? 'Active' : undefined}
        />
        <StatsCard
          icon={HardDrive}
          title="Stopped"
          value={stopped}
          color="gray"
        />
        <StatsCard
          icon={Shield}
          title="System Health"
          value={healthy == null ? '...' : healthy ? 'Healthy' : 'Degraded'}
          color={healthy ? 'orange' : 'gray'}
          trend={healthy ? 'OK' : undefined}
        />
      </div>

      {/* Running servers trend chart */}
      {historyRef.current.length > 1 && (
        <div
          className="card p-5 mb-8"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                Server Activity
              </h2>
              <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                Running servers — last {historyRef.current.length} polls
              </p>
            </div>
            <span
              className="badge"
              style={{
                background: 'rgba(249,115,22,0.1)',
                color: '#fb923c',
                border: '1px solid rgba(249,115,22,0.2)',
              }}
            >
              Live
            </span>
          </div>
          <ResponsiveContainer width="100%" height={130}>
            <AreaChart
              data={historyRef.current}
              margin={{ top: 4, right: 8, bottom: 0, left: -20 }}
            >
              <defs>
                <linearGradient id="runningGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#f97316" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="time"
                tick={{ fontSize: 10, fill: 'var(--text-muted)' }}
                interval="preserveStartEnd"
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                allowDecimals={false}
                tick={{ fontSize: 10, fill: 'var(--text-muted)' }}
                width={24}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                contentStyle={{
                  background: 'var(--bg-card)',
                  border: '1px solid var(--border-strong)',
                  borderRadius: 10,
                  fontSize: 12,
                  color: 'var(--text-primary)',
                }}
                labelStyle={{ color: 'var(--text-secondary)' }}
                cursor={{ stroke: 'rgba(249,115,22,0.3)', strokeWidth: 1 }}
              />
              <Area
                type="monotone"
                dataKey="running"
                stroke="#f97316"
                strokeWidth={2}
                fill="url(#runningGrad)"
                dot={false}
                name="Running"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* Server Overview section */}
      <div>
        <div
          className="flex items-center justify-between pb-4 mb-6"
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <h2 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            Server Overview
          </h2>
          {servers.length > 0 && (
            <span
              className="badge"
              style={{
                background: 'rgba(249,115,22,0.1)',
                color: '#fb923c',
                border: '1px solid rgba(249,115,22,0.2)',
              }}
            >
              {servers.length} total
            </span>
          )}
        </div>

        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1, 2, 3].map(i => (
              <div
                key={i}
                className="card"
                style={{ height: 220, animation: 'pulse 1.5s ease-in-out infinite', opacity: 0.5 }}
              />
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
        Add your first game server to get started. Supports Valheim, Minecraft, Satisfactory and more.
      </p>
      <a href="/servers" className="btn-primary">
        Add Server
      </a>
    </div>
  );
}

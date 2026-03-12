// PortsPage.tsx
import React, { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { Network, CheckCircle, XCircle, ShieldCheck } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';
import { cn } from '../utils/cn';

export function PortsPage() {
  const { data } = useQuery({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
  });
  const servers = data?.servers ?? [];

  const [validationResults, setValidationResults] = useState<Record<string, boolean>>({});

  const validateMutation = useMutation({
    mutationFn: (ports: any[]) => api.post('/api/v1/ports/validate', { ports }),
    onSuccess: (r) => {
      const results: Record<string, boolean> = {};
      (r.data?.results ?? []).forEach((p: any) => {
        results[`${p.internal}-${p.protocol}`] = p.available;
      });
      setValidationResults(results);
      const conflicts = (r.data?.results ?? []).filter((p: any) => !p.available);
      if (conflicts.length === 0) toast.success('All ports available!');
      else toast.error(`${conflicts.length} port conflict(s) detected`);
    },
  });

  const allPorts = servers.flatMap((s: any) =>
    (s.ports ?? []).map((p: any) => ({ ...p, server: s.name, serverId: s.id }))
  );

  // Group ports by server
  const portsByServer: Record<string, { serverName: string; ports: any[] }> = {};
  servers.forEach((s: any) => {
    if ((s.ports ?? []).length > 0) {
      portsByServer[s.id] = { serverName: s.name, ports: s.ports };
    }
  });

  return (
    <div className="p-6 md:p-8 animate-page">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3 mb-6">
        <div>
          <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Port Mappings</h1>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            View and validate port mappings across all servers
          </p>
        </div>
        <button
          onClick={() => validateMutation.mutate(allPorts)}
          disabled={validateMutation.isPending || allPorts.length === 0}
          className="btn-blue"
        >
          <Network className="w-4 h-4" />
          {validateMutation.isPending ? 'Checking...' : 'Validate All'}
        </button>
      </div>

      {allPorts.length === 0 ? (
        <div className="card p-12 flex flex-col items-center text-center">
          <div className="w-14 h-14 rounded-2xl flex items-center justify-center mb-4"
            style={{ background: 'var(--bg-elevated)' }}>
            <Network className="w-7 h-7" style={{ color: 'var(--text-muted)' }} />
          </div>
          <h3 className="font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>No ports configured</h3>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Ports will appear here once servers are configured with port mappings.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {Object.entries(portsByServer).map(([serverId, { serverName, ports }]) => (
            <div key={serverId} className="card overflow-hidden">
              {/* Card header */}
              <div className="flex items-center gap-3 px-5 py-4"
                style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
                <div className="w-7 h-7 rounded-lg flex items-center justify-center"
                  style={{ background: 'var(--primary-subtle)' }}>
                  <Network className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
                </div>
                <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{serverName}</span>
                <span className="badge ml-1" style={{ background: 'rgba(128,128,168,0.12)', color: 'var(--text-muted)' }}>
                  {ports.length} port{ports.length !== 1 ? 's' : ''}
                </span>
              </div>

              {/* Port list */}
              <div>
                {ports.map((p: any, i: number) => {
                  const key = `${p.internal}-${p.protocol}`;
                  const validated = key in validationResults;
                  const available = validationResults[key];

                  return (
                    <div key={i}
                      className="flex items-center justify-between px-5 py-3.5 transition-colors"
                      style={{
                        borderBottom: i < ports.length - 1 ? '1px solid var(--border)' : undefined,
                      }}
                      onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
                      onMouseLeave={e => (e.currentTarget.style.background = '')}
                    >
                      {/* Port numbers */}
                      <div className="flex items-center gap-3">
                        <div className="flex items-center gap-2 font-mono text-sm">
                          <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>{p.internal}</span>
                          <span style={{ color: 'var(--text-muted)' }}>→</span>
                          <span style={{ color: 'var(--text-secondary)' }}>{p.external}</span>
                        </div>
                        <span className="badge uppercase text-[10px] font-bold tracking-wider"
                          style={{
                            background: p.protocol === 'tcp' ? 'rgba(59,130,246,0.12)' : 'rgba(168,85,247,0.12)',
                            color: p.protocol === 'tcp' ? '#60a5fa' : '#c084fc',
                          }}>
                          {p.protocol}
                        </span>
                      </div>

                      {/* Status indicators */}
                      <div className="flex items-center gap-3">
                        {/* Exposed status */}
                        <div className="flex items-center gap-1.5 text-xs">
                          <div className={cn(
                            'w-1.5 h-1.5 rounded-full',
                            p.exposed ? 'bg-green-400' : 'bg-gray-600'
                          )} />
                          <span style={{ color: p.exposed ? '#4ade80' : 'var(--text-muted)' }}>
                            {p.exposed ? 'Exposed' : 'Internal'}
                          </span>
                        </div>

                        {/* Validation result */}
                        {validated && (
                          <div className={cn(
                            'flex items-center gap-1 text-xs badge',
                            available
                              ? 'bg-green-500/12 text-green-400'
                              : 'bg-red-500/12 text-red-400'
                          )}>
                            {available
                              ? <><CheckCircle className="w-3 h-3" /> Available</>
                              : <><XCircle className="w-3 h-3" /> Conflict</>
                            }
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}

          {/* Flat table fallback if no server grouping */}
          {Object.keys(portsByServer).length === 0 && allPorts.length > 0 && (
            <div className="card overflow-hidden">
              <div className="grid grid-cols-12 gap-4 px-5 py-3 text-xs font-semibold uppercase tracking-wider"
                style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
                <div className="col-span-3">Server</div>
                <div className="col-span-2">Internal</div>
                <div className="col-span-2">External</div>
                <div className="col-span-2">Protocol</div>
                <div className="col-span-3">Status</div>
              </div>
              {allPorts.map((p: any, i: number) => (
                <div key={i}
                  className="grid grid-cols-12 gap-4 px-5 py-3 items-center text-sm transition-colors"
                  style={{ borderBottom: i < allPorts.length - 1 ? '1px solid var(--border)' : undefined }}
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
                  onMouseLeave={e => (e.currentTarget.style.background = '')}
                >
                  <div className="col-span-3" style={{ color: 'var(--text-secondary)' }}>{p.server}</div>
                  <div className="col-span-2 font-mono font-semibold" style={{ color: 'var(--text-primary)' }}>{p.internal}</div>
                  <div className="col-span-2 font-mono" style={{ color: 'var(--text-secondary)' }}>{p.external}</div>
                  <div className="col-span-2">
                    <span className="badge uppercase text-[10px]"
                      style={{
                        background: p.protocol === 'tcp' ? 'rgba(59,130,246,0.12)' : 'rgba(168,85,247,0.12)',
                        color: p.protocol === 'tcp' ? '#60a5fa' : '#c084fc',
                      }}>
                      {p.protocol}
                    </span>
                  </div>
                  <div className="col-span-3">
                    {p.exposed ? (
                      <span className="flex items-center gap-1 text-xs text-green-400">
                        <CheckCircle className="w-3.5 h-3.5" /> Exposed
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                        <XCircle className="w-3.5 h-3.5" /> Internal
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

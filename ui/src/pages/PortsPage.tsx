// PortsPage.tsx
import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Network, CheckCircle, XCircle, ShieldCheck, ShieldOff,
  Shield, Trash2, Plus, Loader2, AlertTriangle,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';
import { cn } from '../utils/cn';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface FirewallRule {
  num: number;
  to: string;
  action: string;
  from: string;
  comment: string;
  v6: boolean;
}

interface FirewallStatus {
  available: boolean;
  enabled: boolean;
  rules: FirewallRule[];
}

// ---------------------------------------------------------------------------
// Firewall panel
// ---------------------------------------------------------------------------
function FirewallPanel() {
  const qc = useQueryClient();

  const { data: fw, isLoading, isError } = useQuery<FirewallStatus>({
    queryKey: ['firewall'],
    queryFn: () => api.get('/api/v1/firewall').then(r => r.data),
    refetchInterval: 15_000,
  });

  const [form, setForm] = useState({ port: '', proto: 'tcp', from: '', comment: '' });

  const addMutation = useMutation({
    mutationFn: (body: { port: number; proto: string; from: string; comment: string }) =>
      api.post('/api/v1/firewall/rules', body),
    onSuccess: () => {
      toast.success('Firewall rule added');
      setForm({ port: '', proto: 'tcp', from: '', comment: '' });
      qc.invalidateQueries({ queryKey: ['firewall'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Failed to add rule'),
  });

  const deleteMutation = useMutation({
    mutationFn: (num: number) => api.delete(`/api/v1/firewall/rules/${num}`),
    onSuccess: () => {
      toast.success('Rule removed');
      qc.invalidateQueries({ queryKey: ['firewall'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Failed to delete rule'),
  });

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => api.post('/api/v1/firewall/enabled', { enabled }),
    onSuccess: (_d, enabled) => {
      toast.success(enabled ? 'Firewall enabled' : 'Firewall disabled');
      qc.invalidateQueries({ queryKey: ['firewall'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? 'Failed to toggle firewall'),
  });

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault();
    const port = parseInt(form.port, 10);
    if (!port || port < 1 || port > 65535) {
      toast.error('Enter a valid port number (1–65535)');
      return;
    }
    addMutation.mutate({ port, proto: form.proto, from: form.from.trim(), comment: form.comment.trim() });
  };

  // ── Loading / error / unavailable states ──────────────────────────────────
  if (isLoading) {
    return (
      <div className="card p-8 flex items-center justify-center gap-2" style={{ color: 'var(--text-muted)' }}>
        <Loader2 className="w-4 h-4 animate-spin" />
        <span className="text-sm">Loading firewall status…</span>
      </div>
    );
  }

  if (isError || !fw) {
    return (
      <div className="card p-8 flex items-center gap-3" style={{ color: 'var(--text-secondary)' }}>
        <AlertTriangle className="w-5 h-5 text-yellow-400" />
        <span className="text-sm">Could not reach the firewall API.</span>
      </div>
    );
  }

  if (!fw.available) {
    return (
      <div className="card p-8 flex flex-col items-center text-center gap-3">
        <ShieldOff className="w-8 h-8" style={{ color: 'var(--text-muted)' }} />
        <div>
          <p className="font-semibold" style={{ color: 'var(--text-primary)' }}>UFW not installed</p>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            Install ufw on the host (<code className="font-mono">apt install ufw</code>) and the daemon will pick it up automatically.
          </p>
        </div>
      </div>
    );
  }

  const ipv4Rules = (fw.rules ?? []).filter(r => !r.v6);

  return (
    <div className="card overflow-hidden">
      {/* ── Header ──────────────────────────────────────────────────────── */}
      <div className="flex items-center justify-between px-5 py-4"
        style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>
        <div className="flex items-center gap-3">
          <div className="w-7 h-7 rounded-lg flex items-center justify-center"
            style={{ background: fw.enabled ? 'rgba(34,197,94,0.12)' : 'rgba(239,68,68,0.12)' }}>
            {fw.enabled
              ? <ShieldCheck className="w-3.5 h-3.5 text-green-400" />
              : <ShieldOff className="w-3.5 h-3.5 text-red-400" />}
          </div>
          <div>
            <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              Firewall (UFW)
            </span>
            <span className={cn(
              'ml-2 badge text-xs',
              fw.enabled ? 'bg-green-500/12 text-green-400' : 'bg-red-500/12 text-red-400'
            )}>
              {fw.enabled ? 'Active' : 'Inactive'}
            </span>
          </div>
        </div>

        {/* Enable / Disable toggle */}
        <button
          onClick={() => toggleMutation.mutate(!fw.enabled)}
          disabled={toggleMutation.isPending}
          className={cn('btn text-xs px-3 py-1.5', fw.enabled ? 'btn-red' : 'btn-green')}
        >
          {toggleMutation.isPending
            ? <Loader2 className="w-3 h-3 animate-spin" />
            : fw.enabled ? <><ShieldOff className="w-3 h-3" /> Disable</> : <><ShieldCheck className="w-3 h-3" /> Enable</>}
        </button>
      </div>

      {/* ── Rule list ───────────────────────────────────────────────────── */}
      {ipv4Rules.length === 0 ? (
        <div className="px-5 py-6 text-sm text-center" style={{ color: 'var(--text-muted)' }}>
          No rules configured. Add one below.
        </div>
      ) : (
        <div>
          {/* Column headers */}
          <div className="grid grid-cols-12 gap-2 px-5 py-2 text-xs font-semibold uppercase tracking-wider"
            style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
            <div className="col-span-1">#</div>
            <div className="col-span-3">Port / Service</div>
            <div className="col-span-2">Action</div>
            <div className="col-span-3">Source</div>
            <div className="col-span-2">Comment</div>
            <div className="col-span-1"></div>
          </div>

          {ipv4Rules.map((rule, i) => (
            <div key={rule.num}
              className="grid grid-cols-12 gap-2 px-5 py-3 items-center text-sm transition-colors"
              style={{ borderBottom: i < ipv4Rules.length - 1 ? '1px solid var(--border)' : undefined }}
              onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
              onMouseLeave={e => (e.currentTarget.style.background = '')}
            >
              <div className="col-span-1 text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                {rule.num}
              </div>
              <div className="col-span-3 font-mono font-semibold" style={{ color: 'var(--text-primary)' }}>
                {rule.to}
              </div>
              <div className="col-span-2">
                <span className={cn(
                  'badge text-xs uppercase font-bold',
                  rule.action === 'ALLOW'
                    ? 'bg-green-500/12 text-green-400'
                    : 'bg-red-500/12 text-red-400'
                )}>
                  {rule.action}
                </span>
              </div>
              <div className="col-span-3 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                {rule.from || 'Anywhere'}
              </div>
              <div className="col-span-2 text-xs truncate" style={{ color: 'var(--text-muted)' }}>
                {rule.comment}
              </div>
              <div className="col-span-1 flex justify-end">
                <button
                  onClick={() => deleteMutation.mutate(rule.num)}
                  disabled={deleteMutation.isPending}
                  className="p-1 rounded hover:bg-red-500/12 transition-colors"
                  title="Delete rule"
                >
                  {deleteMutation.isPending
                    ? <Loader2 className="w-3.5 h-3.5 animate-spin text-red-400" />
                    : <Trash2 className="w-3.5 h-3.5 text-red-400" />}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ── Add rule form ────────────────────────────────────────────────── */}
      <form onSubmit={handleAdd}
        className="px-5 py-4 flex flex-wrap gap-2 items-end"
        style={{ borderTop: '1px solid var(--border)', background: 'var(--bg-elevated)' }}>

        <div className="flex flex-col gap-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Port *</label>
          <input
            type="number" min={1} max={65535} placeholder="e.g. 25565"
            value={form.port}
            onChange={e => setForm(f => ({ ...f, port: e.target.value }))}
            className="input w-28 font-mono text-sm"
            required
          />
        </div>

        <div className="flex flex-col gap-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Protocol</label>
          <select
            value={form.proto}
            onChange={e => setForm(f => ({ ...f, proto: e.target.value }))}
            className="input w-20 text-sm"
          >
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
          </select>
        </div>

        <div className="flex flex-col gap-1">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Source CIDR <span style={{ color: 'var(--text-muted)' }}>(blank = Anywhere)</span></label>
          <input
            type="text" placeholder="e.g. 192.168.1.0/24"
            value={form.from}
            onChange={e => setForm(f => ({ ...f, from: e.target.value }))}
            className="input w-44 font-mono text-sm"
          />
        </div>

        <div className="flex flex-col gap-1 flex-1 min-w-32">
          <label className="text-xs" style={{ color: 'var(--text-muted)' }}>Comment</label>
          <input
            type="text" placeholder="e.g. Minecraft server"
            value={form.comment}
            onChange={e => setForm(f => ({ ...f, comment: e.target.value }))}
            className="input text-sm"
          />
        </div>

        <button
          type="submit"
          disabled={addMutation.isPending || !form.port}
          className="btn-blue flex items-center gap-1.5 self-end"
        >
          {addMutation.isPending
            ? <Loader2 className="w-3.5 h-3.5 animate-spin" />
            : <Plus className="w-3.5 h-3.5" />}
          Add Rule
        </button>
      </form>

      {/* ── Hint ────────────────────────────────────────────────────────── */}
      <div className="px-5 py-3 text-xs" style={{ borderTop: '1px solid var(--border)', color: 'var(--text-muted)' }}>
        <Shield className="w-3 h-3 inline mr-1 opacity-60" />
        SSH rules and dashboard ports are managed by the installer. Only IPv4 rules are shown — IPv6 mirrors are created automatically by UFW.
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------
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

  const portsByServer: Record<string, { serverName: string; ports: any[] }> = {};
  servers.forEach((s: any) => {
    if ((s.ports ?? []).length > 0) {
      portsByServer[s.id] = { serverName: s.name, ports: s.ports };
    }
  });

  return (
    <div className="p-6 md:p-8 animate-page space-y-8">

      {/* ── Server Port Mappings ─────────────────────────────────────────── */}
      <section>
        <div className="flex items-center justify-between flex-wrap gap-3 mb-4">
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
            {validateMutation.isPending ? 'Checking…' : 'Validate All'}
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
                <div>
                  {ports.map((p: any, i: number) => {
                    const key = `${p.internal}-${p.protocol}`;
                    const validated = key in validationResults;
                    const available = validationResults[key];
                    return (
                      <div key={i}
                        className="flex items-center justify-between px-5 py-3.5 transition-colors"
                        style={{ borderBottom: i < ports.length - 1 ? '1px solid var(--border)' : undefined }}
                        onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
                        onMouseLeave={e => (e.currentTarget.style.background = '')}
                      >
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
                        <div className="flex items-center gap-3">
                          <div className="flex items-center gap-1.5 text-xs">
                            <div className={cn('w-1.5 h-1.5 rounded-full', p.exposed ? 'bg-green-400' : 'bg-gray-600')} />
                            <span style={{ color: p.exposed ? '#4ade80' : 'var(--text-muted)' }}>
                              {p.exposed ? 'Exposed' : 'Internal'}
                            </span>
                          </div>
                          {validated && (
                            <div className={cn(
                              'flex items-center gap-1 text-xs badge',
                              available ? 'bg-green-500/12 text-green-400' : 'bg-red-500/12 text-red-400'
                            )}>
                              {available
                                ? <><CheckCircle className="w-3 h-3" /> Available</>
                                : <><XCircle className="w-3 h-3" /> Conflict</>}
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* ── Firewall (UFW) ───────────────────────────────────────────────── */}
      <section>
        <div className="mb-4">
          <h2 className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>Firewall Rules</h2>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            Manage UFW rules on the host — open ports for game servers without SSH access.
          </p>
        </div>
        <FirewallPanel />
      </section>

    </div>
  );
}

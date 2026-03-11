import React from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { Shield, RefreshCw, CheckCircle, AlertTriangle, Clock, ExternalLink, FileText } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';
import { cn } from '../utils/cn';

export function SBOMPage() {
  const { data: sbom, isLoading: sbomLoading, refetch: refetchSBOM } = useQuery({
    queryKey: ['sbom'],
    queryFn: () => api.get('/api/v1/sbom').then(r => r.data),
  });
  const { data: cveReport, isLoading: cveLoading, refetch: refetchCVE } = useQuery({
    queryKey: ['cve-report'],
    queryFn: () => api.get('/api/v1/cve-report').then(r => r.data),
  });

  const scanMutation = useMutation({
    mutationFn: () => api.post('/api/v1/sbom/scan'),
    onSuccess: () => {
      toast.success('CVE scan started');
      setTimeout(() => { refetchSBOM(); refetchCVE(); }, 5000);
    },
    onError: () => toast.error('Scan failed'),
  });

  const summary = {
    critical: cveReport?.critical ?? 0,
    high:     cveReport?.high     ?? 0,
    medium:   cveReport?.medium   ?? 0,
    low:      cveReport?.low      ?? 0,
  };
  const isClean = summary.critical === 0 && summary.high === 0;

  return (
    <div className="p-6 md:p-8 animate-page space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3 mb-6">
        <div>
          <h1 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>SBOM &amp; CVE</h1>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            Software Bill of Materials and vulnerability status
          </p>
        </div>
        <button
          onClick={() => scanMutation.mutate()}
          disabled={scanMutation.isPending}
          className="btn-blue"
        >
          <RefreshCw className={cn('w-4 h-4', scanMutation.isPending && 'animate-spin')} />
          {scanMutation.isPending ? 'Scanning...' : 'Scan Now'}
        </button>
      </div>

      {/* Status banner */}
      <div className={cn(
        'flex items-center gap-4 p-4 rounded-xl',
        isClean
          ? 'bg-green-500/8 border border-green-500/20'
          : 'bg-red-500/8 border border-red-500/20'
      )}>
        <div className={cn(
          'w-10 h-10 rounded-xl flex items-center justify-center shrink-0',
          isClean ? 'bg-green-500/15' : 'bg-red-500/15'
        )}>
          {isClean
            ? <CheckCircle className="w-5 h-5 text-green-400" />
            : <AlertTriangle className="w-5 h-5 text-red-400" />
          }
        </div>
        <div>
          <p className={cn('font-semibold', isClean ? 'text-green-400' : 'text-red-400')}>
            {isClean ? 'No Critical or High CVEs detected' : 'Vulnerabilities found — action required'}
          </p>
          <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
            Last scan: {cveReport?.generated_at ?? 'Never'} · Scanner: {cveReport?.scanner ?? 'trivy'}
          </p>
        </div>
      </div>

      {/* Summary + SBOM info cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* SBOM Summary */}
        <div className="card p-5 space-y-4">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: 'rgba(59,130,246,0.12)' }}>
              <FileText className="w-3.5 h-3.5 text-blue-400" />
            </div>
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>SBOM Summary</h3>
          </div>
          <div className="space-y-2">
            {[
              { label: 'Format',     value: sbom?.bomFormat ?? 'CycloneDX' },
              { label: 'Spec Version', value: sbom?.specVersion ?? '—' },
              { label: 'Components', value: String((sbom?.components ?? []).length) },
              { label: 'Serial #',   value: sbom?.serialNumber ?? '—' },
            ].map(({ label, value }) => (
              <div key={label} className="flex items-center justify-between rounded-lg px-3 py-2"
                style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{label}</span>
                <span className="text-sm font-medium font-mono" style={{ color: 'var(--text-primary)' }}>{value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* CVE Severity Breakdown */}
        <div className="card p-5 space-y-4">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: 'var(--primary-subtle)' }}>
              <Shield className="w-3.5 h-3.5" style={{ color: 'var(--primary)' }} />
            </div>
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>CVE Report</h3>
          </div>
          <div className="grid grid-cols-2 gap-3">
            {[
              { label: 'Critical', value: summary.critical, bg: 'rgba(239,68,68,0.12)',  color: '#f87171',  ring: 'rgba(239,68,68,0.3)'  },
              { label: 'High',     value: summary.high,     bg: 'rgba(249,115,22,0.12)', color: '#fb923c',  ring: 'rgba(249,115,22,0.3)' },
              { label: 'Medium',   value: summary.medium,   bg: 'rgba(234,179,8,0.12)',  color: '#facc15',  ring: 'rgba(234,179,8,0.3)'  },
              { label: 'Low',      value: summary.low,      bg: 'rgba(59,130,246,0.12)', color: '#60a5fa',  ring: 'rgba(59,130,246,0.3)' },
            ].map(({ label, value, bg, color, ring }) => (
              <div key={label} className="rounded-xl p-4 flex items-center gap-3"
                style={{ background: bg, border: `1px solid ${ring}` }}>
                <span className="text-3xl font-bold" style={{ color }}>{value}</span>
                <span className="text-xs font-semibold uppercase tracking-wide" style={{ color }}>{label}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Evidence Model */}
      {cveReport?.evidence && (
        <div className="card p-5 space-y-4">
          <div className="flex items-center gap-2">
            <Shield className="w-4 h-4 text-green-400" />
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>CVE-Free Evidence</h3>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <EvidenceRow icon={<Clock className="w-3 h-3" />}        label="Last Checked"   value={cveReport.evidence.last_checked} />
            <EvidenceRow icon={<Shield className="w-3 h-3" />}       label="Status"         value={cveReport.evidence.cve_status} />
            <EvidenceRow icon={<Shield className="w-3 h-3" />}       label="Scanner Hash"   value={cveReport.evidence.scanner_hash} mono />
            <EvidenceRow icon={<ExternalLink className="w-3 h-3" />} label="Authoritative"  value={cveReport.evidence.authoritative_link} link />
          </div>
        </div>
      )}

      {/* Findings table */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4"
          style={{ borderBottom: '1px solid var(--border)' }}>
          <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>SBOM Components</h3>
          <span className="badge text-xs"
            style={{ background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
            {sbom?.specVersion ? `CycloneDX ${sbom.specVersion}` : 'CycloneDX'}
          </span>
        </div>

        {sbomLoading ? (
          <div className="p-5 space-y-2">
            {[1, 2, 3, 4].map(i => (
              <div key={i} className="h-10 rounded-lg animate-pulse" style={{ background: 'var(--bg-elevated)' }} />
            ))}
          </div>
        ) : (
          <div>
            {(sbom?.components ?? []).length === 0 ? (
              <div className="p-10 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
                No components in SBOM yet. Run a scan to populate.
              </div>
            ) : (
              <>
                {/* Table header */}
                <div className="grid grid-cols-12 gap-4 px-5 py-3 text-xs font-semibold uppercase tracking-wider"
                  style={{ borderBottom: '1px solid var(--border)', background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
                  <div className="col-span-4">Package</div>
                  <div className="col-span-2">Version</div>
                  <div className="col-span-2">Type</div>
                  <div className="col-span-3">CVE ID</div>
                  <div className="col-span-1 text-right">Status</div>
                </div>
                <div>
                  {(sbom.components).map((c: any, i: number) => (
                    <div key={i}
                      className="grid grid-cols-12 gap-4 px-5 py-3 items-center text-sm transition-colors"
                      style={{ borderBottom: i < sbom.components.length - 1 ? '1px solid var(--border)' : undefined }}
                      onMouseEnter={e => (e.currentTarget.style.background = 'var(--bg-card-hover)')}
                      onMouseLeave={e => (e.currentTarget.style.background = '')}
                    >
                      <div className="col-span-4 font-medium truncate" style={{ color: 'var(--text-primary)' }}>{c.name}</div>
                      <div className="col-span-2 font-mono text-xs" style={{ color: 'var(--text-muted)' }}>{c.version}</div>
                      <div className="col-span-2 capitalize text-xs" style={{ color: 'var(--text-secondary)' }}>{c.type}</div>
                      <div className="col-span-3 font-mono text-xs" style={{ color: 'var(--text-muted)' }}>—</div>
                      <div className="col-span-1 flex justify-end">
                        <CheckCircle className="w-4 h-4 text-green-500" />
                      </div>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function EvidenceRow({ icon, label, value, mono = false, link = false }:
  { icon: React.ReactNode; label: string; value: string; mono?: boolean; link?: boolean }) {
  return (
    <div className="flex items-start gap-3 rounded-lg p-3"
      style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
      <span className="mt-0.5" style={{ color: 'var(--text-muted)' }}>{icon}</span>
      <div className="min-w-0">
        <div className="text-xs mb-0.5" style={{ color: 'var(--text-muted)' }}>{label}</div>
        {link ? (
          <a href={value} target="_blank" rel="noreferrer"
            className="text-sm text-blue-400 hover:underline truncate block">{value}</a>
        ) : (
          <div className={cn('text-sm truncate', mono && 'font-mono')} style={{ color: 'var(--text-primary)' }}>{value}</div>
        )}
      </div>
    </div>
  );
}

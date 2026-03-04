import React from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { Shield, RefreshCw, CheckCircle, AlertTriangle, Clock, ExternalLink } from 'lucide-react';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';
import { clsx } from 'clsx';

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
    onSuccess: () => { toast.success('CVE scan started'); setTimeout(() => { refetchSBOM(); refetchCVE(); }, 5000); },
    onError:   () => toast.error('Scan failed'),
  });

  const summary = cveReport?.summary ?? { critical: 0, high: 0, medium: 0, low: 0 };
  const isClean = summary.critical === 0 && summary.high === 0;

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-100">SBOM &amp; CVE</h1>
          <p className="text-sm text-gray-400 mt-1">Software Bill of Materials and vulnerability status</p>
        </div>
        <button onClick={() => scanMutation.mutate()} disabled={scanMutation.isPending}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg text-sm transition-colors disabled:opacity-50">
          <RefreshCw className={clsx('w-4 h-4', scanMutation.isPending && 'animate-spin')} />
          {scanMutation.isPending ? 'Scanning...' : 'Run Scan'}
        </button>
      </div>

      {/* CVE Status Banner */}
      <div className={clsx('flex items-center gap-3 p-4 rounded-xl border',
        isClean ? 'bg-green-900/10 border-green-900/20' : 'bg-red-900/10 border-red-900/20')}>
        {isClean
          ? <CheckCircle className="w-6 h-6 text-green-400 shrink-0" />
          : <AlertTriangle className="w-6 h-6 text-red-400 shrink-0" />}
        <div>
          <p className={clsx('font-medium', isClean ? 'text-green-300' : 'text-red-300')}>
            {isClean ? 'No Critical or High CVEs detected' : 'Vulnerabilities found — action required'}
          </p>
          <p className="text-xs text-gray-400 mt-0.5">
            Last scan: {cveReport?.generated_at ?? 'Never'} · Scanner: {cveReport?.scanner ?? 'trivy'}
          </p>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {[
          { label: 'Critical', value: summary.critical, color: 'text-red-400'    },
          { label: 'High',     value: summary.high,     color: 'text-orange-400' },
          { label: 'Medium',   value: summary.medium,   color: 'text-yellow-400' },
          { label: 'Low',      value: summary.low,      color: 'text-blue-400'   },
        ].map(({ label, value, color }) => (
          <div key={label} className="bg-[#141414] border border-[#252525] rounded-xl p-4">
            <div className={clsx('text-2xl font-semibold', color)}>{value}</div>
            <div className="text-xs text-gray-400 mt-1">{label}</div>
          </div>
        ))}
      </div>

      {/* Evidence Model */}
      {cveReport?.evidence && (
        <div className="bg-[#141414] border border-[#252525] rounded-xl p-4 space-y-3">
          <h3 className="text-sm font-medium text-gray-200 flex items-center gap-2">
            <Shield className="w-4 h-4 text-green-400" /> CVE-Free Evidence
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
            <EvidenceRow icon={<Clock className="w-3 h-3" />}    label="Last Checked" value={cveReport.evidence.last_checked} />
            <EvidenceRow icon={<Shield className="w-3 h-3" />}   label="Status"       value={cveReport.evidence.cve_status} />
            <EvidenceRow icon={<Shield className="w-3 h-3" />}   label="Scanner Hash" value={cveReport.evidence.scanner_hash} mono />
            <EvidenceRow icon={<ExternalLink className="w-3 h-3" />} label="Authoritative" value={cveReport.evidence.authoritative_link} link />
          </div>
        </div>
      )}

      {/* SBOM Components */}
      <div className="bg-[#141414] border border-[#252525] rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-[#252525] flex items-center justify-between">
          <h3 className="text-sm font-medium text-gray-200">SBOM Components</h3>
          <span className="text-xs text-gray-500">
            {sbom?.specVersion ? `CycloneDX ${sbom.specVersion}` : 'CycloneDX'}
          </span>
        </div>
        {sbomLoading ? (
          <div className="p-4 space-y-2">
            {[1,2,3].map(i => <div key={i} className="h-10 bg-gray-800 rounded animate-pulse" />)}
          </div>
        ) : (
          <div className="divide-y divide-[#1a1a1a]">
            {(sbom?.components ?? []).length === 0 ? (
              <div className="p-6 text-center text-gray-500 text-sm">No components in SBOM yet. Run a scan to populate.</div>
            ) : (
              (sbom.components).map((c: any, i: number) => (
                <div key={i} className="px-4 py-3 flex items-center justify-between text-sm">
                  <div>
                    <span className="text-gray-200">{c.name}</span>
                    <span className="ml-2 text-xs text-gray-500 font-mono">{c.version}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-gray-500 capitalize">{c.type}</span>
                    <CheckCircle className="w-4 h-4 text-green-500" />
                  </div>
                </div>
              ))
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
    <div className="flex items-start gap-2 p-2 bg-[#0d0d0d] rounded-lg">
      <span className="text-gray-500 mt-0.5">{icon}</span>
      <div className="min-w-0">
        <div className="text-gray-500">{label}</div>
        {link ? (
          <a href={value} target="_blank" rel="noreferrer"
            className="text-blue-400 hover:underline truncate block">{value}</a>
        ) : (
          <div className={clsx('text-gray-300 truncate', mono && 'font-mono')}>{value}</div>
        )}
      </div>
    </div>
  );
}

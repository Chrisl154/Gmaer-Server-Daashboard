import React from 'react';
import { Settings, Info } from 'lucide-react';

export function SettingsPage() {
  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-gray-100">Settings</h1>
        <p className="text-sm text-gray-400 mt-1">System configuration and preferences</p>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 max-w-2xl">
        {[
          { title: 'General', desc: 'Installer mode, install directory, log level' },
          { title: 'Authentication', desc: 'Local auth, OIDC, SAML, OAuth2 providers' },
          { title: 'TLS Certificates', desc: 'Provide or auto-generate TLS certificates' },
          { title: 'Storage', desc: 'NFS mounts, S3 endpoints, backup targets' },
          { title: 'Networking', desc: 'Port exposure, UPnP/NAT, firewall rules' },
          { title: 'Monitoring', desc: 'Prometheus, Grafana, alerting rules' },
        ].map(({ title, desc }) => (
          <div key={title} className="bg-[#141414] border border-[#252525] rounded-xl p-4 hover:border-[#333] transition-colors cursor-pointer">
            <div className="flex items-center gap-3 mb-2">
              <Settings className="w-4 h-4 text-gray-400" />
              <h3 className="text-sm font-medium text-gray-200">{title}</h3>
            </div>
            <p className="text-xs text-gray-500">{desc}</p>
          </div>
        ))}
      </div>
      <div className="flex items-start gap-3 p-3 bg-[#141414] border border-[#252525] rounded-xl max-w-2xl">
        <Info className="w-4 h-4 text-blue-400 shrink-0 mt-0.5" />
        <p className="text-xs text-gray-400">
          Advanced configuration can be edited directly in <code className="font-mono text-gray-300">/etc/games-dashboard/daemon.yaml</code>.
          Restart the daemon after changes.
        </p>
      </div>
    </div>
  );
}

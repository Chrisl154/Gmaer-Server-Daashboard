import { describe, it, expect } from 'vitest';
import type { Server, Backup, Mod, CVEReport, User } from './index';

/**
 * Runtime shape checks — ensures that objects matching our TypeScript types
 * satisfy the expected field contracts.  These catch regressions where a field
 * is renamed or removed from the type definition.
 */

describe('Server type shape', () => {
  it('required fields exist on a minimal server object', () => {
    const server: Server = {
      id: 'srv-1',
      name: 'Test Server',
      adapter: 'valheim',
      state: 'idle',
      deploy_method: 'manual',
      install_dir: '/opt/valheim',
      ports: [],
      config: {},
      resources: { cpu_cores: 2, ram_gb: 4, disk_gb: 20 },
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    };
    expect(server.id).toBe('srv-1');
    expect(server.adapter).toBe('valheim');
    expect(server.state).toBe('idle');
    expect(Array.isArray(server.ports)).toBe(true);
  });
});

describe('Backup type shape', () => {
  it('required fields exist on a minimal backup object', () => {
    const backup: Backup = {
      id: 'bkp-1',
      server_id: 'srv-1',
      type: 'full',
      target: 's3://bucket/bkp-1',
      size_bytes: 1024,
      checksum: 'sha256:abc',
      created_at: '2024-01-01T00:00:00Z',
      status: 'complete',
    };
    expect(backup.type).toBe('full');
    expect(backup.status).toBe('complete');
  });
});

describe('Mod type shape', () => {
  it('required fields exist on a minimal mod object', () => {
    const mod: Mod = {
      id: 'mod-1',
      name: 'Epic Mod',
      version: '1.0.0',
      source: 'local',
      source_url: '',
      checksum: 'sha256:def',
      installed_at: '2024-01-01T00:00:00Z',
      enabled: true,
    };
    expect(mod.source).toBe('local');
    expect(mod.enabled).toBe(true);
  });
});

describe('CVEReport type shape', () => {
  it('has top-level severity count fields', () => {
    const report: CVEReport = {
      generated_at: '2024-01-01T00:00:00Z',
      scanner: 'trivy',
      status: 'clean',
      total_count: 0,
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      findings: [],
      evidence: {
        last_checked: '2024-01-01T00:00:00Z',
        authoritative_link: 'https://osv.dev',
        cve_status: 'clean',
      },
    };
    // These must be top-level — not nested in a 'summary' object
    expect(typeof report.critical).toBe('number');
    expect(typeof report.high).toBe('number');
    expect(typeof report.medium).toBe('number');
    expect(typeof report.low).toBe('number');
  });
});

describe('User type shape', () => {
  it('required fields exist', () => {
    const user: User = {
      id: 'usr-1',
      username: 'admin',
      roles: ['admin'],
      created_at: '2024-01-01T00:00:00Z',
      totp_enabled: false,
    };
    expect(user.roles).toContain('admin');
    expect(user.totp_enabled).toBe(false);
  });
});

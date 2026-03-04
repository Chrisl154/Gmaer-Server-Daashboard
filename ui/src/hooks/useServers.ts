import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'react-hot-toast';
import { api } from '../utils/api';
import type {
  Server,
  ServersResponse,
  CreateServerRequest,
  UpdateServerRequest,
  BackupsResponse,
  BackupJob,
  ModsResponse,
  ModJob,
  InstallModRequest,
  ModTestSuiteResult,
  PortMapping,
  PortValidateResponse,
  CVEReport,
  SystemStatus,
} from '../types';

// ── Servers ────────────────────────────────────────────────────────────────────

export function useServers(refetchInterval = 15_000) {
  return useQuery<ServersResponse>({
    queryKey: ['servers'],
    queryFn: () => api.get('/api/v1/servers').then(r => r.data),
    refetchInterval,
  });
}

export function useServer(id: string, refetchInterval = 10_000) {
  return useQuery<Server>({
    queryKey: ['server', id],
    queryFn: () => api.get(`/api/v1/servers/${id}`).then(r => r.data),
    refetchInterval,
    enabled: !!id,
  });
}

export function useCreateServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateServerRequest) =>
      api.post('/api/v1/servers', req).then(r => r.data as Server),
    onSuccess: () => {
      toast.success('Server created');
      qc.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Create failed'),
  });
}

export function useUpdateServer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: UpdateServerRequest) =>
      api.put(`/api/v1/servers/${id}`, req).then(r => r.data as Server),
    onSuccess: () => {
      toast.success('Server updated');
      qc.invalidateQueries({ queryKey: ['server', id] });
      qc.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Update failed'),
  });
}

export function useDeleteServer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/servers/${id}`),
    onSuccess: () => {
      toast.success('Server deleted');
      qc.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Delete failed'),
  });
}

export function useStartServer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/start`),
    onSuccess: () => {
      toast.success('Server starting...');
      qc.invalidateQueries({ queryKey: ['server', id] });
      qc.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Start failed'),
  });
}

export function useStopServer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/stop`),
    onSuccess: () => {
      toast.success('Server stopping...');
      qc.invalidateQueries({ queryKey: ['server', id] });
      qc.invalidateQueries({ queryKey: ['servers'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Stop failed'),
  });
}

export function useRestartServer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/servers/${id}/restart`),
    onSuccess: () => {
      toast.success('Server restarting...');
      qc.invalidateQueries({ queryKey: ['server', id] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Restart failed'),
  });
}

export function useDeployServer(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: { method: string; steamcmd?: { app_id: string } }) =>
      api.post(`/api/v1/servers/${id}/deploy`, req).then(r => r.data),
    onSuccess: () => {
      toast.success('Deployment started');
      qc.invalidateQueries({ queryKey: ['server', id] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Deploy failed'),
  });
}

// ── Backups ────────────────────────────────────────────────────────────────────

export function useBackups(serverId: string) {
  return useQuery<BackupsResponse>({
    queryKey: ['backups', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/backups`).then(r => r.data),
    enabled: !!serverId,
  });
}

export function useTriggerBackup(serverId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (type: 'full' | 'incremental' = 'full') =>
      api.post(`/api/v1/servers/${serverId}/backup`, { type }).then(r => r.data as BackupJob),
    onSuccess: () => {
      toast.success('Backup started');
      qc.invalidateQueries({ queryKey: ['backups', serverId] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Backup failed'),
  });
}

export function useRestoreBackup(serverId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (backupId: string) =>
      api.post(`/api/v1/servers/${serverId}/restore/${backupId}`).then(r => r.data),
    onSuccess: () => {
      toast.success('Restore started');
      qc.invalidateQueries({ queryKey: ['server', serverId] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Restore failed'),
  });
}

// ── Mods ───────────────────────────────────────────────────────────────────────

export function useMods(serverId: string) {
  return useQuery<ModsResponse>({
    queryKey: ['mods', serverId],
    queryFn: () => api.get(`/api/v1/servers/${serverId}/mods`).then(r => r.data),
    enabled: !!serverId,
  });
}

export function useInstallMod(serverId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: InstallModRequest) =>
      api.post(`/api/v1/servers/${serverId}/mods`, req).then(r => r.data as ModJob),
    onSuccess: () => {
      toast.success('Mod installation started');
      qc.invalidateQueries({ queryKey: ['mods', serverId] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Install failed'),
  });
}

export function useUninstallMod(serverId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (modId: string) => api.delete(`/api/v1/servers/${serverId}/mods/${modId}`),
    onSuccess: () => {
      toast.success('Mod removed');
      qc.invalidateQueries({ queryKey: ['mods', serverId] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Uninstall failed'),
  });
}

export function useRunModTests(serverId: string) {
  return useMutation({
    mutationFn: () =>
      api.post(`/api/v1/servers/${serverId}/mods/test`).then(r => r.data as ModTestSuiteResult),
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Test run failed'),
  });
}

export function useRollbackMods(serverId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (checkpoint?: string) =>
      api.post(`/api/v1/servers/${serverId}/mods/rollback`, { checkpoint }),
    onSuccess: () => {
      toast.success('Mod rollback complete');
      qc.invalidateQueries({ queryKey: ['mods', serverId] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Rollback failed'),
  });
}

// ── Ports ──────────────────────────────────────────────────────────────────────

export function useValidatePorts() {
  return useMutation({
    mutationFn: (ports: PortMapping[]) =>
      api.post('/api/v1/ports/validate', { ports }).then(r => r.data as PortValidateResponse),
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Validation failed'),
  });
}

// ── SBOM / CVE ─────────────────────────────────────────────────────────────────

export function useCVEReport() {
  return useQuery<CVEReport>({
    queryKey: ['cve-report'],
    queryFn: () => api.get('/api/v1/cve-report').then(r => r.data),
    staleTime: 5 * 60_000, // 5 minutes
  });
}

export function useTriggerCVEScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post('/api/v1/sbom/scan').then(r => r.data),
    onSuccess: () => {
      toast.success('CVE scan started');
      qc.invalidateQueries({ queryKey: ['cve-report'] });
    },
    onError: (err: any) => toast.error(err.response?.data?.error ?? 'Scan failed'),
  });
}

// ── System status ──────────────────────────────────────────────────────────────

export function useSystemStatus(refetchInterval = 30_000) {
  return useQuery<SystemStatus>({
    queryKey: ['system-status'],
    queryFn: () => api.get('/api/v1/status').then(r => r.data),
    refetchInterval,
  });
}

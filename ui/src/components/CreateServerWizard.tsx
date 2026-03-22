import React, { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  X, ChevronRight, ChevronLeft, Check,
  Server, Gamepad2, Clock, HardDrive, AlertTriangle,
} from 'lucide-react';
import { toast } from 'react-hot-toast';
import { clsx } from 'clsx';
import { api } from '../utils/api';

// ── Game catalogue ─────────────────────────────────────────────────────────────

interface ConfigField {
  key: string;
  label: string;
  type: 'text' | 'number' | 'select' | 'toggle' | 'password';
  default: string | number | boolean;
  options?: { value: string; label: string }[];
  placeholder?: string;
  hint?: string;
}

interface GameDef {
  id: string;
  name: string;
  studio: string;
  description: string;
  bgColor: string;
  accentText: string;
  accentBorder: string;
  deployMethods: string[];
  rcon: boolean;
  mods: boolean;
  resources: { cpu: number; ram: number; disk: number };
  worldNameField?: { key: string; label: string; placeholder: string };
  configFields: ConfigField[];
}

const GAMES: GameDef[] = [
  {
    id: 'minecraft',
    name: 'Minecraft',
    studio: 'Mojang / Microsoft',
    description: 'Java Edition dedicated server. Full RCON support, plugins, and mods via Paper/Fabric/Forge.',
    bgColor: 'bg-green-950/40',
    accentText: 'text-green-400',
    accentBorder: 'border-green-500',
    deployMethods: ['manual', 'docker'],
    rcon: true,
    mods: true,
    resources: { cpu: 2, ram: 4, disk: 10 },
    worldNameField: { key: 'level_name', label: 'World Name', placeholder: 'world' },
    configFields: [
      { key: 'max_players',   label: 'Max Players',           type: 'number',   default: 20,       placeholder: '20' },
      { key: 'difficulty',    label: 'Difficulty',            type: 'select',   default: 'normal', options: [{ value: 'peaceful', label: 'Peaceful' }, { value: 'easy', label: 'Easy' }, { value: 'normal', label: 'Normal' }, { value: 'hard', label: 'Hard' }] },
      { key: 'gamemode',      label: 'Game Mode',             type: 'select',   default: 'survival', options: [{ value: 'survival', label: 'Survival' }, { value: 'creative', label: 'Creative' }, { value: 'adventure', label: 'Adventure' }, { value: 'spectator', label: 'Spectator' }] },
      { key: 'level_seed',    label: 'World Seed',            type: 'text',     default: '',       placeholder: 'Leave empty for random', hint: 'Determines world generation layout' },
      { key: 'online_mode',   label: 'Online Mode (Auth)',    type: 'toggle',   default: true,     hint: 'Requires players to have a valid Mojang account' },
      { key: 'motd',          label: 'Server MOTD',           type: 'text',     default: 'A Minecraft Server', placeholder: 'A Minecraft Server' },
      { key: 'rcon_password', label: 'RCON Password',         type: 'password', default: '',       placeholder: 'Required for console access' },
    ],
  },
  {
    id: 'valheim',
    name: 'Valheim',
    studio: 'Iron Gate AB',
    description: 'Viking survival and exploration. Worlds are per-save. Deploy via SteamCMD.',
    bgColor: 'bg-blue-950/40',
    accentText: 'text-blue-400',
    accentBorder: 'border-blue-500',
    deployMethods: ['steamcmd', 'manual'],
    rcon: false,
    mods: true,
    resources: { cpu: 2, ram: 4, disk: 5 },
    worldNameField: { key: 'world_name', label: 'World Name', placeholder: 'Dedicated' },
    configFields: [
      { key: 'server_password', label: 'Server Password', type: 'password', default: '', placeholder: 'Min 5 chars to appear in server list', hint: 'Leave empty for no password (unlisted)' },
      { key: 'server_public',   label: 'Show in Server List', type: 'toggle', default: true },
    ],
  },
  {
    id: 'satisfactory',
    name: 'Satisfactory',
    studio: 'Coffee Stain Studios',
    description: 'Cooperative factory building. Sessions auto-save. Deploy via SteamCMD.',
    bgColor: 'bg-orange-950/40',
    accentText: 'text-orange-400',
    accentBorder: 'border-orange-500',
    deployMethods: ['steamcmd', 'manual'],
    rcon: false,
    mods: true,
    resources: { cpu: 4, ram: 12, disk: 20 },
    configFields: [
      { key: 'max_players',       label: 'Max Players',               type: 'number', default: 4,   placeholder: '4' },
      { key: 'autosave_interval', label: 'Autosave Interval (seconds)', type: 'number', default: 300, placeholder: '300' },
    ],
  },
  {
    id: 'palworld',
    name: 'Palworld',
    studio: 'Pocketpair',
    description: 'Creature collection survival game. RCON supported. Deploy via SteamCMD.',
    bgColor: 'bg-yellow-950/40',
    accentText: 'text-yellow-400',
    accentBorder: 'border-yellow-500',
    deployMethods: ['steamcmd', 'manual'],
    rcon: true,
    mods: false,
    resources: { cpu: 4, ram: 8, disk: 10 },
    configFields: [
      { key: 'max_players',    label: 'Max Players',    type: 'number',   default: 32,     placeholder: '32' },
      { key: 'difficulty',     label: 'Difficulty',     type: 'select',   default: 'None', options: [{ value: 'None', label: 'Normal' }, { value: 'Difficult', label: 'Hard' }] },
      { key: 'pvp',            label: 'PvP Enabled',    type: 'toggle',   default: false },
      { key: 'admin_password', label: 'Admin Password', type: 'password', default: '',     placeholder: 'Required for RCON' },
      { key: 'server_password',label: 'Server Password',type: 'password', default: '',     placeholder: 'Leave empty for public' },
    ],
  },
  {
    id: 'eco',
    name: 'Eco',
    studio: 'Strange Loop Games',
    description: 'Ecology simulation and civilization building. WebSocket RCON, extensive config.',
    bgColor: 'bg-emerald-950/40',
    accentText: 'text-emerald-400',
    accentBorder: 'border-emerald-500',
    deployMethods: ['steamcmd', 'manual'],
    rcon: true,
    mods: true,
    resources: { cpu: 4, ram: 8, disk: 20 },
    configFields: [
      { key: 'max_players',       label: 'Max Players',       type: 'number',  default: 100,               placeholder: '100' },
      { key: 'server_description',label: 'Server Description',type: 'text',    default: 'A sustainable world', placeholder: 'A sustainable world' },
      { key: 'admin_password',    label: 'Admin Password',    type: 'password',default: '',                placeholder: 'Required' },
      { key: 'public_server',     label: 'Public Server',     type: 'toggle',  default: true },
    ],
  },
  {
    id: 'enshrouded',
    name: 'Enshrouded',
    studio: 'Keen Games',
    description: 'Survival action RPG. Up to 16 players. Saves automatically. Deploy via SteamCMD.',
    bgColor: 'bg-purple-950/40',
    accentText: 'text-purple-400',
    accentBorder: 'border-purple-500',
    deployMethods: ['steamcmd', 'manual'],
    rcon: false,
    mods: false,
    resources: { cpu: 4, ram: 8, disk: 10 },
    configFields: [
      { key: 'max_players',    label: 'Max Players',    type: 'number',   default: 16, placeholder: '16' },
      { key: 'server_password',label: 'Server Password',type: 'password', default: '', placeholder: 'Leave empty for no password' },
    ],
  },
  {
    id: 'riftbreaker',
    name: 'The Riftbreaker',
    studio: 'EXOR Studios',
    description: 'Sci-fi action RPG and base builder. Manual install. Steam Workshop mods.',
    bgColor: 'bg-red-950/40',
    accentText: 'text-red-400',
    accentBorder: 'border-red-500',
    deployMethods: ['manual', 'custom'],
    rcon: false,
    mods: true,
    resources: { cpu: 2, ram: 4, disk: 5 },
    configFields: [
      { key: 'max_players',    label: 'Max Players',    type: 'number',   default: 4, placeholder: '4' },
      { key: 'server_password',label: 'Server Password',type: 'password', default: '', placeholder: 'Leave empty for no password' },
    ],
  },
];

const BACKUP_SCHEDULES = [
  { value: '0 * * * *',   label: 'Every hour' },
  { value: '0 */6 * * *', label: 'Every 6 hours' },
  { value: '0 3 * * *',   label: 'Daily at 3 am' },
  { value: '0 3 * * 0',   label: 'Weekly (Sunday 3 am)' },
];

const RETENTION_OPTIONS = [7, 14, 30, 60, 90];

const DEPLOY_LABELS: Record<string, string> = {
  steamcmd: 'SteamCMD — auto-download from Steam',
  manual:   'Manual — provide archive / binary URL',
  docker:   'Docker — containerised',
  custom:   'Custom',
};

// ── Wizard state ───────────────────────────────────────────────────────────────

interface WizardState {
  game: GameDef | null;
  serverId: string;
  serverName: string;
  worldName: string;
  installDir: string;
  deployMethod: string;
  config: Record<string, string | number | boolean>;
  backupEnabled: boolean;
  backupSchedule: string;
  backupRetentionDays: number;
}

const STEPS = ['Pick Game', 'Name & Deploy', 'Game Config', 'Backups', 'Review'];

// ── Root component ─────────────────────────────────────────────────────────────

export function CreateServerWizard({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [step, setStep] = useState(0);
  const [state, setState] = useState<WizardState>({
    game: null,
    serverId: '',
    serverName: '',
    worldName: '',
    installDir: '/opt/games',
    deployMethod: '',
    config: {},
    backupEnabled: true,
    backupSchedule: '0 3 * * *',
    backupRetentionDays: 30,
  });

  const qc = useQueryClient();

  // Fetch host resources once for the pre-flight check in step 2.
  const { data: sysRes } = useQuery({
    queryKey: ['system-resources'],
    queryFn: () => api.get('/api/v1/system/resources').then(r => r.data),
    staleTime: 60_000,
  });

  const createMutation = useMutation({
    mutationFn: async () => {
      const g = state.game!;
      const config: Record<string, unknown> = { ...state.config };
      if (g.worldNameField && state.worldName) {
        config[g.worldNameField.key] = state.worldName;
      }
      await api.post('/api/v1/servers', {
        id: state.serverId,
        name: state.serverName,
        adapter: g.id,
        deploy_method: state.deployMethod,
        install_dir: state.installDir,
        config,
        ...(state.backupEnabled
          ? {
              backup_config: {
                enabled: true,
                schedule: state.backupSchedule,
                retain_days: state.backupRetentionDays,
              },
            }
          : {}),
      });
    },
    onSuccess: () => {
      toast.success(`${state.game?.name} server created!`);
      qc.invalidateQueries({ queryKey: ['servers'] });
      onCreated();
    },
    onError: (e: any) =>
      toast.error(e.response?.data?.error ?? 'Failed to create server'),
  });

  const update = (patch: Partial<WizardState>) =>
    setState(prev => ({ ...prev, ...patch }));

  const canAdvance = () => {
    if (step === 0) return !!state.game;
    if (step === 1) return !!state.serverId && !!state.serverName && !!state.deployMethod;
    return true;
  };

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
      <div className="bg-[#0f0f0f] border border-[#222] rounded-2xl w-full max-w-2xl max-h-[90vh] flex flex-col shadow-2xl">

        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[#1e1e1e]">
          <div>
            <h2 className="text-lg font-semibold text-gray-100">New Game Server</h2>
            <p className="text-xs text-gray-500 mt-0.5">{STEPS[step]}</p>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-gray-500 hover:text-gray-300 rounded-lg hover:bg-[#1e1e1e] transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Step bar */}
        <div className="flex items-center gap-1 px-6 pt-4 pb-2">
          {STEPS.map((label, i) => (
            <React.Fragment key={i}>
              <div className={clsx('flex items-center gap-1.5', i <= step ? 'opacity-100' : 'opacity-30')}>
                <div className={clsx(
                  'w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold transition-colors',
                  i < step  ? 'bg-blue-600 text-white' :
                  i === step ? 'bg-blue-600 text-white ring-2 ring-blue-400/40' :
                  'bg-[#222] text-gray-500',
                )}>
                  {i < step ? <Check className="w-3 h-3" /> : i + 1}
                </div>
                <span className={clsx('text-xs hidden sm:block', i === step ? 'text-gray-200' : 'text-gray-500')}>
                  {label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={clsx('flex-1 h-px mx-1 transition-colors', i < step ? 'bg-blue-600' : 'bg-[#222]')} />
              )}
            </React.Fragment>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto px-6 py-4 min-h-0">
          {step === 0 && <StepPickGame    state={state} update={update} />}
          {step === 1 && <StepNameDeploy  state={state} update={update} sysRes={sysRes} />}
          {step === 2 && <StepGameConfig  state={state} update={update} />}
          {step === 3 && <StepBackups     state={state} update={update} />}
          {step === 4 && <StepReview      state={state} />}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-[#1e1e1e]">
          <button
            onClick={() => setStep(p => p - 1)}
            disabled={step === 0}
            className="flex items-center gap-1.5 px-4 py-2 text-sm text-gray-400 hover:text-gray-200 disabled:opacity-0 transition-all"
          >
            <ChevronLeft className="w-4 h-4" /> Back
          </button>

          {step < STEPS.length - 1 ? (
            <button
              onClick={() => setStep(p => p + 1)}
              disabled={!canAdvance()}
              className="flex items-center gap-1.5 px-5 py-2 text-sm font-medium bg-blue-600 hover:bg-blue-700 text-white rounded-lg disabled:opacity-40 transition-colors"
            >
              Continue <ChevronRight className="w-4 h-4" />
            </button>
          ) : (
            <button
              onClick={() => createMutation.mutate()}
              disabled={createMutation.isPending}
              className="flex items-center gap-1.5 px-5 py-2 text-sm font-medium bg-green-600 hover:bg-green-700 text-white rounded-lg disabled:opacity-40 transition-colors"
            >
              {createMutation.isPending ? 'Creating…' : 'Create Server'}
              {!createMutation.isPending && <Check className="w-4 h-4" />}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ── Step 1: Pick Game ──────────────────────────────────────────────────────────

function StepPickGame({
  state,
  update,
}: {
  state: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {GAMES.map(g => {
        const selected = state.game?.id === g.id;
        return (
          <button
            key={g.id}
            onClick={() =>
              update({
                game: g,
                deployMethod: g.deployMethods[0],
                config: Object.fromEntries(g.configFields.map(f => [f.key, f.default])),
                worldName: '',
              })
            }
            className={clsx(
              'text-left p-4 rounded-xl border-2 transition-all',
              g.bgColor,
              selected
                ? `${g.accentBorder}`
                : 'border-[#1e1e1e] hover:border-[#333]',
            )}
          >
            <div className="flex items-start justify-between gap-2 mb-1.5">
              <span className={clsx('text-sm font-semibold', selected ? g.accentText : 'text-gray-200')}>
                {g.name}
              </span>
              <div className="flex gap-1 flex-shrink-0">
                {g.rcon && (
                  <span className="text-[10px] bg-blue-900/60 text-blue-300 px-1.5 py-0.5 rounded font-medium">
                    RCON
                  </span>
                )}
                {g.mods && (
                  <span className="text-[10px] bg-purple-900/60 text-purple-300 px-1.5 py-0.5 rounded font-medium">
                    Mods
                  </span>
                )}
              </div>
            </div>
            <p className="text-[11px] text-gray-500 mb-3 leading-relaxed">{g.description}</p>
            <div className="flex items-center gap-3 text-[10px] text-gray-600">
              <span>{g.resources.cpu} CPU</span>
              <span>{g.resources.ram} GB RAM</span>
              <span>{g.resources.disk} GB Disk</span>
            </div>
          </button>
        );
      })}
    </div>
  );
}

// ── Step 2: Name & Deploy ──────────────────────────────────────────────────────

function ResourceWarning({ game, sysRes }: { game: GameDef; sysRes: any }) {
  if (!sysRes) return null;

  const warnings: string[] = [];

  if (sysRes.cpu_cores < game.resources.cpu) {
    warnings.push(
      `CPU: ${game.resources.cpu} cores recommended, this machine has ${sysRes.cpu_cores}.`
    );
  }
  if (sysRes.free_ram_gb < game.resources.ram) {
    warnings.push(
      `RAM: ${game.resources.ram} GB recommended, only ${sysRes.free_ram_gb.toFixed(1)} GB free right now.`
    );
  }
  if (sysRes.free_disk_gb < game.resources.disk) {
    warnings.push(
      `Disk: ${game.resources.disk} GB recommended, only ${sysRes.free_disk_gb.toFixed(1)} GB free.`
    );
  }

  if (warnings.length === 0) return null;

  return (
    <div
      className="flex gap-3 rounded-xl px-4 py-3"
      style={{
        background: 'rgba(234,179,8,0.08)',
        border: '1px solid rgba(234,179,8,0.25)',
      }}
    >
      <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0 mt-0.5" />
      <div className="space-y-1">
        <p className="text-xs font-semibold text-yellow-300">
          This server may struggle on the current hardware
        </p>
        {warnings.map((w, i) => (
          <p key={i} className="text-xs" style={{ color: '#fbbf24' }}>{w}</p>
        ))}
        <p className="text-[11px] mt-1" style={{ color: '#a16207' }}>
          You can still deploy — this is a warning, not a blocker.
        </p>
      </div>
    </div>
  );
}

function StepNameDeploy({
  state,
  update,
  sysRes,
}: {
  state: WizardState;
  update: (p: Partial<WizardState>) => void;
  sysRes?: any;
}) {
  const g = state.game!;

  const handleNameChange = (name: string) => {
    const id = name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '');
    update({ serverName: name, serverId: id });
  };

  return (
    <div className="space-y-4">
      <ResourceWarning game={g} sysRes={sysRes} />
      <Field label="Server Name" hint="Display name shown in the dashboard">
        <input
          type="text"
          value={state.serverName}
          onChange={e => handleNameChange(e.target.value)}
          placeholder={`My ${g.name} Server`}
          className={INPUT}
          autoFocus
        />
      </Field>

      <Field label="Server ID" hint="Unique slug used in the API — auto-generated from name">
        <input
          type="text"
          value={state.serverId}
          onChange={e => update({ serverId: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '') })}
          placeholder="my-minecraft-server"
          className={INPUT}
        />
      </Field>

      {g.worldNameField && (
        <Field label={g.worldNameField.label} hint="The name of the world / save file on disk">
          <input
            type="text"
            value={state.worldName}
            onChange={e => update({ worldName: e.target.value })}
            placeholder={g.worldNameField.placeholder}
            className={INPUT}
          />
        </Field>
      )}

      <Field label="Install Directory" hint="Where server files will be stored on the host">
        <input
          type="text"
          value={state.installDir}
          onChange={e => update({ installDir: e.target.value })}
          placeholder="/opt/games"
          className={INPUT}
        />
      </Field>

      <Field label="Deploy Method">
        <div className="space-y-2">
          {g.deployMethods.map(m => (
            <label
              key={m}
              className={clsx(
                'flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors',
                state.deployMethod === m
                  ? 'border-blue-500 bg-blue-950/20'
                  : 'border-[#252525] hover:border-[#333]',
              )}
            >
              <input
                type="radio"
                name="deployMethod"
                value={m}
                checked={state.deployMethod === m}
                onChange={() => update({ deployMethod: m })}
                className="accent-blue-500"
              />
              <span className="text-sm text-gray-200">{DEPLOY_LABELS[m] ?? m}</span>
            </label>
          ))}
        </div>
      </Field>
    </div>
  );
}

// ── Step 3: Game Config ────────────────────────────────────────────────────────

function StepGameConfig({
  state,
  update,
}: {
  state: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const g = state.game!;

  const setConfig = (key: string, val: string | number | boolean) =>
    update({ config: { ...state.config, [key]: val } });

  if (g.configFields.length === 0) {
    return (
      <div className="py-16 text-center text-gray-500 text-sm">
        No additional configuration required for {g.name}.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {g.configFields.map(f => {
        const val = state.config[f.key] ?? f.default;
        return (
          <Field key={f.key} label={f.label} hint={f.hint}>
            {f.type === 'select' && (
              <select
                value={String(val)}
                onChange={e => setConfig(f.key, e.target.value)}
                className={INPUT}
              >
                {f.options!.map(o => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            )}

            {f.type === 'toggle' && (
              <label className="flex items-center gap-3 cursor-pointer select-none">
                <div
                  className={clsx(
                    'relative w-10 h-5 rounded-full transition-colors',
                    val ? 'bg-blue-600' : 'bg-[#333]',
                  )}
                >
                  <input
                    type="checkbox"
                    className="sr-only"
                    checked={!!val}
                    onChange={e => setConfig(f.key, e.target.checked)}
                  />
                  <div
                    className={clsx(
                      'absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform',
                      val ? 'translate-x-5' : 'translate-x-0.5',
                    )}
                  />
                </div>
                <span className="text-sm text-gray-300">{val ? 'Enabled' : 'Disabled'}</span>
              </label>
            )}

            {(f.type === 'text' || f.type === 'password' || f.type === 'number') && (
              <input
                type={f.type === 'password' ? 'password' : f.type === 'number' ? 'number' : 'text'}
                value={String(val)}
                onChange={e =>
                  setConfig(f.key, f.type === 'number' ? Number(e.target.value) : e.target.value)
                }
                placeholder={f.placeholder}
                className={INPUT}
              />
            )}
          </Field>
        );
      })}
    </div>
  );
}

// ── Step 4: Backups ────────────────────────────────────────────────────────────

function StepBackups({
  state,
  update,
}: {
  state: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  return (
    <div className="space-y-5">
      {/* Enable toggle */}
      <label className="flex items-center gap-3 cursor-pointer select-none p-4 rounded-xl border border-[#1e1e1e] hover:border-[#2a2a2a] transition-colors">
        <div className={clsx('relative w-10 h-5 rounded-full transition-colors', state.backupEnabled ? 'bg-blue-600' : 'bg-[#333]')}>
          <input
            type="checkbox"
            className="sr-only"
            checked={state.backupEnabled}
            onChange={e => update({ backupEnabled: e.target.checked })}
          />
          <div className={clsx('absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform', state.backupEnabled ? 'translate-x-5' : 'translate-x-0.5')} />
        </div>
        <div>
          <span className="text-sm font-medium text-gray-200">Enable Automatic Backups</span>
          <p className="text-xs text-gray-500 mt-0.5">Automatically back up world saves on a schedule</p>
        </div>
      </label>

      {state.backupEnabled && (
        <>
          <Field label="Backup Schedule">
            <div className="space-y-2">
              {BACKUP_SCHEDULES.map(s => (
                <label
                  key={s.value}
                  className={clsx(
                    'flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors',
                    state.backupSchedule === s.value
                      ? 'border-blue-500 bg-blue-950/20'
                      : 'border-[#252525] hover:border-[#333]',
                  )}
                >
                  <input
                    type="radio"
                    name="schedule"
                    value={s.value}
                    checked={state.backupSchedule === s.value}
                    onChange={() => update({ backupSchedule: s.value })}
                    className="accent-blue-500"
                  />
                  <span className="text-sm text-gray-200">{s.label}</span>
                  <span className="ml-auto text-[10px] text-gray-600 font-mono">{s.value}</span>
                </label>
              ))}
            </div>
          </Field>

          <Field label="Retention Period" hint="Backups older than this are automatically deleted">
            <div className="flex flex-wrap gap-2">
              {RETENTION_OPTIONS.map(d => (
                <button
                  key={d}
                  onClick={() => update({ backupRetentionDays: d })}
                  className={clsx(
                    'px-4 py-2 rounded-lg text-sm font-medium border transition-colors',
                    state.backupRetentionDays === d
                      ? 'bg-blue-600 border-blue-600 text-white'
                      : 'border-[#252525] text-gray-400 hover:border-[#444]',
                  )}
                >
                  {d} days
                </button>
              ))}
            </div>
          </Field>
        </>
      )}
    </div>
  );
}

// ── Step 5: Review ─────────────────────────────────────────────────────────────

function StepReview({ state }: { state: WizardState }) {
  const g = state.game!;

  return (
    <div className="space-y-3">
      <ReviewSection title="Game" icon={<Gamepad2 className="w-3.5 h-3.5" />}>
        <ReviewRow label="Game"         value={g.name} />
        <ReviewRow label="Studio"       value={g.studio} />
        <ReviewRow label="RCON Support" value={g.rcon ? 'Yes' : 'No'} />
        <ReviewRow label="Mod Support"  value={g.mods ? 'Yes' : 'No'} />
      </ReviewSection>

      <ReviewSection title="Server" icon={<Server className="w-3.5 h-3.5" />}>
        <ReviewRow label="Name"          value={state.serverName} />
        <ReviewRow label="ID"            value={state.serverId} mono />
        {g.worldNameField && state.worldName && (
          <ReviewRow label={g.worldNameField.label} value={state.worldName} />
        )}
        <ReviewRow label="Deploy Method" value={DEPLOY_LABELS[state.deployMethod] ?? state.deployMethod} />
        <ReviewRow label="Install Dir"   value={state.installDir} mono />
      </ReviewSection>

      {g.configFields.length > 0 && (
        <ReviewSection title="Game Config" icon={<HardDrive className="w-3.5 h-3.5" />}>
          {g.configFields.map(f => {
            const val = state.config[f.key] ?? f.default;
            const display =
              f.type === 'password'
                ? (val ? '••••••••' : '(not set)')
              : f.type === 'toggle'
                ? (val ? 'Enabled' : 'Disabled')
              : f.type === 'select'
                ? (f.options?.find(o => o.value === String(val))?.label ?? String(val))
              : String(val) || '(empty)';
            return <ReviewRow key={f.key} label={f.label} value={display} />;
          })}
        </ReviewSection>
      )}

      <ReviewSection title="Backups" icon={<Clock className="w-3.5 h-3.5" />}>
        <ReviewRow label="Auto Backup" value={state.backupEnabled ? 'Enabled' : 'Disabled'} />
        {state.backupEnabled && (
          <>
            <ReviewRow
              label="Schedule"
              value={BACKUP_SCHEDULES.find(s => s.value === state.backupSchedule)?.label ?? state.backupSchedule}
            />
            <ReviewRow label="Retention" value={`${state.backupRetentionDays} days`} />
          </>
        )}
      </ReviewSection>

      <div className="bg-blue-950/20 border border-blue-800/30 rounded-xl p-3 text-xs text-blue-300 leading-relaxed">
        Clicking <strong>Create Server</strong> will register the server and begin deployment.
        Monitor progress in the server detail page.
      </div>
    </div>
  );
}

// ── Shared primitives ──────────────────────────────────────────────────────────

const INPUT =
  'w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 ' +
  'focus:outline-none focus:border-blue-500 placeholder-gray-600 transition-colors';

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-xs font-medium text-gray-400 mb-1.5">{label}</label>
      {children}
      {hint && <p className="text-xs text-gray-600 mt-1">{hint}</p>}
    </div>
  );
}

function ReviewSection({
  title,
  icon,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-[#141414] border border-[#1e1e1e] rounded-xl p-4 space-y-2">
      <div className="flex items-center gap-2 text-[10px] font-semibold text-gray-500 uppercase tracking-widest mb-2.5">
        {icon}
        {title}
      </div>
      {children}
    </div>
  );
}

function ReviewRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between text-sm gap-4">
      <span className="text-gray-500 flex-shrink-0">{label}</span>
      <span className={clsx('text-gray-200 text-right', mono && 'font-mono text-xs')}>{value}</span>
    </div>
  );
}

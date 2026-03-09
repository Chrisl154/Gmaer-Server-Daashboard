import React, { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Activity, Check, ChevronRight, Eye, EyeOff, Loader2, Shield, Sliders } from 'lucide-react';
import toast from 'react-hot-toast';
import { useAuthStore } from '../store/authStore';
import { api } from '../utils/api';

type Step = 0 | 1 | 2;

const STEPS = [
  { label: 'Admin account', icon: Shield },
  { label: 'Two-factor auth', icon: Shield },
  { label: 'Basic settings', icon: Sliders },
];

// ── Step 1: Create admin account ────────────────────────────────────────────

function StepAccount({ onNext }: { onNext: () => void }) {
  const { login } = useAuthStore();
  const [form, setForm] = useState({ username: '', password: '', confirm: '' });
  const [showPw, setShowPw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const validationError = (() => {
    if (!form.username) return 'Username is required';
    if (form.password.length < 8) return 'Password must be at least 8 characters';
    if (form.password !== form.confirm) return 'Passwords do not match';
    return '';
  })();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (validationError) { setError(validationError); return; }
    setError('');
    setLoading(true);
    try {
      await api.post('/api/v1/system/bootstrap', {
        username: form.username,
        password: form.password,
      });
      // Log in immediately so subsequent wizard steps have an auth token.
      await login(form.username, form.password);
      onNext();
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Bootstrap failed');
    } finally {
      setLoading(false);
    }
  };

  const field = (key: keyof typeof form, label: string, type: string, placeholder: string) => (
    <div key={key}>
      <label className="block text-xs text-gray-400 mb-1">{label}</label>
      <div className="relative">
        <input
          type={type === 'password' ? (showPw ? 'text' : 'password') : type}
          value={form[key]}
          onChange={e => setForm(p => ({ ...p, [key]: e.target.value }))}
          placeholder={placeholder}
          autoComplete={key === 'username' ? 'username' : 'new-password'}
          className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500 pr-10"
        />
        {type === 'password' && (
          <button
            type="button"
            onClick={() => setShowPw(p => !p)}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300"
          >
            {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
          </button>
        )}
      </div>
    </div>
  );

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <h2 className="text-base font-semibold text-gray-100">Create your admin account</h2>
        <p className="text-xs text-gray-500 mt-1">This will be the first administrator of Games Dashboard.</p>
      </div>

      {field('username', 'Username', 'text', 'admin')}
      {field('password', 'Password', 'password', '••••••••')}
      {field('confirm', 'Confirm password', 'password', '••••••••')}

      {error && (
        <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">{error}</p>
      )}

      <button
        type="submit"
        disabled={loading || !!validationError}
        className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <ChevronRight className="w-4 h-4" />}
        {loading ? 'Creating account…' : 'Continue'}
      </button>
    </form>
  );
}

// ── Step 2: Optional TOTP setup ──────────────────────────────────────────────

function StepTOTP({ onNext }: { onNext: () => void }) {
  const { setupTOTP, verifyTOTP } = useAuthStore();
  const [totpData, setTotpData] = useState<{ secret: string; qr_code_url: string } | null>(null);
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    setLoading(true);
    setupTOTP()
      .then(data => setTotpData(data))
      .catch(() => setError('Failed to load TOTP setup. You can skip and configure it later.'))
      .finally(() => setLoading(false));
  }, [setupTOTP]);

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setVerifying(true);
    try {
      await verifyTOTP(code);
      toast.success('Two-factor authentication enabled');
      onNext();
    } catch {
      setError('Invalid code — check your authenticator app and try again.');
    } finally {
      setVerifying(false);
    }
  };

  const qrImgSrc = totpData
    ? `https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(totpData.qr_code_url)}`
    : null;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-base font-semibold text-gray-100">Set up two-factor authentication</h2>
        <p className="text-xs text-gray-500 mt-1">
          Scan the QR code with an authenticator app (Google Authenticator, Authy, etc.), then enter the code to verify.
        </p>
      </div>

      {loading && (
        <div className="flex justify-center py-6">
          <Loader2 className="w-6 h-6 animate-spin text-gray-500" />
        </div>
      )}

      {totpData && !loading && (
        <>
          <div className="flex flex-col items-center gap-3 py-2">
            {qrImgSrc && (
              <img
                src={qrImgSrc}
                alt="TOTP QR code"
                className="rounded-lg border border-[#252525] bg-white p-2"
                width={180}
                height={180}
              />
            )}
            <div className="w-full">
              <label className="block text-xs text-gray-400 mb-1">Manual entry key</label>
              <code className="block w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-xs text-gray-300 break-all select-all">
                {totpData.secret}
              </code>
            </div>
          </div>

          <form onSubmit={handleVerify} className="space-y-3">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Verification code</label>
              <input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                value={code}
                onChange={e => setCode(e.target.value.replace(/\D/g, ''))}
                placeholder="000000"
                className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500 tracking-widest text-center"
              />
            </div>

            {error && (
              <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">{error}</p>
            )}

            <button
              type="submit"
              disabled={verifying || code.length !== 6}
              className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {verifying ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}
              {verifying ? 'Verifying…' : 'Enable 2FA'}
            </button>
          </form>
        </>
      )}

      {error && !totpData && (
        <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">{error}</p>
      )}

      <button
        onClick={onNext}
        className="w-full px-4 py-2 text-sm text-gray-400 hover:text-gray-200 bg-[#1a1a1a] hover:bg-[#252525] rounded-lg transition-colors"
      >
        Skip for now
      </button>
    </div>
  );
}

// ── Step 3: Basic server settings ────────────────────────────────────────────

function StepSettings({ onFinish }: { onFinish: () => void }) {
  const [form, setForm] = useState({
    log_level: 'info',
    metrics_enabled: true,
    metrics_path: '/metrics',
    backup_retain_days: 30,
    backup_schedule: '0 3 * * *',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await api.patch('/api/v1/admin/settings', {
        log_level: form.log_level,
        metrics: { enabled: form.metrics_enabled, path: form.metrics_path },
        backup: { retain_days: form.backup_retain_days, default_schedule: form.backup_schedule },
      });
      toast.success('Settings saved');
      onFinish();
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to save settings');
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <div>
        <h2 className="text-base font-semibold text-gray-100">Basic settings</h2>
        <p className="text-xs text-gray-500 mt-1">Configure logging, metrics, and backups. Everything else can be changed later in Settings.</p>
      </div>

      {/* Log level */}
      <div>
        <label className="block text-xs text-gray-400 mb-1">Log level</label>
        <select
          value={form.log_level}
          onChange={e => setForm(p => ({ ...p, log_level: e.target.value }))}
          className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
        >
          {['debug', 'info', 'warn', 'error'].map(l => (
            <option key={l} value={l}>{l}</option>
          ))}
        </select>
      </div>

      {/* Metrics */}
      <div className="space-y-2">
        <label className="block text-xs text-gray-400">Prometheus metrics</label>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setForm(p => ({ ...p, metrics_enabled: !p.metrics_enabled }))}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${form.metrics_enabled ? 'bg-blue-600' : 'bg-[#252525]'}`}
          >
            <span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${form.metrics_enabled ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
          </button>
          <span className="text-xs text-gray-400">{form.metrics_enabled ? 'Enabled' : 'Disabled'}</span>
        </div>
        {form.metrics_enabled && (
          <input
            type="text"
            value={form.metrics_path}
            onChange={e => setForm(p => ({ ...p, metrics_path: e.target.value }))}
            placeholder="/metrics"
            className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
          />
        )}
      </div>

      {/* Backup */}
      <div className="space-y-2">
        <label className="block text-xs text-gray-400">Backup retention (days)</label>
        <input
          type="number"
          min={1}
          max={365}
          value={form.backup_retain_days}
          onChange={e => setForm(p => ({ ...p, backup_retain_days: Number(e.target.value) }))}
          className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500"
        />
        <label className="block text-xs text-gray-400">Backup schedule (cron)</label>
        <input
          type="text"
          value={form.backup_schedule}
          onChange={e => setForm(p => ({ ...p, backup_schedule: e.target.value }))}
          placeholder="0 3 * * *"
          className="w-full bg-[#0d0d0d] border border-[#252525] rounded-lg px-3 py-2 text-sm text-gray-100 focus:outline-none focus:border-blue-500 font-mono"
        />
        <p className="text-xs text-gray-600">Default: daily at 03:00</p>
      </div>

      {/* TLS / bind note */}
      <div className="bg-[#0d0d0d] border border-[#1a1a1a] rounded-lg px-3 py-3">
        <p className="text-xs text-gray-500">
          <span className="text-gray-400 font-medium">Bind address & TLS</span> — edit{' '}
          <code className="text-blue-400">daemon.yaml</code> and restart the daemon to change the listen
          address or TLS certificate paths.
        </p>
      </div>

      {error && (
        <p className="text-xs text-red-400 bg-red-900/20 border border-red-900/30 rounded-lg px-3 py-2">{error}</p>
      )}

      <div className="flex gap-3">
        <button
          type="button"
          onClick={onFinish}
          className="flex-1 px-4 py-2 text-sm text-gray-400 hover:text-gray-200 bg-[#1a1a1a] hover:bg-[#252525] rounded-lg transition-colors"
        >
          Skip
        </button>
        <button
          type="submit"
          disabled={loading}
          className="flex-1 flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg disabled:opacity-50 transition-colors"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}
          {loading ? 'Saving…' : 'Finish setup'}
        </button>
      </div>
    </form>
  );
}

// ── Wizard shell ─────────────────────────────────────────────────────────────

export function SetupWizardPage() {
  const { isAuthenticated } = useAuthStore();
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>(0);

  // If already fully logged in, send to dashboard.
  if (isAuthenticated && step === 0) return <Navigate to="/" replace />;

  const advance = () => setStep(s => (s + 1) as Step);

  const finish = () => {
    toast.success('Setup complete — sign in to get started.');
    navigate('/login', { replace: true });
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a] flex items-center justify-center p-4">
      <div className="w-full max-w-sm">

        {/* Header */}
        <div className="flex flex-col items-center mb-8">
          <div className="w-14 h-14 bg-blue-600/20 border border-blue-600/30 rounded-2xl flex items-center justify-center mb-4">
            <Activity className="w-7 h-7 text-blue-400" />
          </div>
          <h1 className="text-xl font-semibold text-gray-100">Games Dashboard</h1>
          <p className="text-sm text-gray-400 mt-1">First-time setup</p>
        </div>

        {/* Step indicator */}
        <div className="flex items-center gap-2 mb-8">
          {STEPS.map((s, i) => (
            <React.Fragment key={i}>
              <div className="flex items-center gap-1.5">
                <div className={`w-5 h-5 rounded-full flex items-center justify-center text-xs font-semibold transition-colors ${
                  i < step
                    ? 'bg-green-600 text-white'
                    : i === step
                    ? 'bg-blue-600 text-white'
                    : 'bg-[#252525] text-gray-500'
                }`}>
                  {i < step ? <Check className="w-3 h-3" /> : i + 1}
                </div>
                <span className={`text-xs hidden sm:block transition-colors ${i === step ? 'text-gray-300' : 'text-gray-600'}`}>
                  {s.label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={`flex-1 h-px transition-colors ${i < step ? 'bg-green-600' : 'bg-[#252525]'}`} />
              )}
            </React.Fragment>
          ))}
        </div>

        {/* Card */}
        <div className="bg-[#141414] border border-[#252525] rounded-xl p-6">
          {step === 0 && <StepAccount onNext={advance} />}
          {step === 1 && <StepTOTP onNext={advance} />}
          {step === 2 && <StepSettings onFinish={finish} />}
        </div>

        <p className="text-center text-xs text-gray-600 mt-6">
          Step {step + 1} of {STEPS.length}
        </p>
      </div>
    </div>
  );
}

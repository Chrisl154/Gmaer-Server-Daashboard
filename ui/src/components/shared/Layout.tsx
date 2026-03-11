import { useState } from 'react';
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, Server, Cpu, Database, Package,
  Network, Shield, FileText, Settings, ClipboardList, LogOut,
  ChevronLeft, ChevronRight, Activity, Gamepad2,
} from 'lucide-react';
import { useAuthStore } from '../../store/authStore';
import { cn } from '../../utils/cn';

const NAV_ITEMS = [
  { path: '/',         label: 'Dashboard', icon: LayoutDashboard },
  { path: '/servers',  label: 'Servers',   icon: Server },
  { path: '/nodes',    label: 'Nodes',     icon: Cpu },
  { path: '/backups',  label: 'Backups',   icon: Database },
  { path: '/mods',     label: 'Mods',      icon: Package },
  { path: '/ports',    label: 'Ports',     icon: Network },
  { path: '/security', label: 'Security',  icon: Shield },
  { path: '/logs',     label: 'Logs',      icon: ClipboardList },
  { path: '/sbom',     label: 'SBOM / CVE',icon: FileText },
  { path: '/settings', label: 'Settings',  icon: Settings },
];

export function Layout() {
  const [collapsed, setCollapsed] = useState(false);
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();

  const handleLogout = () => { logout(); navigate('/login'); };

  const isActive = (path: string) =>
    path === '/' ? pathname === '/' : pathname.startsWith(path);

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: 'var(--bg-page)' }}>

      {/* ── Sidebar ─────────────────────────────────────────────────────── */}
      <aside
        className={cn(
          'flex flex-col h-full shrink-0 transition-all duration-300 ease-in-out relative z-20',
          collapsed ? 'w-[64px]' : 'w-[240px]',
        )}
        style={{
          background: 'var(--bg-sidebar)',
          borderRight: '1px solid var(--border)',
          boxShadow: '4px 0 24px rgba(0,0,0,0.25)',
        }}
      >
        {/* Logo */}
        <div
          className={cn('flex items-center h-16 px-4 shrink-0', collapsed ? 'justify-center' : 'gap-3')}
          style={{ borderBottom: '1px solid var(--border)' }}
        >
          <div
            className="w-8 h-8 rounded-xl flex items-center justify-center shrink-0"
            style={{ background: 'linear-gradient(135deg, #f97316, #ea580c)', boxShadow: '0 0 18px rgba(249,115,22,0.45)' }}
          >
            <Gamepad2 size={16} className="text-white" />
          </div>
          {!collapsed && (
            <div className="overflow-hidden">
              <p className="text-sm leading-tight" style={{ color: 'var(--text-primary)', fontWeight: 700 }}>
                Games Dashboard
              </p>
              <p className="text-[10px]" style={{ color: 'var(--text-muted)' }}>v1.0.0</p>
            </div>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto py-3 px-2">
          {!collapsed && <p className="label px-2 mb-2">Navigation</p>}
          <ul className="flex flex-col gap-0.5">
            {NAV_ITEMS.map(({ path, label, icon: Icon }) => {
              const active = isActive(path);
              return (
                <li key={path}>
                  <Link
                    to={path}
                    title={collapsed ? label : undefined}
                    className={cn(
                      'relative flex items-center rounded-lg transition-all duration-150 group',
                      collapsed ? 'justify-center px-0 py-2.5' : 'gap-3 px-3 py-2.5',
                      active ? 'nav-active' : '',
                    )}
                    style={{
                      background: active ? 'rgba(249,115,22,0.1)' : 'transparent',
                      color: active ? '#fb923c' : 'var(--text-secondary)',
                      textDecoration: 'none',
                    }}
                    onMouseEnter={e => { if (!active) (e.currentTarget as HTMLElement).style.background = 'rgba(255,255,255,0.04)'; }}
                    onMouseLeave={e => { if (!active) (e.currentTarget as HTMLElement).style.background = 'transparent'; }}
                  >
                    <Icon size={17} style={{ color: active ? '#f97316' : undefined, flexShrink: 0 }} />
                    {!collapsed && (
                      <span className="text-sm font-medium truncate" style={{ color: active ? '#f0f0f8' : undefined }}>
                        {label}
                      </span>
                    )}
                    {collapsed && (
                      <span
                        className="absolute left-full ml-3 px-2.5 py-1 rounded-lg text-xs font-medium
                                   whitespace-nowrap pointer-events-none opacity-0 group-hover:opacity-100
                                   transition-opacity duration-150 z-50"
                        style={{ background: '#1e1e38', color: 'var(--text-primary)', border: '1px solid var(--border-strong)', boxShadow: '0 4px 14px rgba(0,0,0,0.5)' }}
                      >
                        {label}
                      </span>
                    )}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        {/* Bottom: user + collapse */}
        <div className="shrink-0 px-2 pb-3" style={{ borderTop: '1px solid var(--border)' }}>
          <button
            onClick={() => setCollapsed(c => !c)}
            className={cn('w-full flex items-center rounded-lg py-2 mt-2 transition-all duration-150 text-sm', collapsed ? 'justify-center' : 'gap-2 px-3')}
            style={{ color: 'var(--text-muted)', background: 'transparent', border: 'none', cursor: 'pointer' }}
            onMouseEnter={e => (e.currentTarget.style.background = 'rgba(255,255,255,0.04)')}
            onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
          >
            {collapsed ? <ChevronRight size={15} /> : <><ChevronLeft size={15} /><span>Collapse</span></>}
          </button>

          <div className={cn('flex items-center rounded-lg py-2 mt-1', collapsed ? 'justify-center' : 'gap-2.5 px-3')}>
            <div
              className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold"
              style={{ background: 'linear-gradient(135deg, #3b82f6, #1d4ed8)', color: '#fff' }}
            >
              {user?.username?.[0]?.toUpperCase() ?? 'A'}
            </div>
            {!collapsed && (
              <>
                <div className="flex-1 overflow-hidden">
                  <p className="text-xs font-semibold truncate" style={{ color: 'var(--text-primary)' }}>{user?.username ?? 'admin'}</p>
                  <p className="text-[10px] truncate" style={{ color: 'var(--text-muted)' }}>{user?.roles?.[0] ?? 'admin'}</p>
                </div>
                <button
                  onClick={handleLogout}
                  title="Log out"
                  className="p-1.5 rounded-md transition-all duration-150"
                  style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}
                  onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = 'rgba(239,68,68,0.12)'; (e.currentTarget as HTMLElement).style.color = '#f87171'; }}
                  onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = 'var(--text-muted)'; }}
                >
                  <LogOut size={14} />
                </button>
              </>
            )}
          </div>

          {collapsed && (
            <button
              onClick={handleLogout}
              title="Log out"
              className="w-full flex justify-center items-center py-2 rounded-lg mt-0.5 transition-all duration-150"
              style={{ background: 'transparent', border: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}
              onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = 'rgba(239,68,68,0.12)'; (e.currentTarget as HTMLElement).style.color = '#f87171'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = 'var(--text-muted)'; }}
            >
              <LogOut size={14} />
            </button>
          )}
        </div>
      </aside>

      {/* ── Main ───────────────────────────────────────────────────────── */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* Top bar */}
        <div
          className="flex items-center h-16 px-6 shrink-0"
          style={{ background: 'rgba(11,11,22,0.85)', borderBottom: '1px solid var(--border)', backdropFilter: 'blur(14px)' }}
        >
          <div className="flex items-center gap-2">
            <Activity size={13} style={{ color: '#f97316' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Games Dashboard</span>
          </div>
          <div className="ml-auto flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse inline-block" />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>Live</span>
          </div>
        </div>

        {/* Scrollable page content */}
        <div className="flex-1 overflow-y-auto">
          <div className="animate-page">
            <Outlet />
          </div>
        </div>
      </main>
    </div>
  );
}

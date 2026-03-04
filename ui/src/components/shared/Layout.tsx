import React, { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  Server,
  HardDrive,
  Package,
  Network,
  Shield,
  FileSearch,
  Settings,
  LogOut,
  ChevronLeft,
  ChevronRight,
  Activity,
  Menu,
  Layers,
} from 'lucide-react';
import { clsx } from 'clsx';
import { useAuthStore } from '../../store/authStore';

const NAV_ITEMS = [
  { path: '/',        icon: LayoutDashboard, label: 'Dashboard'  },
  { path: '/servers', icon: Server,          label: 'Servers'    },
  { path: '/nodes',   icon: Layers,          label: 'Nodes'      },
  { path: '/backups', icon: HardDrive,       label: 'Backups'    },
  { path: '/mods',    icon: Package,         label: 'Mods'       },
  { path: '/ports',   icon: Network,         label: 'Ports'      },
  { path: '/security',icon: Shield,          label: 'Security'   },
  { path: '/sbom',    icon: FileSearch,      label: 'SBOM & CVE' },
  { path: '/settings',icon: Settings,        label: 'Settings'   },
];

export function Layout() {
  const [collapsed, setCollapsed] = useState(false);
  const { user, logout } = useAuthStore();

  return (
    <div className="flex h-screen bg-[#0a0a0a] text-gray-100 overflow-hidden">
      {/* Sidebar */}
      <aside
        className={clsx(
          'flex flex-col bg-[#101010] border-r border-[#1a1a1a] transition-all duration-200 shrink-0',
          collapsed ? 'w-14' : 'w-56'
        )}
      >
        {/* Logo */}
        <div className="flex items-center gap-3 px-3 py-4 border-b border-[#1a1a1a]">
          <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center shrink-0">
            <Activity className="w-4 h-4 text-white" />
          </div>
          {!collapsed && (
            <div className="min-w-0">
              <div className="text-sm font-semibold text-gray-100 truncate">Games Dashboard</div>
              <div className="text-xs text-gray-500">v1.0.0</div>
            </div>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 py-3 space-y-0.5 px-1.5 overflow-y-auto">
          {NAV_ITEMS.map(({ path, icon: Icon, label }) => (
            <NavLink
              key={path}
              to={path}
              end={path === '/'}
              className={({ isActive }) =>
                clsx(
                  'flex items-center gap-3 px-2.5 py-2 rounded-lg text-sm transition-colors',
                  isActive
                    ? 'bg-blue-600/15 text-blue-400'
                    : 'text-gray-400 hover:text-gray-100 hover:bg-[#1a1a1a]'
                )
              }
            >
              <Icon className="w-4 h-4 shrink-0" />
              {!collapsed && <span className="truncate">{label}</span>}
            </NavLink>
          ))}
        </nav>

        {/* User / Collapse */}
        <div className="border-t border-[#1a1a1a] p-1.5 space-y-0.5">
          {!collapsed && user && (
            <div className="px-2.5 py-2 rounded-lg flex items-center gap-2">
              <div className="w-6 h-6 rounded-full bg-blue-600/30 flex items-center justify-center text-xs text-blue-400 font-medium shrink-0">
                {user.username?.[0]?.toUpperCase() ?? 'U'}
              </div>
              <span className="text-xs text-gray-400 truncate">{user.username}</span>
            </div>
          )}
          <button
            onClick={logout}
            className="w-full flex items-center gap-3 px-2.5 py-2 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-red-900/10 transition-colors"
          >
            <LogOut className="w-4 h-4 shrink-0" />
            {!collapsed && <span>Sign out</span>}
          </button>
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="w-full flex items-center gap-3 px-2.5 py-2 rounded-lg text-sm text-gray-500 hover:text-gray-300 hover:bg-[#1a1a1a] transition-colors"
          >
            {collapsed ? (
              <ChevronRight className="w-4 h-4 shrink-0" />
            ) : (
              <>
                <ChevronLeft className="w-4 h-4 shrink-0" />
                <span>Collapse</span>
              </>
            )}
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}

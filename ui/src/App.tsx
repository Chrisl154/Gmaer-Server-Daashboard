import React, { useCallback, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import toast, { Toaster } from 'react-hot-toast';
import { useAuthStore } from './store/authStore';
import { useInactivityTimer } from './hooks/useInactivityTimer';
import { useConnectionStatus } from './hooks/useConnectionStatus';
import { api } from './utils/api';
import { Layout } from './components/shared/Layout';
import { LoginPage } from './pages/LoginPage';
import { SetupWizardPage } from './pages/SetupWizardPage';
import { DashboardPage } from './pages/DashboardPage';
import { ServersPage } from './pages/ServersPage';
import { ServerDetailPage } from './pages/ServerDetailPage';
import { BackupsPage } from './pages/BackupsPage';
import { ModsPage } from './pages/ModsPage';
import { PortsPage } from './pages/PortsPage';
import { SecurityPage } from './pages/SecurityPage';
import { SBOMPage } from './pages/SBOMPage';
import { SettingsPage } from './pages/SettingsPage';
import { NodesPage } from './pages/NodesPage';
import { LogsPage } from './pages/LogsPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      staleTime: 30_000,
      refetchOnWindowFocus: true,
    },
  },
});

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

// Wraps all authenticated pages. Logs the user out after 29m 50s of inactivity.
function InactivityGuard({ children }: { children: React.ReactNode }) {
  const logout = useAuthStore(s => s.logout);
  const navigate = useNavigate();

  const handleTimeout = useCallback(() => {
    logout();
    navigate('/login', { replace: true });
    toast('Logged out due to inactivity.');
  }, [logout, navigate]);

  useInactivityTimer(handleTimeout);
  return <>{children}</>;
}

function ConnectionBanner() {
  const connected = useConnectionStatus();
  if (connected) return null;
  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, right: 0, zIndex: 9999,
      background: 'linear-gradient(90deg, #b91c1c, #dc2626)',
      color: '#fff', textAlign: 'center',
      padding: '8px 16px', fontSize: '13px', fontWeight: 500,
      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
    }}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ animation: 'spin 1s linear infinite' }}>
        <path d="M21 12a9 9 0 1 1-6.219-8.56" />
      </svg>
      Reconnecting to daemon — the dashboard is restarting and will be back shortly.
      <style>{`@keyframes spin { to { transform: rotate(360deg) } }`}</style>
    </div>
  );
}

function InitCheck() {
  const navigate = useNavigate();
  const { checkAuth, isAuthenticated } = useAuthStore();

  useEffect(() => {
    checkAuth();
    // Redirect to setup wizard if no admin account exists yet.
    api.get('/api/v1/system/init-status')
      .then(r => {
        if (!r.data.initialized && !isAuthenticated) {
          navigate('/setup', { replace: true });
        }
      })
      .catch(() => {/* daemon unreachable — let normal auth flow handle it */});
  }, [checkAuth, isAuthenticated, navigate]);

  return null;
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ConnectionBanner />
      <BrowserRouter>
        <InitCheck />
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/setup" element={<SetupWizardPage />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <InactivityGuard>
                  <Layout />
                </InactivityGuard>
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardPage />} />
            <Route path="servers" element={<ServersPage />} />
            <Route path="servers/:id" element={<ServerDetailPage />} />
            <Route path="backups" element={<BackupsPage />} />
            <Route path="mods" element={<ModsPage />} />
            <Route path="ports" element={<PortsPage />} />
            <Route path="security" element={<SecurityPage />} />
            <Route path="logs" element={<LogsPage />} />
            <Route path="sbom" element={<SBOMPage />} />
            <Route path="nodes" element={<NodesPage />} />
            <Route path="settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster
        position="bottom-right"
        toastOptions={{
          style: {
            background: '#1a1a1a',
            color: '#e5e5e5',
            border: '1px solid #333',
            borderRadius: '8px',
          },
        }}
      />
    </QueryClientProvider>
  );
}

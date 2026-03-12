import React, { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import { useAuthStore } from './store/authStore';
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
      <BrowserRouter>
        <InitCheck />
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/setup" element={<SetupWizardPage />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout />
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

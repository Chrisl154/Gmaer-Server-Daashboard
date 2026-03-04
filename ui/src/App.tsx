import React, { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import { useAuthStore } from './store/authStore';
import { Layout } from './components/shared/Layout';
import { LoginPage } from './pages/LoginPage';
import { DashboardPage } from './pages/DashboardPage';
import { ServersPage } from './pages/ServersPage';
import { ServerDetailPage } from './pages/ServerDetailPage';
import { BackupsPage } from './pages/BackupsPage';
import { ModsPage } from './pages/ModsPage';
import { PortsPage } from './pages/PortsPage';
import { SecurityPage } from './pages/SecurityPage';
import { SBOMPage } from './pages/SBOMPage';
import { SettingsPage } from './pages/SettingsPage';

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

export function App() {
  const { checkAuth } = useAuthStore();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
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
            <Route path="sbom" element={<SBOMPage />} />
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

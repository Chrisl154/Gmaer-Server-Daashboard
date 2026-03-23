import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import React from 'react';

const mockLogout = vi.fn();
const mockNavigate = vi.fn();

vi.mock('../../store/authStore', () => ({
  useAuthStore: () => ({
    user: { username: 'admin', roles: ['admin'] },
    logout: mockLogout,
  }),
}));
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

import { Layout } from './Layout';

function wrap(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<div>home</div>} />
          <Route path="*" element={<div>page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>
  );
}

beforeEach(() => vi.clearAllMocks());

describe('Layout', () => {
  it('renders Games Dashboard logo', () => {
    wrap();
    expect(screen.getAllByText('Games Dashboard').length).toBeGreaterThanOrEqual(1);
  });

  it('renders all nav labels', () => {
    wrap();
    ['Dashboard', 'Servers', 'Nodes', 'Backups', 'Mods', 'Ports', 'Security', 'Logs', 'Settings'].forEach(label => {
      expect(screen.getByText(label)).toBeTruthy();
    });
  });

  it('renders SBOM / CVE nav item', () => {
    wrap();
    expect(screen.getByText('SBOM / CVE')).toBeTruthy();
  });

  it('shows logged-in username', () => {
    wrap();
    expect(screen.getAllByText('admin').length).toBeGreaterThanOrEqual(1);
  });

  it('renders Collapse button', () => {
    wrap();
    expect(screen.getByText('Collapse')).toBeTruthy();
  });

  it('collapses sidebar on Collapse button click', () => {
    wrap();
    const collapseBtn = screen.getByText('Collapse');
    fireEvent.click(collapseBtn.closest('button')!);
    // After collapse, "Collapse" text should be gone (icon-only mode)
    expect(screen.queryByText('Collapse')).toBeNull();
  });

  it('calls logout and navigates on logout button click', () => {
    wrap();
    const logoutBtn = screen.getByTitle('Log out');
    fireEvent.click(logoutBtn);
    expect(mockLogout).toHaveBeenCalledOnce();
    expect(mockNavigate).toHaveBeenCalledWith('/login');
  });

  it('renders outlet content', () => {
    wrap('/');
    expect(screen.getByText('home')).toBeTruthy();
  });
});

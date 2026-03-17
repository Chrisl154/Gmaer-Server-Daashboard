import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

vi.mock('../utils/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}));
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

import { SBOMPage } from './SBOMPage';
import { api } from '../utils/api';

const mockGet  = api.get  as ReturnType<typeof vi.fn>;
const mockPost = api.post as ReturnType<typeof vi.fn>;

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter><SBOMPage /></MemoryRouter>
    </QueryClientProvider>
  );
}

const sbomData = {
  bomFormat:    'CycloneDX',
  specVersion:  '1.4',
  serialNumber: 'urn:uuid:test-serial',
  components:   [
    { name: 'react',   version: '18.2.0', type: 'library' },
    { name: 'express', version: '4.18.0', type: 'library' },
  ],
};

const cveData = {
  critical: 0, high: 0, medium: 2, low: 5,
  generated_at: '2024-01-01T00:00:00Z',
  scanner: 'trivy',
};

beforeEach(() => vi.clearAllMocks());

describe('SBOMPage', () => {
  it('renders page heading', () => {
    mockGet.mockResolvedValue({ data: {} });
    wrap();
    expect(screen.getByText('SBOM & CVE')).toBeTruthy();
  });

  it('renders Scan Now button', () => {
    mockGet.mockResolvedValue({ data: {} });
    wrap();
    expect(screen.getByRole('button', { name: /scan now/i })).toBeTruthy();
  });

  it('shows SBOM Summary section', async () => {
    mockGet.mockResolvedValue({ data: sbomData });
    wrap();
    await waitFor(() => expect(screen.getByText('SBOM Summary')).toBeTruthy());
  });

  it('shows CVE Report section', async () => {
    mockGet.mockResolvedValue({ data: cveData });
    wrap();
    await waitFor(() => expect(screen.getByText('CVE Report')).toBeTruthy());
  });

  it('shows critical CVE count', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('cve-report')) return Promise.resolve({ data: { ...cveData, critical: 3 } });
      return Promise.resolve({ data: sbomData });
    });
    wrap();
    await waitFor(() => {
      const threes = screen.getAllByText('3');
      expect(threes.length).toBeGreaterThan(0);
    });
  });

  it('shows clean status banner when no critical/high CVEs', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('cve-report')) return Promise.resolve({ data: cveData });
      return Promise.resolve({ data: sbomData });
    });
    wrap();
    await waitFor(() =>
      expect(screen.getByText('No Critical or High CVEs detected')).toBeTruthy()
    );
  });

  it('shows vulnerability banner when critical CVEs exist', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('cve-report')) return Promise.resolve({ data: { ...cveData, critical: 1 } });
      return Promise.resolve({ data: sbomData });
    });
    wrap();
    await waitFor(() =>
      expect(screen.getByText('Vulnerabilities found — action required')).toBeTruthy()
    );
  });

  it('shows SBOM components in table', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('cve-report')) return Promise.resolve({ data: cveData });
      return Promise.resolve({ data: sbomData });
    });
    wrap();
    await waitFor(() => expect(screen.getByText('react')).toBeTruthy());
    expect(screen.getByText('express')).toBeTruthy();
  });

  it('shows empty components message when SBOM has no components', async () => {
    mockGet.mockImplementation((url: string) => {
      if (url.includes('cve-report')) return Promise.resolve({ data: cveData });
      return Promise.resolve({ data: { ...sbomData, components: [] } });
    });
    wrap();
    await waitFor(() =>
      expect(screen.getByText(/no components in sbom yet/i)).toBeTruthy()
    );
  });

  it('triggers scan mutation on Scan Now click', async () => {
    mockGet.mockResolvedValue({ data: {} });
    mockPost.mockResolvedValue({ data: {} });
    wrap();
    fireEvent.click(screen.getByRole('button', { name: /scan now/i }));
    await waitFor(() => expect(mockPost).toHaveBeenCalledWith('/api/v1/sbom/scan'));
  });
});

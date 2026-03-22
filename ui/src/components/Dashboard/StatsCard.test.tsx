import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import React from 'react';
import { Server } from 'lucide-react';
import { StatsCard } from './StatsCard';

describe('StatsCard', () => {
  it('renders title and numeric value', () => {
    render(<StatsCard title="Total Servers" value={5} icon={Server} color="blue" />);
    expect(screen.getByText('Total Servers')).toBeTruthy();
    expect(screen.getByText('5')).toBeTruthy();
  });

  it('renders string value', () => {
    render(<StatsCard title="System Health" value="Healthy" icon={Server} color="orange" />);
    expect(screen.getByText('Healthy')).toBeTruthy();
  });

  it('renders trend badge when provided', () => {
    render(<StatsCard title="Running" value={3} icon={Server} color="green" trend="Active" />);
    expect(screen.getByText('Active')).toBeTruthy();
  });

  it('does not render trend badge when omitted', () => {
    render(<StatsCard title="Stopped" value={2} icon={Server} color="gray" />);
    expect(screen.queryByText('Active')).toBeNull();
  });

  it('renders zero value', () => {
    render(<StatsCard title="Running" value={0} icon={Server} color="green" />);
    expect(screen.getByText('0')).toBeTruthy();
  });
});

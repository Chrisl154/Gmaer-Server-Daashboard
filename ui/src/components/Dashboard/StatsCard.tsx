import React from 'react';
import { LucideIcon } from 'lucide-react';
import { cn } from '../../utils/cn';

interface StatsCardProps {
  title: string;
  value: string | number;
  icon: LucideIcon;
  color: 'orange' | 'blue' | 'green' | 'gray';
  trend?: string;
}

const COLOR_STYLES: Record<
  StatsCardProps['color'],
  { iconBg: string; iconColor: string; trendBg: string; trendColor: string }
> = {
  orange: {
    iconBg: 'linear-gradient(135deg, #f97316, #ea580c)',
    iconColor: '#ffffff',
    trendBg: 'rgba(249,115,22,0.1)',
    trendColor: '#fb923c',
  },
  blue: {
    iconBg: 'linear-gradient(135deg, #3b82f6, #1d4ed8)',
    iconColor: '#ffffff',
    trendBg: 'rgba(59,130,246,0.1)',
    trendColor: '#60a5fa',
  },
  green: {
    iconBg: 'rgba(34,197,94,0.15)',
    iconColor: '#4ade80',
    trendBg: 'rgba(34,197,94,0.1)',
    trendColor: '#4ade80',
  },
  gray: {
    iconBg: 'rgba(148,163,184,0.1)',
    iconColor: '#94a3b8',
    trendBg: 'rgba(148,163,184,0.1)',
    trendColor: '#94a3b8',
  },
};

export function StatsCard({ title, value, icon: Icon, color, trend }: StatsCardProps) {
  const styles = COLOR_STYLES[color];

  return (
    <div className="card p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div
          className="w-11 h-11 rounded-xl flex items-center justify-center shrink-0"
          style={{ background: styles.iconBg }}
        >
          <Icon className="w-5 h-5" style={{ color: styles.iconColor }} />
        </div>
        {trend && (
          <span
            className="text-xs font-medium px-2 py-0.5 rounded-full"
            style={{ background: styles.trendBg, color: styles.trendColor }}
          >
            {trend}
          </span>
        )}
      </div>

      <div>
        <div
          className="text-3xl font-bold leading-none mb-1"
          style={{ color: 'var(--text-primary)' }}
        >
          {value}
        </div>
        <div className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
          {title}
        </div>
      </div>
    </div>
  );
}

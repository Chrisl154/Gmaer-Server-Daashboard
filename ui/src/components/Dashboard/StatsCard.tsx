import React from 'react';
import { clsx } from 'clsx';

interface StatsCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  color: 'blue' | 'green' | 'red' | 'gray' | 'yellow';
}

const COLOR_MAP = {
  blue:   'bg-blue-500/10 text-blue-400',
  green:  'bg-green-500/10 text-green-400',
  red:    'bg-red-500/10 text-red-400',
  gray:   'bg-gray-500/10 text-gray-400',
  yellow: 'bg-yellow-500/10 text-yellow-400',
};

export function StatsCard({ icon, label, value, color }: StatsCardProps) {
  return (
    <div className="bg-[#141414] border border-[#252525] rounded-xl p-4">
      <div className="flex items-center gap-3 mb-3">
        <div className={clsx('w-8 h-8 rounded-lg flex items-center justify-center', COLOR_MAP[color])}>
          {icon}
        </div>
        <span className="text-xs text-gray-400">{label}</span>
      </div>
      <div className="text-2xl font-semibold text-gray-100">{value}</div>
    </div>
  );
}

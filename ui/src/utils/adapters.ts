import { Server, Swords, Factory, PawPrint, Leaf, Sprout, Sword, Zap } from 'lucide-react';

export const ADAPTER_ICONS: Record<string, React.FC<{ className?: string }>> = {
  valheim:      Sword,
  minecraft:    Swords,
  satisfactory: Factory,
  palworld:     PawPrint,
  eco:          Leaf,
  enshrouded:   Sprout,
  riftbreaker:  Zap,
  default:      Server,
};

export const ADAPTER_NAMES: Record<string, string> = {
  valheim:      'Valheim',
  minecraft:    'Minecraft',
  satisfactory: 'Satisfactory',
  palworld:     'Palworld',
  eco:          'Eco',
  enshrouded:   'Enshrouded',
  riftbreaker:  'The Riftbreaker',
};

export const ADAPTER_COLORS: Record<string, string> = {
  valheim:      '#4a90d9',
  minecraft:    '#5c8a3c',
  satisfactory: '#f5a623',
  palworld:     '#7b68ee',
  eco:          '#4caf50',
  enshrouded:   '#9c7c4a',
  riftbreaker:  '#e74c3c',
};

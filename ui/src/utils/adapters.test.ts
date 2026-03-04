import { describe, it, expect } from 'vitest';
import { ADAPTER_NAMES, ADAPTER_COLORS } from './adapters';

const SUPPORTED_ADAPTERS = [
  'valheim',
  'minecraft',
  'satisfactory',
  'palworld',
  'eco',
  'enshrouded',
  'riftbreaker',
] as const;

describe('ADAPTER_NAMES', () => {
  it('has entries for all supported adapters', () => {
    for (const adapter of SUPPORTED_ADAPTERS) {
      expect(ADAPTER_NAMES[adapter]).toBeTruthy();
    }
  });

  it('returns human-readable names', () => {
    expect(ADAPTER_NAMES.valheim).toBe('Valheim');
    expect(ADAPTER_NAMES.minecraft).toBe('Minecraft');
    expect(ADAPTER_NAMES.satisfactory).toBe('Satisfactory');
    expect(ADAPTER_NAMES.palworld).toBe('Palworld');
    expect(ADAPTER_NAMES.eco).toBe('Eco');
    expect(ADAPTER_NAMES.enshrouded).toBe('Enshrouded');
    expect(ADAPTER_NAMES.riftbreaker).toBe('The Riftbreaker');
  });
});

describe('ADAPTER_COLORS', () => {
  it('has colour entries for all supported adapters', () => {
    for (const adapter of SUPPORTED_ADAPTERS) {
      expect(ADAPTER_COLORS[adapter]).toMatch(/^#[0-9a-fA-F]{6}$/);
    }
  });

  it('returns distinct colours', () => {
    const colors = SUPPORTED_ADAPTERS.map(a => ADAPTER_COLORS[a]);
    const unique = new Set(colors);
    expect(unique.size).toBe(SUPPORTED_ADAPTERS.length);
  });
});

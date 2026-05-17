export type UnitScale = {
  divisor: number;
  unitLabel: string;
  joiner: string;
};

const clampPrecision = (precision: number) => (precision < 0 ? 0 : precision);

export const resolveUnitScale = (unit?: string, maxValue: number | null = null): UnitScale => {
  const normalized = unit?.trim().toLowerCase() ?? '';
  if (!normalized) return { divisor: 1, unitLabel: '', joiner: '' };
  if (normalized === '%') return { divisor: 1, unitLabel: '%', joiner: '' };

  const max = Math.max(0, maxValue ?? 0);
  const binaryUnits = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;
  const binaryPerSecUnits = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s', 'PB/s'] as const;
  const iopsUnits = ['IOPS', 'kIOPS', 'MIOPS', 'GIOPS'] as const;

  const pickScaledUnit = (base: number, units: readonly string[]) => {
    let index = 0;
    let value = max;
    while (value >= base && index < units.length - 1) {
      value /= base;
      index += 1;
    }
    return { divisor: base ** index, unitLabel: units[index] };
  };

  if (normalized === 'b' || normalized === 'bytes') {
    const scaled = pickScaledUnit(1024, binaryUnits);
    return { ...scaled, joiner: ' ' };
  }

  if (normalized === 'b/s' || normalized === 'bps') {
    const scaled = pickScaledUnit(1024, binaryPerSecUnits);
    return { ...scaled, joiner: ' ' };
  }

  if (normalized.includes('iops')) {
    const scaled = pickScaledUnit(1000, iopsUnits);
    return { ...scaled, joiner: ' ' };
  }

  return { divisor: 1, unitLabel: unit ?? '', joiner: ' ' };
};

export const formatScaledValue = (
  value: number | null,
  precision: number,
  unitScale: UnitScale,
  includeUnit: boolean,
  fallback: string = 'N/A',
): string => {
  if (value === null || !Number.isFinite(value)) return fallback;
  const digits = clampPrecision(precision);
  const scaled = value / unitScale.divisor;
  const numeric = digits === 0 ? String(Math.round(scaled)) : scaled.toFixed(digits);
  if (!includeUnit || !unitScale.unitLabel) return numeric;
  return `${numeric}${unitScale.joiner}${unitScale.unitLabel}`;
};

export const computeSeriesStats = (values: Array<number | null | undefined>) => {
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  let maxAbs = 0;
  let sum = 0;
  let count = 0;

  values.forEach((value) => {
    if (value == null || !Number.isFinite(value)) return;
    count += 1;
    sum += value;
    if (value < min) min = value;
    if (value > max) max = value;
    const abs = Math.abs(value);
    if (abs > maxAbs) maxAbs = abs;
  });

  if (count === 0) {
    return { min: null, max: null, avg: null, maxAbs: 0, count: 0 };
  }

  return {
    min,
    max,
    avg: sum / count,
    maxAbs,
    count,
  };
};

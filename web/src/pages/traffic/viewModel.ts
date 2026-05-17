import type { TrafficStats } from '@app-types/traffic';

const byteUnits = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;
const bitUnits = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps', 'Pbps'] as const;

const formatScaled = (value: number, units: readonly string[], base: number, precision = 2) => {
  if (!Number.isFinite(value) || value <= 0) return `0 ${units[0]}`;
  const index = Math.min(units.length - 1, Math.floor(Math.log(value) / Math.log(base)));
  const scaled = value / Math.pow(base, index);
  return `${parseFloat(scaled.toFixed(precision))} ${units[index]}`;
};

export const formatTrafficBytes = (bytes: number): string =>
  formatScaled(bytes, byteUnits, 1024, 2);

export const formatBandwidth = (bytesPerSec: number): string =>
  formatScaled(bytesPerSec * 8, bitUnits, 1000, 2);

export const formatOptionalBandwidth = (bytesPerSec: number | null): string =>
  bytesPerSec === null ? '-' : formatBandwidth(bytesPerSec);

export const formatCoverage = (ratio: number): string => {
  if (!Number.isFinite(ratio) || ratio <= 0) return '0%';
  return `${Math.min(100, ratio * 100).toFixed(2)}%`;
};

export const selectedTrafficText = (stats: TrafficStats): string =>
  formatTrafficBytes(stats.selected_bytes);

export const formatCycleRange = (
  start: string,
  end: string,
  locale: string,
  timezone: string,
): string => {
  const options: Intl.DateTimeFormatOptions = {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: timezone || undefined,
  };
  const fmt = new Intl.DateTimeFormat(locale, options);
  const startDate = new Date(start);
  const endDate = new Date(end);
  if (Number.isNaN(startDate.getTime()) || Number.isNaN(endDate.getTime())) return '-';
  return `${fmt.format(startDate)} - ${fmt.format(endDate)}`;
};

export const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === 'AbortError';

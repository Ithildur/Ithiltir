import type uPlot from 'uplot';

export type MetricPoint = {
  timestamp: number;
  value: number | null;
};

type ChartValueStats = {
  max: number;
  maxAbs: number;
};

const NICE_TICKS = [1, 1.2, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10];

export const isDarkTheme = () =>
  typeof document !== 'undefined' && document.documentElement.classList.contains('dark');

export const themeColor = (name: string) =>
  typeof document === 'undefined'
    ? ''
    : getComputedStyle(document.documentElement).getPropertyValue(name).trim();

export const xLabel = (timestampSec: number, withTime: boolean) => {
  if (!Number.isFinite(timestampSec)) return '';
  const date = new Date(timestampSec * 1000);
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  if (!withTime) return `${month}-${day}`;
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${month}-${day} ${hours}:${minutes}`;
};

export const niceMax = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  const stepIndex = NICE_TICKS.findIndex((tick) => tick >= normalized);
  const step = stepIndex >= 0 ? NICE_TICKS[stepIndex] : 10;
  const isExact = Math.abs(step - normalized) < 1e-6;
  if (isExact && stepIndex >= 0 && stepIndex < NICE_TICKS.length - 1) {
    return NICE_TICKS[stepIndex + 1] * magnitude;
  }
  return step * magnitude;
};

const trimTrailingZeros = (value: string) =>
  value.replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1');

export const axisLabel = (value: number, divisor: number) => {
  if (!Number.isFinite(value) || !Number.isFinite(divisor) || divisor === 0) return '';
  const scaled = value / divisor;
  const abs = Math.abs(scaled);
  const digits = abs >= 10 ? 0 : abs >= 1 ? 1 : 2;
  const text = digits === 0 ? String(Math.round(scaled)) : scaled.toFixed(digits);
  return trimTrailingZeros(text);
};

export const sortMetricPoints = (data: MetricPoint[]) => {
  if (data.length <= 1) return data;
  return [...data].sort((a, b) => a.timestamp - b.timestamp);
};

export const alignMetricPoints = (points: MetricPoint[]): uPlot.AlignedData | null => {
  if (points.length === 0) return null;
  const xs: number[] = [];
  const ys: Array<number | null> = [];

  points.forEach((point) => {
    if (!Number.isFinite(point.timestamp)) return;
    // uPlot time scale expects seconds, input is milliseconds.
    xs.push(Math.floor(point.timestamp / 1000));
    ys.push(typeof point.value === 'number' && Number.isFinite(point.value) ? point.value : null);
  });

  return xs.length > 0 ? [xs, ys] : null;
};

export const chartSpanDays = (aligned: uPlot.AlignedData | null) => {
  if (!aligned || aligned[0].length < 2) return 0;
  const xs = aligned[0] as number[];
  const min = xs[0];
  const max = xs[xs.length - 1];
  if (!Number.isFinite(min) || !Number.isFinite(max) || max <= min) return 0;
  return (max - min) / 86400;
};

export const chartValueStats = (aligned: uPlot.AlignedData | null): ChartValueStats => {
  if (!aligned || aligned.length < 2) {
    return { max: 0, maxAbs: 0 };
  }
  const values = aligned[1] as Array<number | null>;
  let max = 0;
  let maxAbs = 0;
  values.forEach((value) => {
    if (value == null || !Number.isFinite(value)) return;
    if (value > max) max = value;
    const abs = Math.abs(value);
    if (abs > maxAbs) maxAbs = abs;
  });
  return { max, maxAbs };
};

export const latestMetricValue = (points: MetricPoint[]) => {
  for (let i = points.length - 1; i >= 0; i -= 1) {
    const value = points[i].value;
    if (typeof value === 'number' && Number.isFinite(value)) return value;
  }
  return null;
};

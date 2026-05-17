import type { MetricHistoryPoint } from '@app-types/metricsHistory';
import type { NodeView } from '@app-types/frontMetrics';
import type { MetricPoint } from '@components/dashboard/MetricHistoryChart';
import { computeSeriesStats, resolveUnitScale } from '@utils/metricFormat';
import { formatLocalDateTime } from '@utils/time';
import type { Lang } from '@i18n';
import type { MetricConfig, MetricSection, MetricTransform } from './config';

export type MetricStats = {
  min: number | null;
  max: number | null;
  avg: number | null;
  count: number;
  unitScale: ReturnType<typeof resolveUnitScale>;
};

export type StatsLookup = Map<string, MetricStats>;
export type SeriesLookup = Record<string, MetricPoint[]>;

export const metricSeriesKey = (metric: MetricConfig): string => metric.seriesKey ?? metric.metric;

export const formatDateTime = (value: number, lang: Lang): string => {
  if (!Number.isFinite(value)) return '';
  return formatLocalDateTime(value, lang);
};

export const configBySeriesKey = (metricSections: MetricSection[]): Map<string, MetricConfig> => {
  const entries: Array<[string, MetricConfig]> = [];
  metricSections.forEach((section) => {
    section.metrics.forEach((metric) => entries.push([metricSeriesKey(metric), metric]));
  });
  return new Map(entries);
};

export const metricPoints = (
  points: MetricHistoryPoint[],
  transform?: MetricTransform,
): MetricPoint[] =>
  points.map((point) => {
    const value =
      point.value == null || !Number.isFinite(point.value)
        ? null
        : transform
          ? transform(point.value)
          : point.value;

    return {
      timestamp: Date.parse(point.ts),
      value,
    };
  });

export const findLatestTimestamp = (points: MetricPoint[]): number | null => {
  let latest = 0;
  points.forEach((point) => {
    if (Number.isFinite(point.timestamp) && point.timestamp > latest) {
      latest = point.timestamp;
    }
  });
  return latest > 0 ? latest : null;
};

export const findLastUpdatedAt = (seriesLookup: SeriesLookup): number | null =>
  findLatestTimestamp(Object.values(seriesLookup).flat());

export const statsLookup = (
  metricSections: MetricSection[],
  seriesLookup: SeriesLookup,
): StatsLookup => {
  const map: StatsLookup = new Map();

  metricSections.forEach((section) => {
    section.metrics.forEach((metric) => {
      const values = (seriesLookup[metricSeriesKey(metric)] ?? []).map((point) => point.value);
      const stats = computeSeriesStats(values);
      map.set(metricSeriesKey(metric), {
        min: stats.min,
        max: stats.max,
        avg: stats.avg,
        count: stats.count,
        unitScale: resolveUnitScale(metric.unit, stats.maxAbs),
      });
    });
  });

  return map;
};

export const resolveMetricDevice = (
  metric: MetricConfig,
  ioDevice: string,
  mountDevice: string,
): string | undefined => {
  if (metric.historyDevice) return metric.historyDevice;
  if (metric.deviceKey === 'io') return ioDevice || undefined;
  if (metric.deviceKey === 'mount') return mountDevice || undefined;
  return undefined;
};

export const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === 'AbortError';

const hasTemperature = (value: number | undefined): boolean =>
  typeof value === 'number' && Number.isFinite(value) && value > 0;

export const hasCpuTemperature = (node: NodeView | null): boolean =>
  Boolean(
    node?.thermal?.sensors?.some(
      (sensor) => sensor.kind.trim().toLowerCase() === 'cpu' && hasTemperature(sensor.temp_c),
    ),
  );

export const diskTemperatureMetrics = (node: NodeView | null): MetricConfig[] => {
  const devices = node?.disk?.temperature_devices ?? [];
  const seen = new Set<string>();
  const metrics: MetricConfig[] = [];

  devices.forEach((device) => {
    const disk = device.trim();
    if (!disk || seen.has(disk)) return;
    seen.add(disk);
    metrics.push({
      seriesKey: `disk.temp_c:${disk}`,
      metric: 'disk.temp_c',
      titleKey: 'stats_disk_temp',
      label: disk,
      unit: '°C',
      precision: 1,
      aggregation: 'max',
      historyDevice: disk,
    });
  });

  return metrics;
};

const diskTemperatureMetric = (historyAvailable: boolean): MetricConfig => ({
  metric: 'disk.temp_c',
  titleKey: 'stats_disk_temp',
  unit: '°C',
  precision: 1,
  aggregation: 'max',
  historyAvailable,
});

export const visibleMetricSections = (
  sections: MetricSection[],
  node: NodeView | null,
): MetricSection[] => {
  const known = node != null;
  const cpuAvailable = known && hasCpuTemperature(node);
  const diskMetrics = diskTemperatureMetrics(node);
  const diskAvailable = diskMetrics.length > 0;
  const showTemperatures = cpuAvailable || diskAvailable;

  const visible = sections
    .map((section): MetricSection | null => {
      const metrics = section.metrics.flatMap((metric) => {
        if (!metric.availabilityKey) return metric;
        if (!showTemperatures) return [];
        return { ...metric, historyAvailable: cpuAvailable };
      });
      return metrics.length > 0 ? { ...section, metrics } : null;
    })
    .filter((section): section is MetricSection => section != null);

  if (showTemperatures) {
    visible.push({
      key: 'disk_temperature',
      titleKey: 'stats_disk_temp',
      metrics: diskAvailable ? diskMetrics : [diskTemperatureMetric(false)],
    });
  }
  return visible;
};

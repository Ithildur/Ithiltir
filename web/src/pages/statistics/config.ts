import type { TranslationKey } from '@i18n';
import type { MetricHistoryAggregation, MetricHistoryRange } from '@app-types/metricsHistory';

export type MetricTransform = (value: number | null) => number | null;

export type MetricConfig = {
  seriesKey?: string;
  metric: string;
  titleKey: TranslationKey;
  label?: string;
  unit?: string;
  precision?: number;
  deviceKey?: 'io' | 'mount';
  historyDevice?: string;
  historyAvailable?: boolean;
  availabilityKey?: 'cpuTemperature';
  aggregation?: MetricHistoryAggregation;
  transform?: MetricTransform;
};

export type MetricSection = {
  key: string;
  titleKey: TranslationKey;
  metrics: MetricConfig[];
};

export const RANGE_OPTIONS: MetricHistoryRange[] = ['30m', '1h', '12h', '24h', '1w', '15d', '30d'];
export const AGGREGATION_OPTIONS: MetricHistoryAggregation[] = ['avg', 'max', 'min', 'last'];
export const DEFAULT_AGGREGATION: MetricHistoryAggregation = 'avg';

export const buildMetricSections = (percentTransform: MetricTransform): MetricSection[] => [
  {
    key: 'cpu',
    titleKey: 'cpu',
    metrics: [
      {
        metric: 'cpu.usage_ratio',
        titleKey: 'stats_cpu_usage_ratio',
        unit: '%',
        precision: 1,
        transform: percentTransform,
      },
      { metric: 'cpu.load1', titleKey: 'stats_load_1m', precision: 2 },
      { metric: 'cpu.load5', titleKey: 'stats_load_5m', precision: 2 },
      { metric: 'cpu.load15', titleKey: 'stats_load_15m', precision: 2 },
      {
        metric: 'cpu.temp_c',
        titleKey: 'stats_cpu_temp',
        unit: '°C',
        precision: 1,
        aggregation: 'max',
        availabilityKey: 'cpuTemperature',
      },
    ],
  },
  {
    key: 'memory',
    titleKey: 'memory',
    metrics: [
      { metric: 'mem.used', titleKey: 'stats_mem_used_amount', unit: 'B', precision: 0 },
      {
        metric: 'mem.used_ratio',
        titleKey: 'stats_mem_used_ratio',
        unit: '%',
        precision: 1,
        transform: percentTransform,
      },
    ],
  },
  {
    key: 'network',
    titleKey: 'network',
    metrics: [
      { metric: 'net.recv_bps', titleKey: 'stats_net_recv_bps', unit: 'B/s', precision: 0 },
      { metric: 'net.sent_bps', titleKey: 'stats_net_sent_bps', unit: 'B/s', precision: 0 },
    ],
  },
  {
    key: 'connections',
    titleKey: 'conn_short',
    metrics: [
      { metric: 'conn.tcp', titleKey: 'tcp_conn_count', precision: 0, aggregation: 'max' },
      { metric: 'conn.udp', titleKey: 'udp_conn_count', precision: 0, aggregation: 'max' },
    ],
  },
  {
    key: 'processes',
    titleKey: 'procs_short',
    metrics: [{ metric: 'proc.count', titleKey: 'procs_short', precision: 0, aggregation: 'last' }],
  },
  {
    key: 'disk_io',
    titleKey: 'stats_disk_io_section',
    metrics: [
      {
        metric: 'disk.read_bps',
        titleKey: 'disk_io_read',
        unit: 'B/s',
        precision: 0,
        deviceKey: 'io',
      },
      {
        metric: 'disk.write_bps',
        titleKey: 'disk_io_write',
        unit: 'B/s',
        precision: 0,
        deviceKey: 'io',
      },
      {
        metric: 'disk.read_iops',
        titleKey: 'disk_iops_read',
        precision: 0,
        deviceKey: 'io',
      },
      {
        metric: 'disk.write_iops',
        titleKey: 'disk_iops_write',
        precision: 0,
        deviceKey: 'io',
      },
    ],
  },
  {
    key: 'disk_usage',
    titleKey: 'stats_partition_section',
    metrics: [
      {
        metric: 'disk.used',
        titleKey: 'stats_disk_used',
        unit: 'B',
        precision: 0,
        deviceKey: 'mount',
      },
      {
        metric: 'disk.used_ratio',
        titleKey: 'stats_disk_used_ratio',
        unit: '%',
        precision: 1,
        deviceKey: 'mount',
        transform: percentTransform,
      },
    ],
  },
];

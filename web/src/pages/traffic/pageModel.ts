import Gauge from 'lucide-react/dist/esm/icons/gauge';
import Network from 'lucide-react/dist/esm/icons/network';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import type { LucideIcon } from 'lucide-react';
import type { TrafficDailyItem, TrafficStats, TrafficSummary } from '@app-types/traffic';
import type { I18nValue } from '@i18n';
import {
  formatBandwidth,
  formatCoverage,
  formatCycleRange,
  formatOptionalBandwidth,
  formatTrafficBytes,
  selectedTrafficText,
} from './viewModel';

export type CurrentStatTone = 'default' | 'accent' | 'warning' | 'muted';

export type TrafficChartMode = 'previous_daily' | 'current_daily' | 'monthly';

export type TrafficDetailErrorKey = 'traffic_error';

export type TrafficHeroStatTone = 'accent' | 'warning' | 'slate' | 'red';

type Translate = I18nValue['t'];

export type ChartPoint = {
  key: string;
  label: string;
  title: string;
  inBytes: number;
  outBytes: number;
};

export type CurrentStatItem = {
  key: string;
  label: string;
  value: string;
};

export type CurrentStatGroup = {
  key: string;
  label: string;
  tone: CurrentStatTone;
  items: CurrentStatItem[];
};

export type TrafficHeroStatItem = {
  key: string;
  label: string;
  value: string;
  icon: LucideIcon;
  tone: TrafficHeroStatTone;
  labelEmphasis: boolean;
};

export type CoverageStats = {
  sampleCount: number;
  expectedSampleCount: number;
  coverageRatio: number;
};

export const localeFor = (lang: string) => (lang === 'zh' ? 'zh-CN' : 'en-US');

const formatCycleLabel = (start: string, locale: string, timezone: string): string => {
  const date = new Date(start);
  if (Number.isNaN(date.getTime())) return '-';
  return new Intl.DateTimeFormat(locale, {
    month: '2-digit',
    day: '2-digit',
    timeZone: timezone || undefined,
  }).format(date);
};

const monthName = (start: string, locale: string, timezone: string): string => {
  const date = new Date(start);
  if (Number.isNaN(date.getTime())) return '-';
  return new Intl.DateTimeFormat(locale, {
    month: 'short',
    timeZone: timezone || undefined,
  }).format(date);
};

export const dailyPointFrom = (
  item: TrafficDailyItem,
  locale: string,
  timezone: string,
): ChartPoint => ({
  key: `${item.start}-${item.iface}`,
  label: formatCycleLabel(item.start, locale, timezone),
  title: formatCycleRange(item.start, item.end, locale, timezone),
  inBytes: item.stats.in_bytes,
  outBytes: item.stats.out_bytes,
});

export const monthlyPointFrom = (item: TrafficSummary, locale: string): ChartPoint => ({
  key: `${item.cycle.start}-${item.iface}`,
  label: monthName(item.cycle.start, locale, item.cycle.timezone),
  title: formatCycleRange(item.cycle.start, item.cycle.end, locale, item.cycle.timezone),
  inBytes: item.stats.in_bytes,
  outBytes: item.stats.out_bytes,
});

export const coverageStatsFrom = (stats: TrafficStats[]): CoverageStats | null => {
  if (stats.length === 0) return null;
  const sampleCount = stats.reduce((total, item) => total + item.sample_count, 0);
  const expectedSampleCount = stats.reduce((total, item) => total + item.expected_sample_count, 0);
  const coverageRatio =
    expectedSampleCount <= 0 ? 1 : Math.min(1, sampleCount / expectedSampleCount);
  return {
    sampleCount,
    expectedSampleCount,
    coverageRatio,
  };
};

export const currentStatToneFor = (
  directionMode: TrafficSummary['direction_mode'],
  selectedDirection: TrafficStats['selected_bytes_direction'],
  group: 'in' | 'out',
): CurrentStatTone => {
  if (directionMode === 'out') {
    return group === 'out' ? 'warning' : 'muted';
  }
  if (directionMode === 'both') {
    return group === 'out' ? 'warning' : 'accent';
  }
  if (selectedDirection === 'in' || selectedDirection === 'out') {
    return group === selectedDirection ? (group === 'out' ? 'warning' : 'accent') : 'muted';
  }
  return group === 'out' ? 'warning' : 'accent';
};

export const buildTrafficHeroStats = ({
  summary,
  showCoverage,
  coverageWarning,
  t,
}: {
  summary: TrafficSummary | null;
  showCoverage: boolean;
  coverageWarning: boolean;
  t: Translate;
}): TrafficHeroStatItem[] => {
  if (!summary) return [];

  const stats = summary.stats;
  return [
    {
      key: 'selected',
      label: t('traffic_selected_total'),
      value: selectedTrafficText(stats),
      icon: Network,
      tone: 'accent',
      labelEmphasis: true,
    },
    ...(stats.p95_enabled
      ? [
          {
            key: 'p95',
            label: t('traffic_selected_p95'),
            value: formatOptionalBandwidth(stats.selected_p95_bytes_per_sec),
            icon: Gauge,
            tone: 'warning' as const,
            labelEmphasis: false,
          },
        ]
      : []),
    {
      key: 'peak',
      label: t('traffic_selected_peak'),
      value: formatBandwidth(stats.selected_peak_bytes_per_sec),
      icon: Gauge,
      tone: 'slate',
      labelEmphasis: false,
    },
    ...(showCoverage
      ? [
          {
            key: 'coverage',
            label: t('traffic_coverage'),
            value: formatCoverage(stats.coverage_ratio),
            icon: RefreshCw,
            tone: coverageWarning ? ('red' as const) : ('slate' as const),
            labelEmphasis: false,
          },
        ]
      : []),
  ];
};

export const buildTrafficCurrentStatGroups = (
  summary: TrafficSummary | null,
  t: Translate,
): CurrentStatGroup[] => {
  if (!summary) return [];

  const stats = summary.stats;
  return [
    {
      key: 'out',
      label: t('traffic_outbound'),
      tone: currentStatToneFor(summary.direction_mode, stats.selected_bytes_direction, 'out'),
      items: [
        {
          key: 'out_total',
          label: t('traffic_out_total'),
          value: formatTrafficBytes(stats.out_bytes),
        },
        ...(stats.p95_enabled
          ? [
              {
                key: 'out_p95',
                label: t('traffic_out_p95'),
                value: formatOptionalBandwidth(stats.out_p95_bytes_per_sec),
              },
            ]
          : []),
        {
          key: 'out_peak',
          label: t('traffic_out_peak'),
          value: formatBandwidth(stats.out_peak_bytes_per_sec),
        },
      ],
    },
    {
      key: 'in',
      label: t('traffic_inbound'),
      tone: currentStatToneFor(summary.direction_mode, stats.selected_bytes_direction, 'in'),
      items: [
        {
          key: 'in_total',
          label: t('traffic_in_total'),
          value: formatTrafficBytes(stats.in_bytes),
        },
        ...(stats.p95_enabled
          ? [
              {
                key: 'in_p95',
                label: t('traffic_in_p95'),
                value: formatOptionalBandwidth(stats.in_p95_bytes_per_sec),
              },
            ]
          : []),
        {
          key: 'in_peak',
          label: t('traffic_in_peak'),
          value: formatBandwidth(stats.in_peak_bytes_per_sec),
        },
      ],
    },
  ];
};

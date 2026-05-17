import React from 'react';
import ArrowLeft from 'lucide-react/dist/esm/icons/arrow-left';
import CircleHelp from 'lucide-react/dist/esm/icons/circle-help';
import Gauge from 'lucide-react/dist/esm/icons/gauge';
import Network from 'lucide-react/dist/esm/icons/network';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import type { LucideIcon } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import type {
  TrafficDaily,
  TrafficDailyItem,
  TrafficMonthly,
  TrafficStats,
  TrafficSummary,
} from '@app-types/traffic';
import Button from '@components/ui/Button';
import Select from '@components/ui/Select';
import ThemeToggle from '@components/ui/ThemeToggle';
import { Tooltip } from '@components/ui/Tooltip';
import { useI18n, type TranslationKey } from '@i18n';
import { useBootstrapAuth } from '@hooks/useBootstrapAuth';
import {
  fetchTrafficDaily,
  fetchTrafficIfaces,
  fetchTrafficMonthly,
  fetchTrafficSummary,
} from '@lib/statisticsApi';
import {
  formatBandwidth,
  formatCoverage,
  formatCycleRange,
  formatOptionalBandwidth,
  formatTrafficBytes,
  isAbortError,
  selectedTrafficText,
} from './viewModel';

const chartWidth = 760;
const chartHeight = 280;
const chartBasePad = { left: 48, right: 28, top: 26, bottom: 44 };
const chartAxisLabelGap = 10;
const chartAxisLabelCharWidth = 7;
const trafficCoverageWarningThreshold = 0.995;
const monthlyHistoryMonths = 12;

type ChartMode = 'previous_daily' | 'current_daily' | 'monthly';

type ChartPoint = {
  key: string;
  label: string;
  title: string;
  inBytes: number;
  outBytes: number;
};

const chartModeOptions: { mode: ChartMode; label: TranslationKey; daily: boolean }[] = [
  { mode: 'previous_daily', label: 'traffic_chart_previous_daily', daily: true },
  { mode: 'current_daily', label: 'traffic_chart_current_daily', daily: true },
  { mode: 'monthly', label: 'traffic_chart_monthly', daily: false },
];

const isDailyChartMode = (mode: ChartMode) => mode !== 'monthly';

const localeFor = (lang: string) => (lang === 'zh' ? 'zh-CN' : 'en-US');

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

const dailyPointFrom = (item: TrafficDailyItem, locale: string, timezone: string): ChartPoint => ({
  key: `${item.start}-${item.iface}`,
  label: formatCycleLabel(item.start, locale, timezone),
  title: formatCycleRange(item.start, item.end, locale, timezone),
  inBytes: item.stats.in_bytes,
  outBytes: item.stats.out_bytes,
});

const monthlyPointFrom = (item: TrafficSummary, locale: string): ChartPoint => ({
  key: `${item.cycle.start}-${item.iface}`,
  label: monthName(item.cycle.start, locale, item.cycle.timezone),
  title: formatCycleRange(item.cycle.start, item.cycle.end, locale, item.cycle.timezone),
  inBytes: item.stats.in_bytes,
  outBytes: item.stats.out_bytes,
});

const statTone = {
  accent: 'text-(--theme-fg-accent)',
  warning: 'text-(--theme-fg-warning-strong)',
  slate: 'text-(--theme-fg-muted)',
  red: 'text-(--theme-bg-danger-emphasis)',
} as const;

type StatTone = keyof typeof statTone;

const HeroStat = ({
  label,
  value,
  detail,
  icon: Icon,
  tone = 'slate',
}: {
  label: string;
  value: string;
  detail?: string;
  icon: LucideIcon;
  tone?: StatTone;
}) => (
  <div className="min-w-0 py-3 md:border-l md:border-(--theme-border-subtle) md:pl-8 first:md:border-l-0 first:md:pl-0 dark:md:border-(--theme-border-default)">
    <div className="flex min-w-0 items-center gap-2">
      <Icon size={15} className={statTone[tone]} />
      <div className="truncate text-[11px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-muted)">
        {label}
      </div>
    </div>
    <div className="mt-2 truncate font-mono text-2xl font-semibold tracking-tight text-(--theme-fg-default)">
      {value}
    </div>
    {detail && <div className="mt-1 truncate text-xs text-(--theme-fg-muted)">{detail}</div>}
  </div>
);

type CurrentStatTone = 'default' | 'accent' | 'warning';

type CurrentStatItem = {
  key: string;
  label: string;
  value: string;
};

type CurrentStatGroup = {
  key: string;
  label: string;
  tone: CurrentStatTone;
  items: CurrentStatItem[];
};

type CoverageStats = {
  sampleCount: number;
  expectedSampleCount: number;
  coverageRatio: number;
};

const coverageStatsFrom = (stats: TrafficStats[]): CoverageStats | null => {
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

const currentStatValueClass = (tone: CurrentStatTone = 'default') => {
  switch (tone) {
    case 'accent':
      return 'font-mono font-semibold text-(--theme-fg-accent)';
    case 'warning':
      return 'font-mono font-semibold text-(--theme-fg-warning-strong)';
    default:
      return 'font-mono font-semibold text-(--theme-fg-default)';
  }
};

const currentStatDotClass = (tone: CurrentStatTone = 'default') => {
  switch (tone) {
    case 'accent':
      return 'bg-(--theme-fg-accent)';
    case 'warning':
      return 'bg-(--theme-fg-warning-strong)';
    default:
      return 'bg-(--theme-fg-muted)';
  }
};

const HealthChip = ({ label, value }: { label: string; value: string }) => (
  <span className="inline-flex items-center gap-1.5 text-xs font-medium text-(--theme-fg-muted)">
    <span>{label}</span>
    <span className="font-mono text-(--theme-fg-default)">{value}</span>
  </span>
);

const TrafficTrendChart = ({
  points,
  mode,
  dailyAvailable,
  onModeChange,
}: {
  points: ChartPoint[];
  mode: ChartMode;
  dailyAvailable: boolean;
  onModeChange: (mode: ChartMode) => void;
}) => {
  const { t } = useI18n();
  const dailyUnavailable = isDailyChartMode(mode) && !dailyAvailable;
  const maxValue = Math.max(
    1,
    ...points.flatMap((item) => [item.inBytes, item.outBytes, item.inBytes + item.outBytes]),
  );
  const axisValues = [maxValue, maxValue / 2, 0];
  const axisLabels = axisValues.map((value) => (dailyUnavailable ? '' : formatTrafficBytes(value)));
  const axisLabelWidth =
    Math.max(0, ...axisLabels.map((label) => label.length)) * chartAxisLabelCharWidth;
  const chartPad = {
    ...chartBasePad,
    left: Math.max(chartBasePad.left, Math.ceil(axisLabelWidth + chartAxisLabelGap + 2)),
  };
  const innerWidth = chartWidth - chartPad.left - chartPad.right;
  const innerHeight = chartHeight - chartPad.top - chartPad.bottom;
  const barStep = points.length > 0 ? innerWidth / points.length : innerWidth;
  const barWidth = Math.min(18, Math.max(3, barStep / 4));
  const pointInset = Math.max(6, barWidth + 4);
  const usableWidth = Math.max(1, innerWidth - pointInset * 2);
  const step = points.length > 1 ? usableWidth / (points.length - 1) : usableWidth;
  const yFor = (value: number) => chartPad.top + innerHeight - (value / maxValue) * innerHeight;
  const xFor = (index: number) =>
    points.length > 1 ? chartPad.left + pointInset + index * step : chartPad.left + innerWidth / 2;
  const linePoints = points.map((item, index) => ({
    x: xFor(index),
    y: yFor(item.inBytes + item.outBytes),
  }));
  const linePath = linePoints
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(1)} ${point.y.toFixed(1)}`)
    .join(' ');
  const areaPath =
    linePoints.length > 0
      ? `${linePath} L ${linePoints[linePoints.length - 1].x.toFixed(1)} ${chartPad.top + innerHeight} L ${linePoints[0].x.toFixed(1)} ${chartPad.top + innerHeight} Z`
      : '';
  const gradientId = React.useId().replace(/:/g, '');
  const labelEvery = Math.max(1, Math.ceil(points.length / 8));
  const dailyUnavailableText = t('traffic_daily_billing_only');
  const placeholderBars = [0.42, 0.58, 0.36, 0.68, 0.5, 0.74, 0.46, 0.62];

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <div className="text-sm font-semibold text-(--theme-fg-default)">
            {t('traffic_trend_chart')}
          </div>
          <div className="inline-flex rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) p-0.5 dark:border-(--theme-border-default)">
            {chartModeOptions.map((option) => {
              const selected = mode === option.mode;
              const unavailable = option.daily && !dailyAvailable;
              const button = (
                <button
                  type="button"
                  className={`rounded px-2.5 py-1 text-xs font-semibold transition-colors disabled:cursor-not-allowed ${
                    selected
                      ? unavailable
                        ? 'bg-(--theme-bg-muted) text-(--theme-fg-muted)'
                        : 'bg-(--theme-bg-accent-emphasis) text-(--theme-fg-on-emphasis)'
                      : unavailable
                        ? 'text-(--theme-fg-muted) opacity-60'
                        : 'text-(--theme-fg-muted) hover:text-(--theme-fg-default)'
                  }`}
                  disabled={unavailable}
                  onClick={() => onModeChange(option.mode)}
                >
                  {t(option.label)}
                </button>
              );
              return option.daily ? (
                <Tooltip
                  key={option.mode}
                  content={dailyAvailable ? null : dailyUnavailableText}
                  className="inline-flex"
                >
                  {button}
                </Tooltip>
              ) : (
                <React.Fragment key={option.mode}>{button}</React.Fragment>
              );
            })}
          </div>
          {dailyUnavailable && (
            <Tooltip content={dailyUnavailableText} className="inline-flex">
              <span className="inline-flex size-5 cursor-help items-center justify-center text-(--theme-fg-muted)">
                <CircleHelp size={14} />
              </span>
            </Tooltip>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs text-(--theme-fg-muted)">
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-(--theme-fg-accent)" />
            {t('traffic_in_total')}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-(--theme-fg-warning-strong)" />
            {t('traffic_out_total')}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-0.5 w-4 rounded-full bg-(--theme-fg-default)" />
            {t('traffic_total_trend')}
          </span>
        </div>
      </div>

      <svg
        className={`block h-auto w-full ${dailyUnavailable ? 'opacity-60 saturate-0' : ''}`}
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        role="img"
        aria-label={t('traffic_trend_chart')}
      >
        <defs>
          <linearGradient id={`${gradientId}-area`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--theme-fg-accent)" stopOpacity="0.2" />
            <stop offset="100%" stopColor="var(--theme-fg-accent)" stopOpacity="0" />
          </linearGradient>
        </defs>

        {axisValues.map((value, index) => {
          const y = yFor(value);
          return (
            <g key={value}>
              <line
                x1={chartPad.left}
                x2={chartWidth - chartPad.right}
                y1={y}
                y2={y}
                stroke="var(--theme-border-subtle)"
                strokeDasharray="4 6"
              />
              <text
                x={chartPad.left - chartAxisLabelGap}
                y={y + 4}
                textAnchor="end"
                className="fill-(--theme-fg-muted) text-[11px] font-medium"
              >
                {axisLabels[index]}
              </text>
            </g>
          );
        })}

        {areaPath && <path d={areaPath} fill={`url(#${gradientId}-area)`} />}

        {dailyUnavailable && (
          <g opacity="0.55">
            {placeholderBars.map((height, index) => {
              const x = chartPad.left + (innerWidth / placeholderBars.length) * index + 10;
              const width = Math.max(8, innerWidth / placeholderBars.length - 22);
              const y = chartPad.top + innerHeight - innerHeight * height;
              return (
                <rect
                  key={index}
                  x={x}
                  y={y}
                  width={width}
                  height={innerHeight * height}
                  rx="3"
                  fill="var(--theme-fg-muted)"
                  opacity="0.25"
                />
              );
            })}
            <path
              d={`M ${chartPad.left + 12} ${chartPad.top + innerHeight * 0.64} C ${chartPad.left + innerWidth * 0.24} ${chartPad.top + innerHeight * 0.5}, ${chartPad.left + innerWidth * 0.38} ${chartPad.top + innerHeight * 0.72}, ${chartPad.left + innerWidth * 0.56} ${chartPad.top + innerHeight * 0.42} S ${chartPad.left + innerWidth * 0.86} ${chartPad.top + innerHeight * 0.34}, ${chartPad.left + innerWidth - 12} ${chartPad.top + innerHeight * 0.48}`}
              fill="none"
              stroke="var(--theme-fg-muted)"
              strokeLinecap="round"
              strokeWidth="2.25"
            />
          </g>
        )}

        {points.length === 0 && (
          <text
            x={chartPad.left + innerWidth / 2}
            y={chartPad.top + innerHeight / 2}
            textAnchor="middle"
            className="fill-(--theme-fg-muted) text-sm font-semibold"
          >
            {dailyUnavailable ? dailyUnavailableText : t('traffic_no_data')}
          </text>
        )}

        {points.map((item, index) => {
          const x = xFor(index);
          const inHeight = chartPad.top + innerHeight - yFor(item.inBytes);
          const outHeight = chartPad.top + innerHeight - yFor(item.outBytes);
          const baseY = chartPad.top + innerHeight;
          const showLabel = index === 0 || index === points.length - 1 || index % labelEvery === 0;
          return (
            <g key={item.key}>
              <title>{`${item.title} · ${formatTrafficBytes(item.inBytes + item.outBytes)}`}</title>
              <rect
                x={x - barWidth - 2}
                y={baseY - inHeight}
                width={barWidth}
                height={Math.max(1, inHeight)}
                rx="3"
                fill="var(--theme-fg-accent)"
                opacity="0.7"
              />
              <rect
                x={x + 2}
                y={baseY - outHeight}
                width={barWidth}
                height={Math.max(1, outHeight)}
                rx="3"
                fill="var(--theme-fg-warning-strong)"
                opacity="0.72"
              />
              {showLabel && (
                <text
                  x={x}
                  y={chartHeight - 16}
                  textAnchor="middle"
                  className="fill-(--theme-fg-muted) text-[11px] font-semibold"
                >
                  {item.label}
                </text>
              )}
            </g>
          );
        })}

        {linePath && (
          <path
            d={linePath}
            fill="none"
            stroke="var(--theme-fg-default)"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2.25"
          />
        )}
        {linePoints.map((point) => (
          <circle
            key={`${point.x}-${point.y}`}
            cx={point.x}
            cy={point.y}
            r="3"
            fill="var(--theme-bg-default)"
            stroke="var(--theme-fg-default)"
            strokeWidth="1.75"
          />
        ))}
      </svg>
    </div>
  );
};

const Page = () => {
  useBootstrapAuth();
  const { t, lang } = useI18n();
  const { serverId } = useParams();
  const numericServerId = serverId ? Number(serverId) : Number.NaN;
  const isValidServerId = Number.isFinite(numericServerId) && numericServerId > 0;
  const locale = localeFor(lang);

  const [ifaces, setIfaces] = React.useState<string[]>([]);
  const [iface, setIface] = React.useState('');
  const [summary, setSummary] = React.useState<TrafficSummary | null>(null);
  const [currentDaily, setCurrentDaily] = React.useState<TrafficDaily>({ items: [] });
  const [previousDaily, setPreviousDaily] = React.useState<TrafficDaily>({ items: [] });
  const [monthly, setMonthly] = React.useState<TrafficMonthly>({
    includes_current: true,
    items: [],
  });
  const [ifaceServerId, setIfaceServerId] = React.useState<number | null>(null);
  const [chartMode, setChartMode] = React.useState<ChartMode>('current_daily');
  const [loading, setLoading] = React.useState(false);
  const [errorKey, setErrorKey] = React.useState<TranslationKey | null>(null);
  const trafficRequestRef = React.useRef(0);

  React.useEffect(() => {
    if (!isValidServerId) return;
    const controller = new AbortController();
    setIfaces([]);
    setIface('');
    setIfaceServerId(null);
    fetchTrafficIfaces({ serverId: numericServerId, signal: controller.signal })
      .then((items) => {
        const names = items
          .map((item) => item.name.trim())
          .filter((name) => name && name.toLowerCase() !== 'all');
        setIfaces(names);
        setIfaceServerId(numericServerId);
        setIface((current) => (current && names.includes(current) ? current : (names[0] ?? '')));
      })
      .catch((error) => {
        if (isAbortError(error)) return;
        setErrorKey('traffic_error');
      });
    return () => controller.abort();
  }, [isValidServerId, numericServerId]);

  const loadTraffic = React.useCallback(
    async (signal?: AbortSignal) => {
      if (!isValidServerId) {
        trafficRequestRef.current += 1;
        setLoading(false);
        return;
      }
      if (!iface || ifaceServerId !== numericServerId) {
        trafficRequestRef.current += 1;
        setLoading(false);
        setSummary(null);
        setCurrentDaily({ items: [] });
        setPreviousDaily({ items: [] });
        setMonthly({ includes_current: true, items: [] });
        return;
      }

      const requestID = ++trafficRequestRef.current;
      const isCurrentRequest = () => trafficRequestRef.current === requestID && !signal?.aborted;

      setLoading(true);
      setErrorKey(null);
      try {
        const params = {
          serverId: numericServerId,
          iface,
          signal,
        };
        const [nextSummary, nextMonthly] = await Promise.all([
          fetchTrafficSummary(params),
          fetchTrafficMonthly({ ...params, months: monthlyHistoryMonths }),
        ]);
        let nextCurrentDaily: TrafficDaily = { items: [] };
        let nextPreviousDaily: TrafficDaily = { items: [] };
        if (nextSummary.usage_mode === 'billing') {
          [nextCurrentDaily, nextPreviousDaily] = await Promise.all([
            fetchTrafficDaily({ ...params, period: 'current' }),
            fetchTrafficDaily({ ...params, period: 'previous' }),
          ]);
        }
        if (!isCurrentRequest()) return;
        setSummary(nextSummary);
        setCurrentDaily(nextCurrentDaily);
        setPreviousDaily(nextPreviousDaily);
        setMonthly(nextMonthly);
      } catch (error) {
        if (isAbortError(error) || !isCurrentRequest()) return;
        setSummary(null);
        setCurrentDaily({ items: [] });
        setPreviousDaily({ items: [] });
        setMonthly({ includes_current: true, items: [] });
        setErrorKey('traffic_error');
      } finally {
        if (isCurrentRequest()) {
          setLoading(false);
        }
      }
    },
    [iface, ifaceServerId, isValidServerId, numericServerId],
  );

  React.useEffect(() => {
    const controller = new AbortController();
    void loadTraffic(controller.signal);
    return () => controller.abort();
  }, [loadTraffic]);

  const serverLabel =
    summary?.server_name?.trim() || (isValidServerId ? `#${numericServerId}` : '');
  const showP95 = Boolean(summary?.stats.p95_enabled);
  const showCoverage = summary?.usage_mode === 'billing';
  const summaryCoverageWarning = Boolean(
    summary && summary.stats.coverage_ratio < trafficCoverageWarningThreshold,
  );
  const directionLabel = summary
    ? t(`traffic_direction_${summary.direction_mode}` as TranslationKey)
    : '';
  const cycleLabel = summary ? t(`traffic_cycle_${summary.cycle.mode}` as TranslationKey) : '';
  const currentDailyPoints = React.useMemo(() => {
    const timezone = summary?.cycle.timezone || '';
    return currentDaily.items.map((item) => dailyPointFrom(item, locale, timezone));
  }, [currentDaily.items, locale, summary?.cycle.timezone]);
  const previousDailyPoints = React.useMemo(() => {
    const timezone = summary?.cycle.timezone || '';
    return previousDaily.items.map((item) => dailyPointFrom(item, locale, timezone));
  }, [locale, previousDaily.items, summary?.cycle.timezone]);
  const monthlyPoints = React.useMemo(
    () =>
      monthly.items
        .slice(0, monthlyHistoryMonths)
        .reverse()
        .map((item) => monthlyPointFrom(item, locale)),
    [locale, monthly.items],
  );
  const dailyAvailable = summary?.usage_mode === 'billing';
  const chartPoints =
    chartMode === 'current_daily'
      ? currentDailyPoints
      : chartMode === 'previous_daily'
        ? previousDailyPoints
        : monthlyPoints;
  const chartCoverageStats = React.useMemo(() => {
    if (!showCoverage) return null;
    if (chartMode === 'current_daily') {
      return coverageStatsFrom(currentDaily.items.map((item) => item.stats));
    }
    if (chartMode === 'previous_daily') {
      return coverageStatsFrom(previousDaily.items.map((item) => item.stats));
    }
    return coverageStatsFrom(
      monthly.items.slice(0, monthlyHistoryMonths).map((item) => item.stats),
    );
  }, [chartMode, currentDaily.items, monthly.items, previousDaily.items, showCoverage]);
  const chartCoverageWarning = Boolean(
    chartCoverageStats && chartCoverageStats.coverageRatio < trafficCoverageWarningThreshold,
  );

  const statItems = React.useMemo(() => {
    if (!summary) return [];
    const stats = summary.stats;
    return [
      {
        key: 'selected',
        label: t('traffic_selected_total'),
        value: selectedTrafficText(stats),
        icon: Network,
        tone: 'accent' as const,
      },
      ...(stats.p95_enabled
        ? [
            {
              key: 'p95',
              label: t('traffic_selected_p95'),
              value: formatOptionalBandwidth(stats.selected_p95_bytes_per_sec),
              icon: Gauge,
              tone: 'warning' as const,
            },
          ]
        : []),
      {
        key: 'peak',
        label: t('traffic_selected_peak'),
        value: formatBandwidth(stats.selected_peak_bytes_per_sec),
        icon: Gauge,
        tone: 'slate' as const,
      },
      ...(showCoverage
        ? [
            {
              key: 'coverage',
              label: t('traffic_coverage'),
              value: formatCoverage(stats.coverage_ratio),
              icon: RefreshCw,
              tone: summaryCoverageWarning ? ('red' as const) : ('slate' as const),
            },
          ]
        : []),
    ];
  }, [showCoverage, summary, summaryCoverageWarning, t]);

  const currentStatGroups = React.useMemo<CurrentStatGroup[]>(() => {
    if (!summary) return [];
    const stats = summary.stats;
    return [
      {
        key: 'out',
        label: t('traffic_outbound'),
        tone: 'warning',
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
        tone: 'accent',
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
  }, [summary, t]);

  if (!isValidServerId) {
    return (
      <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
        <main className="mx-auto max-w-410 px-4 py-12 sm:px-6 lg:px-8">
          <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-6 text-sm text-(--theme-fg-muted) dark:border-(--theme-border-default)">
            {t('stats_no_server')}
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
      <header className="sticky top-0 z-40 border-b border-(--theme-border-subtle) bg-(--theme-surface-control) backdrop-blur-md dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)/90">
        <div className="mx-auto flex h-16 max-w-410 items-center justify-between px-4 sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <Link
              to="/"
              className="inline-flex items-center gap-2 text-xs font-semibold text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default)"
            >
              <ArrowLeft size={14} />
              <span>{t('common_back_to_dashboard')}</span>
            </Link>
            <div className="h-5 w-px bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold text-(--theme-fg-default)">
                {t('traffic_title')}
              </div>
              <div className="truncate text-xs text-(--theme-fg-muted)">{serverLabel}</div>
            </div>
          </div>
          <div className="hidden items-center gap-3 md:flex">
            <ThemeToggle size="sm" variant="soft" />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-410 space-y-5 px-4 py-8 sm:px-6 lg:px-8">
        <section className="border-b border-(--theme-border-subtle) pb-6 dark:border-(--theme-border-default)">
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <h1 className="truncate text-3xl font-semibold tracking-tight text-(--theme-fg-default)">
                  {t('traffic_title')}
                </h1>
                <span className="h-6 w-px bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
                <span className="truncate text-xl font-medium text-(--theme-fg-muted)">
                  {serverLabel}
                </span>
              </div>
              <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-(--theme-fg-muted)">
                <span className="font-semibold text-(--theme-fg-default)">
                  {t('traffic_current_cycle')}
                </span>
                {summary && (
                  <span>
                    {formatCycleRange(
                      summary.cycle.start,
                      summary.cycle.end,
                      locale,
                      summary.cycle.timezone,
                    )}
                  </span>
                )}
                {summary && <span>{cycleLabel}</span>}
                {summary && <span>{directionLabel}</span>}
              </div>
            </div>

            <div className="flex flex-wrap items-end gap-2 lg:justify-end">
              <label className="grid min-w-48 gap-1">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-muted)">
                  {t('traffic_iface')}
                </span>
                <Select
                  value={iface}
                  disabled={ifaces.length === 0}
                  onChange={(event) => setIface(event.target.value)}
                >
                  {ifaces.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </Select>
              </label>
              <Button
                type="button"
                variant="secondary"
                icon={RefreshCw}
                onClick={() => void loadTraffic()}
                disabled={loading}
              >
                {t('stats_refresh')}
              </Button>
            </div>
          </div>
        </section>

        {errorKey && <div className="text-sm text-(--theme-fg-danger)">{t(errorKey)}</div>}
        {loading && !summary && (
          <div className="text-sm text-(--theme-fg-muted)">{t('stats_loading')}</div>
        )}
        {!loading && !errorKey && !summary && (
          <div className="text-sm text-(--theme-fg-muted)">{t('traffic_no_data')}</div>
        )}

        {summary && (
          <>
            <section className="divide-y divide-(--theme-border-subtle) border-b border-(--theme-border-subtle) dark:divide-(--theme-border-default) dark:border-(--theme-border-default)">
              <div className="grid gap-x-8 gap-y-3 py-4 sm:grid-cols-2 lg:grid-cols-4">
                {statItems.map((item, index) => (
                  <div
                    key={item.key}
                    className={
                      index === 0
                        ? ''
                        : 'lg:border-l lg:border-(--theme-border-subtle) lg:pl-8 lg:dark:border-(--theme-border-default)'
                    }
                  >
                    <HeroStat
                      label={item.label}
                      value={item.value}
                      icon={item.icon}
                      tone={item.tone}
                    />
                  </div>
                ))}
              </div>

              <div className="grid gap-8 py-7 xl:grid-cols-[minmax(0,1fr)_360px]">
                <div className="min-w-0">
                  <TrafficTrendChart
                    points={chartPoints}
                    mode={chartMode}
                    dailyAvailable={dailyAvailable}
                    onModeChange={setChartMode}
                  />

                  {chartCoverageStats && (
                    <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2">
                      <HealthChip
                        label={t('traffic_samples')}
                        value={`${chartCoverageStats.sampleCount}/${chartCoverageStats.expectedSampleCount}`}
                      />
                      <HealthChip
                        label={t('traffic_coverage')}
                        value={formatCoverage(chartCoverageStats.coverageRatio)}
                      />
                      {chartCoverageWarning && (
                        <span className="text-xs font-semibold text-(--theme-fg-warning-strong)">
                          {t('traffic_coverage_low')}
                        </span>
                      )}
                    </div>
                  )}
                </div>

                <aside className="border-t border-(--theme-border-subtle) pt-6 xl:border-l xl:border-t-0 xl:pl-8 xl:pt-0 dark:border-(--theme-border-default)">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-(--theme-fg-default)">
                        {t('traffic_current_cycle_stats')}
                      </div>
                      <div className="mt-1 text-xs text-(--theme-fg-muted)">{directionLabel}</div>
                    </div>
                  </div>
                  <div className="mt-5 grid gap-5 text-sm">
                    {currentStatGroups.map((group) => (
                      <section key={group.key} className="min-w-0">
                        <div
                          className={`mb-3 flex items-center gap-2 text-xs font-semibold ${currentStatValueClass(group.tone)}`}
                        >
                          <span
                            className={`size-2 rounded-full ${currentStatDotClass(group.tone)}`}
                          />
                          <span>{group.label}</span>
                        </div>
                        <dl className="grid gap-2.5">
                          {group.items.map((item) => (
                            <div
                              key={item.key}
                              className="flex min-w-0 items-center justify-between gap-4"
                            >
                              <dt className="min-w-0 text-(--theme-fg-muted)">{item.label}</dt>
                              <dd
                                className={`min-w-0 text-right ${currentStatValueClass(group.tone)}`}
                              >
                                {item.value}
                              </dd>
                            </div>
                          ))}
                        </dl>
                      </section>
                    ))}
                  </div>
                </aside>
              </div>
            </section>

            <section>
              <div className="py-4">
                <div className="text-sm font-semibold text-(--theme-fg-default)">
                  {t('traffic_monthly_history')}
                </div>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-(--theme-border-subtle) text-sm dark:divide-(--theme-border-default)">
                  <thead className="bg-(--theme-bg-muted)">
                    <tr className="text-left text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                      <th className="px-4 py-3">{t('traffic_cycle')}</th>
                      <th className="px-4 py-3">{t('traffic_in_total')}</th>
                      <th className="px-4 py-3">{t('traffic_out_total')}</th>
                      <th className="px-4 py-3">{t('traffic_in_peak')}</th>
                      <th className="px-4 py-3">{t('traffic_out_peak')}</th>
                      {showP95 && <th className="px-4 py-3">{t('traffic_in_p95')}</th>}
                      {showP95 && <th className="px-4 py-3">{t('traffic_out_p95')}</th>}
                      {showCoverage && <th className="px-4 py-3">{t('traffic_coverage')}</th>}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-(--theme-border-subtle) dark:divide-(--theme-border-default)">
                    {monthly.items.map((item) => (
                      <tr
                        key={`${item.cycle.start}-${item.iface}`}
                        className="bg-(--theme-bg-default) transition-colors hover:bg-(--theme-surface-row-hover)"
                      >
                        <td className="whitespace-nowrap px-4 py-3 font-mono text-xs">
                          {formatCycleRange(
                            item.cycle.start,
                            item.cycle.end,
                            locale,
                            item.cycle.timezone,
                          )}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatTrafficBytes(item.stats.in_bytes)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatTrafficBytes(item.stats.out_bytes)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatBandwidth(item.stats.in_peak_bytes_per_sec)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatBandwidth(item.stats.out_peak_bytes_per_sec)}
                        </td>
                        {showP95 && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatOptionalBandwidth(item.stats.in_p95_bytes_per_sec)}
                          </td>
                        )}
                        {showP95 && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatOptionalBandwidth(item.stats.out_p95_bytes_per_sec)}
                          </td>
                        )}
                        {showCoverage && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatCoverage(item.stats.coverage_ratio)}
                          </td>
                        )}
                      </tr>
                    ))}
                    {monthly.items.length === 0 && (
                      <tr>
                        <td
                          colSpan={(showP95 ? 7 : 5) + (showCoverage ? 1 : 0)}
                          className="px-4 py-6 text-sm text-(--theme-fg-muted)"
                        >
                          {t('traffic_no_data')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  );
};

export default Page;

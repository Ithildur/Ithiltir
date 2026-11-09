import React from 'react';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import MetricHistoryChart from '@components/dashboard/MetricHistoryChart';
import Button from '@components/ui/Button';
import Select from '@components/ui/Select';
import { useI18n, type TranslationKey } from '@i18n';
import { ApiError } from '@lib/api';
import { fetchPVEHistory } from '@lib/virtApi';
import type { PVEConsolidation, PVEHistory, PVEHistoryPoint, PVETimeframe } from '@app-types/virt';
import type { NodeVMSettings } from '@app-types/api';
import { isCanceledRequestError } from '@utils/errors';
import { formatScaledValue, resolveUnitScale } from '@utils/metricFormat';

const timeframes: PVETimeframe[] = ['hour', 'day', 'week', 'month', 'year'];
// Proxmox RRD palette: the second series is used memory, outgoing traffic, or writes.
const green = '#94ae0a';
const blue = '#115fa6';
const charts: {
  field: Exclude<keyof PVEHistoryPoint, 'timestamp'>;
  label: TranslationKey;
  seriesLabel?: TranslationKey;
  unit: string;
  factor?: number;
  color: string;
  secondary?: {
    field: Exclude<keyof PVEHistoryPoint, 'timestamp'>;
    label: TranslationKey;
    color: string;
    fill?: string;
  };
}[] = [
  { field: 'cpu_ratio', label: 'cpu', unit: '%', factor: 100, color: green },
  {
    field: 'memory_bytes',
    label: 'memory',
    seriesLabel: 'pve_memory_used',
    unit: 'B',
    color: blue,
    secondary: {
      field: 'memory_limit_bytes',
      label: 'pve_memory_limit',
      color: green,
      fill: `${green}99`,
    },
  },
  {
    field: 'net_in_bytes_per_second',
    label: 'network',
    seriesLabel: 'pve_net_in',
    unit: 'bit/s',
    factor: 8,
    color: green,
    secondary: {
      field: 'net_out_bytes_per_second',
      label: 'pve_net_out',
      color: blue,
      fill: `${blue}99`,
    },
  },
  {
    field: 'disk_read_bytes_per_second',
    label: 'pve_disk_io',
    seriesLabel: 'pve_disk_read',
    unit: 'B/s',
    color: green,
    secondary: {
      field: 'disk_write_bytes_per_second',
      label: 'pve_disk_write',
      color: blue,
      fill: `${blue}99`,
    },
  },
];

const errorKey = (error: unknown): TranslationKey => {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'forbidden':
        return 'pve_forbidden';
      case 'node_offline':
        return 'pve_history_offline';
      case 'unsupported':
        return 'pve_history_unsupported';
      case 'vm_not_found':
      case 'not_found':
        return 'pve_vm_missing';
      case 'busy':
        return 'pve_history_busy';
      case 'deadline_exceeded':
        return 'pve_history_timeout';
    }
  }
  return 'pve_history_error';
};

const VMHistory = ({
  serverId,
  vmId,
  settings,
}: {
  serverId: number;
  vmId: number;
  settings: NodeVMSettings;
}) => {
  const { t } = useI18n();
  const [timeframe, setTimeframe] = React.useState<PVETimeframe>('hour');
  const [consolidation, setConsolidation] = React.useState<PVEConsolidation>('AVERAGE');
  const [history, setHistory] = React.useState<PVEHistory | null>(null);
  const [error, setError] = React.useState<TranslationKey | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [revision, setRevision] = React.useState(0);

  React.useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    void fetchPVEHistory(serverId, vmId, timeframe, consolidation, controller.signal)
      .then((next) => {
        if (!controller.signal.aborted) setHistory(next);
      })
      .catch((failure: unknown) => {
        if (controller.signal.aborted || isCanceledRequestError(failure)) return;
        setHistory(null);
        setError(errorKey(failure));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [serverId, vmId, timeframe, consolidation, revision]);

  const points =
    history?.timeframe === timeframe && history.consolidation === consolidation
      ? history.points
      : [];

  return (
    <section
      aria-label={t('pve_history')}
      className="space-y-3 border-t border-(--theme-border-subtle) pt-3"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{t('pve_history')}</h3>
        <div className="flex flex-wrap gap-2">
          <Select
            width="auto"
            aria-label={t('stats_range')}
            value={timeframe}
            onChange={(event) => setTimeframe(event.target.value as PVETimeframe)}
          >
            {timeframes.map((range) => (
              <option key={range} value={range}>
                {t(`pve_range_${range}`)}
              </option>
            ))}
          </Select>
          <Select
            width="auto"
            aria-label={t('pve_consolidation')}
            value={consolidation}
            onChange={(event) => setConsolidation(event.target.value as PVEConsolidation)}
          >
            <option value="AVERAGE">{t('pve_average')}</option>
            <option value="MAX">{t('pve_max')}</option>
          </Select>
          <Button
            variant="secondary"
            icon={RefreshCw}
            disabled={loading}
            onClick={() => setRevision((value) => value + 1)}
          >
            {t('stats_refresh')}
          </Button>
        </div>
      </div>
      {error ? (
        <p role="alert" className="py-6 text-sm text-(--theme-fg-danger)">
          {t(error)}
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
          {charts.map(({ field, label, seriesLabel, unit, factor = 1, color, secondary }) => {
            const thresholdPercent =
              field === 'cpu_ratio'
                ? settings.vm_cpu_threshold
                : field === 'memory_bytes' && settings.vm_memory_enabled
                  ? settings.vm_memory_threshold
                  : null;
            const series = points.map((point) => ({
              timestamp: Date.parse(point.timestamp),
              value: point[field] == null ? null : point[field] * factor,
              threshold:
                thresholdPercent == null || point[field] == null
                  ? null
                  : field === 'cpu_ratio'
                    ? thresholdPercent
                    : point.memory_limit_bytes != null && point.memory_limit_bytes > 0
                      ? (point.memory_limit_bytes * thresholdPercent) / 100
                      : null,
            }));
            const exceeded = series.filter(
              (point) =>
                point.value != null && point.threshold != null && point.value > point.threshold,
            ).length;
            const secondarySeries = secondary
              ? {
                  label: t(secondary.label),
                  data: points.map((point) => {
                    const value = point[secondary.field];
                    return {
                      timestamp: Date.parse(point.timestamp),
                      value: value == null ? null : value * factor,
                    };
                  }),
                  stroke: secondary.color,
                  fill: secondary.fill,
                }
              : undefined;
            const legend = [
              { label: t(seriesLabel ?? label), color, data: series },
              ...(secondarySeries
                ? [
                    {
                      label: secondarySeries.label,
                      color: secondarySeries.stroke,
                      data: secondarySeries.data,
                    },
                  ]
                : []),
            ];
            return (
              <div key={field} className="min-w-0">
                <div className="mb-2 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-xs">
                  <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    <h4 className="font-semibold text-(--theme-fg-muted)">{t(label)}</h4>
                    {thresholdPercent != null && (
                      <span className="text-(--theme-fg-muted)">
                        {t('pve_threshold_value', { value: `${thresholdPercent}%` })}
                      </span>
                    )}
                    {!loading && exceeded > 0 && (
                      <span className="text-(--theme-fg-warning-strong)">
                        {t('pve_exceeded_points', { count: exceeded })}
                      </span>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-x-3 gap-y-1">
                    {legend.map((metric) => {
                      const latest =
                        metric.data.findLast((point) => point.value != null)?.value ?? null;
                      return (
                        <span key={metric.label} className="inline-flex items-center gap-1.5">
                          {secondarySeries && (
                            <>
                              <span
                                aria-hidden="true"
                                className="size-2 shrink-0 rounded-full"
                                style={{ backgroundColor: metric.color }}
                              />
                              <span className="text-(--theme-fg-muted)">{metric.label}</span>
                            </>
                          )}
                          <span className="font-mono">
                            {formatScaledValue(
                              latest,
                              1,
                              resolveUnitScale(unit, latest),
                              true,
                              '—',
                            )}
                          </span>
                        </span>
                      );
                    })}
                  </div>
                </div>
                <MetricHistoryChart
                  title={t(seriesLabel ?? label)}
                  variant="inline"
                  height={180}
                  showXAxis
                  showYAxisUnit
                  xAxisMaxLabels={2}
                  unit={unit}
                  stroke={color}
                  fill={`${color}99`}
                  loading={loading}
                  data={series}
                  secondarySeries={secondarySeries}
                />
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
};

export default VMHistory;

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
const metricGroups = ['resources', 'disk'] as const;
type MetricGroup = (typeof metricGroups)[number];
const charts: {
  group: MetricGroup;
  field: Exclude<keyof PVEHistoryPoint, 'timestamp'>;
  label: TranslationKey;
  unit: string;
  factor?: number;
}[] = [
  { group: 'resources', field: 'cpu_ratio', label: 'cpu', unit: '%', factor: 100 },
  { group: 'resources', field: 'memory_bytes', label: 'memory', unit: 'B' },
  { group: 'resources', field: 'net_in_bytes_per_second', label: 'pve_net_in', unit: 'B/s' },
  { group: 'resources', field: 'net_out_bytes_per_second', label: 'pve_net_out', unit: 'B/s' },
  { group: 'disk', field: 'disk_read_bytes_per_second', label: 'pve_disk_read', unit: 'B/s' },
  { group: 'disk', field: 'disk_write_bytes_per_second', label: 'pve_disk_write', unit: 'B/s' },
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
  const [metricGroup, setMetricGroup] = React.useState<MetricGroup>('resources');

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
      <div
        role="group"
        aria-label={t('pve_metric_group')}
        className="flex flex-wrap gap-1 border-b border-(--theme-border-subtle)"
      >
        {metricGroups.map((group) => (
          <Button
            key={group}
            variant="plain"
            size="none"
            aria-pressed={metricGroup === group}
            onClick={() => setMetricGroup(group)}
            className={`rounded-none border-b-2 px-3 py-2 text-xs tracking-normal active:scale-100 ${metricGroup === group ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)' : 'border-transparent text-(--theme-fg-muted) hover:text-(--theme-fg-default)'}`}
          >
            {t(`pve_metrics_${group}`)}
          </Button>
        ))}
      </div>
      {error ? (
        <p role="alert" className="py-6 text-sm text-(--theme-fg-danger)">
          {t(error)}
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
          {charts
            .filter((chart) => chart.group === metricGroup)
            .map(({ field, label, unit, factor = 1 }) => {
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
              const latest = series.findLast((point) => point.value != null)?.value ?? null;
              const scale = resolveUnitScale(
                unit,
                series.reduce((max, point) => Math.max(max, point.value ?? 0), 0),
              );
              return (
                <div key={field} className="min-w-0">
                  <div className="mb-2 flex items-baseline justify-between gap-3 text-xs">
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
                    <span className="font-mono">
                      {formatScaledValue(latest, 1, scale, true, '—')}
                    </span>
                  </div>
                  <MetricHistoryChart
                    title={t(label)}
                    variant="inline"
                    height={180}
                    showXAxis
                    xAxisMaxLabels={2}
                    unit={unit}
                    loading={loading}
                    data={series}
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

import React from 'react';
import type { TranslationKey } from '@i18n';
import type { MetricHistoryAggregation, MetricHistoryRange } from '@app-types/metricsHistory';
import type { MetricPoint } from '@components/dashboard/MetricHistoryChart';
import { fetchMetricHistory } from '@lib/metricsHistoryApi';
import { computeSeriesStats, resolveUnitScale } from '@utils/metricFormat';
import { DEFAULT_AGGREGATION } from '../config';
import type { MetricConfig } from '../config';
import { metricPoints, findLatestTimestamp, isAbortError, resolveMetricDevice } from '../viewModel';

type UseStatisticsDetailParams = {
  numericServerId: number;
  isValidServerId: boolean;
  range: MetricHistoryRange;
  ioDevice: string;
  mountDevice: string;
};

export const useStatisticsDetail = ({
  numericServerId,
  isValidServerId,
  range,
  ioDevice,
  mountDevice,
}: UseStatisticsDetailParams) => {
  const abortRef = React.useRef<AbortController | null>(null);

  const [isOpen, setIsOpen] = React.useState(false);
  const [metric, setMetric] = React.useState<MetricConfig | null>(null);
  const [selectedRange, setSelectedRange] = React.useState<MetricHistoryRange>(range);
  const [aggregation, setAggregation] =
    React.useState<MetricHistoryAggregation>(DEFAULT_AGGREGATION);
  const [series, setSeries] = React.useState<MetricPoint[]>([]);
  const [stepSec, setStepSec] = React.useState<number | null>(null);
  const [updatedAt, setUpdatedAt] = React.useState<number | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const [errorKey, setErrorKey] = React.useState<TranslationKey | null>(null);

  React.useEffect(() => {
    if (!isOpen) {
      abortRef.current?.abort();
      setSeries([]);
      setStepSec(null);
      setUpdatedAt(null);
      setIsLoading(false);
      setErrorKey(null);
    }
  }, [isOpen]);

  const unitScale = React.useMemo(() => {
    if (!metric) return resolveUnitScale(undefined, 0);
    const values = series.map((point) => point.value);
    const stats = computeSeriesStats(values);
    return resolveUnitScale(metric.unit, stats.maxAbs);
  }, [metric, series]);

  const selectedDevice = React.useMemo(
    () => (metric ? resolveMetricDevice(metric, ioDevice, mountDevice) : undefined),
    [metric, ioDevice, mountDevice],
  );

  const deviceLabel = React.useMemo(() => {
    if (metric?.historyDevice) return metric.historyDevice;
    if (!metric?.deviceKey) return '';
    return selectedDevice || '';
  }, [metric, selectedDevice]);

  const reload = React.useCallback(async () => {
    if (!metric || !isValidServerId) return;

    const device = selectedDevice;
    if (metric.deviceKey && !device) {
      setErrorKey(metric.deviceKey === 'io' ? 'stats_device_required' : 'stats_partition_required');
      setSeries([]);
      setStepSec(null);
      setUpdatedAt(null);
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setIsLoading(true);
    setErrorKey(null);
    let aborted = false;

    try {
      const response = await fetchMetricHistory({
        serverId: numericServerId,
        metric: metric.metric,
        range: selectedRange,
        aggregation,
        device,
        signal: controller.signal,
      });

      const nextSeries = metricPoints(response.points, metric.transform);
      setSeries(nextSeries);
      setStepSec(response.step_sec);
      setUpdatedAt(findLatestTimestamp(nextSeries));
    } catch (error) {
      if (isAbortError(error)) {
        aborted = true;
        return;
      }
      setErrorKey('stats_error');
    } finally {
      if (!aborted) {
        setIsLoading(false);
      }
    }
  }, [aggregation, isValidServerId, metric, numericServerId, selectedDevice, selectedRange]);

  React.useEffect(() => {
    if (!isOpen || !metric) return;
    void reload();
  }, [aggregation, isOpen, metric, selectedRange, reload]);

  const open = React.useCallback(
    (nextMetric: MetricConfig) => {
      setMetric(nextMetric);
      setSelectedRange(range);
      setAggregation(nextMetric.aggregation ?? DEFAULT_AGGREGATION);
      setIsOpen(true);
      setErrorKey(null);
    },
    [range],
  );

  const close = React.useCallback(() => {
    setIsOpen(false);
    setMetric(null);
  }, []);

  return {
    isOpen,
    metric,
    range: selectedRange,
    setRange: setSelectedRange,
    aggregation,
    setAggregation,
    series,
    stepSec,
    updatedAt,
    isLoading,
    errorKey,
    unitLabel: unitScale.unitLabel || metric?.unit || '',
    deviceLabel,
    reload,
    open,
    close,
  };
};

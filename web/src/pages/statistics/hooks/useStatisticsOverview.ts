import React from 'react';
import type { TranslationKey } from '@i18n';
import type { NodeView } from '@app-types/frontMetrics';
import { fetchFrontMetrics } from '@lib/frontApi';
import { fetchMetricHistory } from '@lib/metricsHistoryApi';
import type { MetricSection } from '../config';
import {
  configBySeriesKey,
  metricPoints,
  statsLookup as collectStats,
  findLastUpdatedAt,
  metricSeriesKey,
  isAbortError,
  type SeriesLookup,
  resolveMetricDevice,
  visibleMetricSections,
} from '../viewModel';
import { DEFAULT_AGGREGATION } from '../config';
import type { MetricHistoryAggregation, MetricHistoryRange } from '@app-types/metricsHistory';

type UseStatisticsOverviewParams = {
  numericServerId: number;
  isValidServerId: boolean;
  metricSections: MetricSection[];
};

const hasLoadedRequests = (loaded: Map<string, Set<string>>): boolean => {
  for (const keys of loaded.values()) {
    if (keys.size > 0) return true;
  }
  return false;
};

const historyKey = (
  serverId: number,
  metric: string,
  range: MetricHistoryRange,
  aggregation: MetricHistoryAggregation,
  device?: string,
) => JSON.stringify([serverId, metric, range, aggregation, device ?? null]);

export const useStatisticsOverview = ({
  numericServerId,
  isValidServerId,
  metricSections,
}: UseStatisticsOverviewParams) => {
  const [nodeView, setNodeView] = React.useState<NodeView | null>(null);
  const [serverMissing, setServerMissing] = React.useState(false);
  const [ioDeviceOptions, setIoDeviceOptions] = React.useState<string[]>([]);
  const [ioDevice, setIoDevice] = React.useState('');
  const [mountOptions, setMountOptions] = React.useState<string[]>([]);
  const [mountDevice, setMountDevice] = React.useState('');
  const [range, setRange] = React.useState<MetricHistoryRange>('24h');
  const [seriesLookup, setSeriesLookup] = React.useState<SeriesLookup>({});
  const [visibleMetrics, setVisibleMetrics] = React.useState<string[]>([]);
  const [loadingMetrics, setLoadingMetrics] = React.useState<string[]>([]);
  const [failedMetrics, setFailedMetrics] = React.useState<string[]>([]);
  const [reloadTicks, setReloadTicks] = React.useState<Record<string, number>>({});
  const [stepSec, setStepSec] = React.useState<number | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const [errorKey, setErrorKey] = React.useState<TranslationKey | null>(null);
  const abortRef = React.useRef<AbortController | null>(null);
  const loadedRequestKeysRef = React.useRef<Map<string, Set<string>>>(new Map());
  const previousDevicesRef = React.useRef({ ioDevice: '', mountDevice: '' });

  const sections = React.useMemo(
    () => visibleMetricSections(metricSections, nodeView),
    [metricSections, nodeView],
  );
  const configBySeries = React.useMemo(() => configBySeriesKey(sections), [sections]);

  const visibleMetricsSet = React.useMemo(() => new Set(visibleMetrics), [visibleMetrics]);
  const loadingMetricsSet = React.useMemo(() => new Set(loadingMetrics), [loadingMetrics]);
  const failedMetricsSet = React.useMemo(() => new Set(failedMetrics), [failedMetrics]);
  const deviceSeriesKeys = React.useMemo(() => {
    const io = new Set<string>();
    const mount = new Set<string>();
    configBySeries.forEach((metric) => {
      const key = metricSeriesKey(metric);
      if (metric.deviceKey === 'io') io.add(key);
      if (metric.deviceKey === 'mount') mount.add(key);
    });
    return { io, mount };
  }, [configBySeries]);

  const clearMetricState = React.useCallback(
    (seriesKeys: Set<string>) => {
      if (seriesKeys.size === 0) return;

      seriesKeys.forEach((seriesKey) => loadedRequestKeysRef.current.delete(seriesKey));
      setSeriesLookup((prev) => {
        const next = { ...prev };
        seriesKeys.forEach((seriesKey) => {
          delete next[seriesKey];
        });
        return next;
      });
      setFailedMetrics((prev) => prev.filter((seriesKey) => !seriesKeys.has(seriesKey)));
      setLoadingMetrics((prev) => {
        const next = prev.filter((seriesKey) => !seriesKeys.has(seriesKey));
        seriesKeys.forEach((seriesKey) => {
          if (visibleMetricsSet.has(seriesKey)) next.push(seriesKey);
        });
        return Array.from(new Set(next));
      });
    },
    [visibleMetricsSet],
  );

  const showMetric = React.useCallback((seriesKey: string) => {
    setVisibleMetrics((prev) => (prev.includes(seriesKey) ? prev : [...prev, seriesKey]));
    setLoadingMetrics((prev) => (prev.includes(seriesKey) ? prev : [...prev, seriesKey]));
  }, []);

  const reloadMetric = React.useCallback((seriesKey: string) => {
    loadedRequestKeysRef.current.delete(seriesKey);
    setFailedMetrics((prev) => prev.filter((item) => item !== seriesKey));
    setVisibleMetrics((prev) => (prev.includes(seriesKey) ? prev : [...prev, seriesKey]));
    setLoadingMetrics((prev) => (prev.includes(seriesKey) ? prev : [...prev, seriesKey]));
    setReloadTicks((prev) => ({ ...prev, [seriesKey]: (prev[seriesKey] ?? 0) + 1 }));
  }, []);

  React.useEffect(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    loadedRequestKeysRef.current.clear();
    setSeriesLookup({});
    setLoadingMetrics([]);
    setFailedMetrics([]);
    setReloadTicks({});
    setStepSec(null);
    setIsLoading(false);
    setErrorKey(null);
  }, [numericServerId, range]);

  React.useEffect(() => {
    const previous = previousDevicesRef.current;
    if (previous.ioDevice !== ioDevice) {
      clearMetricState(deviceSeriesKeys.io);
    }
    if (previous.mountDevice !== mountDevice) {
      clearMetricState(deviceSeriesKeys.mount);
    }
    previousDevicesRef.current = { ioDevice, mountDevice };
  }, [clearMetricState, deviceSeriesKeys, ioDevice, mountDevice]);

  React.useEffect(() => {
    if (!isValidServerId) {
      setNodeView(null);
      setServerMissing(true);
      return;
    }

    const controller = new AbortController();
    setServerMissing(false);

    fetchFrontMetrics({ signal: controller.signal })
      .then((nodes) => {
        const found = nodes.find((node) => Number(node.node.id) === numericServerId) ?? null;
        setNodeView(found);
        setServerMissing(!found);
        const nextIoDeviceOptions = found?.disk?.io?.by_device
          ? Object.keys(found.disk.io.by_device)
          : [];
        const nextMountOptions = Array.from(
          new Set(found?.disk?.mounts?.map((mount) => mount.mountpoint).filter(Boolean) ?? []),
        );

        setIoDeviceOptions(nextIoDeviceOptions);
        setMountOptions(nextMountOptions);
        setIoDevice((current) =>
          current && nextIoDeviceOptions.includes(current)
            ? current
            : (nextIoDeviceOptions[0] ?? ''),
        );
        setMountDevice((current) =>
          current && nextMountOptions.includes(current) ? current : (nextMountOptions[0] ?? ''),
        );
      })
      .catch((error) => {
        if (isAbortError(error)) return;
        setNodeView(null);
        setServerMissing(true);
        setIoDeviceOptions([]);
        setMountOptions([]);
        setIoDevice('');
        setMountDevice('');
      });

    return () => controller.abort();
  }, [isValidServerId, numericServerId]);

  React.useEffect(() => {
    if (!isValidServerId) return;

    if (visibleMetricsSet.size === 0) {
      setIsLoading(false);
      setLoadingMetrics([]);
      return;
    }

    const controller = new AbortController();
    abortRef.current?.abort();
    abortRef.current = controller;

    const loadMetrics = async () => {
      const loadedRequests: Array<{ seriesKey: string; requestKey: string }> = [];
      let aborted = false;

      try {
        const configs = Array.from(configBySeries.values())
          .filter((metric) => {
            if (metric.historyAvailable === false) return false;
            if (!visibleMetricsSet.has(metricSeriesKey(metric))) return false;
            return metric.deviceKey
              ? Boolean(resolveMetricDevice(metric, ioDevice, mountDevice))
              : true;
          })
          .map((metric) => {
            const device = resolveMetricDevice(metric, ioDevice, mountDevice);
            const aggregation = metric.aggregation ?? DEFAULT_AGGREGATION;
            return {
              metric,
              seriesKey: metricSeriesKey(metric),
              device,
              aggregation,
              requestKey: historyKey(numericServerId, metric.metric, range, aggregation, device),
            };
          });

        if (configs.length === 0) {
          setIsLoading(false);
          setLoadingMetrics([]);
          return;
        }

        const pendingConfigs = configs.filter(
          ({ seriesKey, requestKey }) =>
            !loadedRequestKeysRef.current.get(seriesKey)?.has(requestKey),
        );

        if (pendingConfigs.length === 0) {
          setIsLoading(false);
          setLoadingMetrics([]);
          return;
        }

        const seriesKeysToLoad = Array.from(
          new Set(pendingConfigs.map(({ seriesKey }) => seriesKey)),
        );
        const hasGlobalPending = pendingConfigs.some(
          ({ metric }) => !metric.deviceKey && !metric.historyDevice,
        );
        setIsLoading(hasGlobalPending);
        setLoadingMetrics(seriesKeysToLoad);
        if (hasGlobalPending) {
          setErrorKey(null);
        }

        const results = await Promise.allSettled(
          pendingConfigs.map(async ({ metric, seriesKey, aggregation, device, requestKey }) => {
            const response = await fetchMetricHistory({
              serverId: numericServerId,
              metric: metric.metric,
              range,
              aggregation,
              device,
              signal: controller.signal,
            });
            return { metric, seriesKey, requestKey, response };
          }),
        );

        if (results.some((result) => result.status === 'rejected' && isAbortError(result.reason))) {
          aborted = true;
          return;
        }

        const nextSeries: SeriesLookup = {};
        const failedSeriesKeys: string[] = [];
        const loadedSeriesKeys = new Set<string>();
        let nextStepSec: number | null = null;

        results.forEach((result, index) => {
          if (result.status === 'rejected') {
            const failedConfig = pendingConfigs[index];
            if (failedConfig) {
              failedSeriesKeys.push(failedConfig.seriesKey);
              loadedRequests.push({
                seriesKey: failedConfig.seriesKey,
                requestKey: failedConfig.requestKey,
              });
            }
            return;
          }

          const { metric, seriesKey, requestKey, response } = result.value;
          if (nextStepSec == null) nextStepSec = response.step_sec;
          nextSeries[seriesKey] = metricPoints(response.points, metric.transform);
          loadedRequests.push({ seriesKey, requestKey });
          loadedSeriesKeys.add(seriesKey);
        });

        if (nextStepSec != null) {
          setStepSec(nextStepSec);
        }
        if (loadedRequests.length > 0) {
          setSeriesLookup((prev) => ({ ...prev, ...nextSeries }));
        }
        setFailedMetrics((prev) => {
          const next = new Set(prev);
          loadedSeriesKeys.forEach((seriesKey) => next.delete(seriesKey));
          failedSeriesKeys.forEach((seriesKey) => next.add(seriesKey));
          return Array.from(next);
        });
        if (failedSeriesKeys.length > 0) {
          const hasLoadedHistory =
            loadedRequests.length > 0 || hasLoadedRequests(loadedRequestKeysRef.current);
          const hasGlobalFailure = failedSeriesKeys.some((seriesKey) => {
            const metric = configBySeries.get(seriesKey);
            return !metric?.deviceKey && !metric?.historyDevice;
          });
          if (hasGlobalFailure) {
            setErrorKey(hasLoadedHistory ? 'stats_partial_error' : 'stats_error');
          }
        } else {
          let hasGlobalLoaded = false;
          loadedSeriesKeys.forEach((seriesKey) => {
            const metric = configBySeries.get(seriesKey);
            if (!metric?.deviceKey && !metric?.historyDevice) {
              hasGlobalLoaded = true;
            }
          });
          if (hasGlobalLoaded) {
            setErrorKey(null);
          }
        }
      } catch (error) {
        if (isAbortError(error)) {
          aborted = true;
          return;
        }
        setErrorKey('stats_error');
      } finally {
        if (loadedRequests.length > 0) {
          loadedRequests.forEach(({ seriesKey, requestKey }) => {
            const metricRequests = loadedRequestKeysRef.current.get(seriesKey) ?? new Set<string>();
            metricRequests.add(requestKey);
            loadedRequestKeysRef.current.set(seriesKey, metricRequests);
          });
        }
        if (!aborted) {
          setIsLoading(false);
          setLoadingMetrics([]);
        }
      }
    };

    void loadMetrics();

    return () => controller.abort();
  }, [
    ioDevice,
    isValidServerId,
    configBySeries,
    mountDevice,
    numericServerId,
    range,
    reloadTicks,
    visibleMetricsSet,
  ]);

  const lastUpdatedAt = React.useMemo(() => findLastUpdatedAt(seriesLookup), [seriesLookup]);
  const statsLookup = React.useMemo(
    () => collectStats(sections, seriesLookup),
    [sections, seriesLookup],
  );

  return {
    nodeView,
    metricSections: sections,
    serverMissing,
    ioDeviceOptions,
    ioDevice,
    setIoDevice,
    mountOptions,
    mountDevice,
    setMountDevice,
    range,
    setRange,
    seriesLookup,
    visibleMetricsSet,
    loadingMetricsSet,
    failedMetricsSet,
    stepSec,
    isLoading,
    errorKey,
    showMetric,
    reloadMetric,
    lastUpdatedAt,
    statsLookup,
  };
};

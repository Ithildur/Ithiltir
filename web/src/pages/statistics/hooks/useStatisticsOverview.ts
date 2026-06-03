import React from 'react';
import type { NodeView } from '@app-types/frontMetrics';
import type { TranslationKey } from '@i18n';
import { loadStatisticsNodeView } from '../nodeView';
import type { MetricSection } from '../config';
import { visibleMetricSections } from '../viewModel';
import { isCanceledRequestError } from '@utils/errors';
import type { MetricHistoryRange } from '@app-types/metricsHistory';
import { useMetricHistorySeries } from './useMetricHistorySeries';

type UseStatisticsOverviewParams = {
  numericServerId: number;
  isValidServerId: boolean;
  metricSections: MetricSection[];
};

export const useStatisticsOverview = ({
  numericServerId,
  isValidServerId,
  metricSections,
}: UseStatisticsOverviewParams) => {
  const [nodeView, setNodeView] = React.useState<NodeView | null>(null);
  const [serverMissing, setServerMissing] = React.useState(false);
  const [overviewErrorKey, setOverviewErrorKey] = React.useState<TranslationKey | null>(null);
  const [ioDeviceOptions, setIoDeviceOptions] = React.useState<string[]>([]);
  const [ioDevice, setIoDevice] = React.useState('');
  const [mountOptions, setMountOptions] = React.useState<string[]>([]);
  const [mountDevice, setMountDevice] = React.useState('');
  const [range, setRange] = React.useState<MetricHistoryRange>('24h');

  const sections = React.useMemo(
    () => visibleMetricSections(metricSections, nodeView),
    [metricSections, nodeView],
  );

  React.useEffect(() => {
    if (!isValidServerId) {
      setNodeView(null);
      setServerMissing(true);
      setOverviewErrorKey(null);
      return;
    }

    const controller = new AbortController();
    setServerMissing(false);
    setOverviewErrorKey(null);

    loadStatisticsNodeView(numericServerId, controller.signal)
      .then((found) => {
        setNodeView(found);
        setServerMissing(!found);
        setOverviewErrorKey(null);
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
        if (isCanceledRequestError(error)) return;
        setNodeView(null);
        setServerMissing(false);
        setOverviewErrorKey('stats_load_error');
        setIoDeviceOptions([]);
        setMountOptions([]);
        setIoDevice('');
        setMountDevice('');
      });

    return () => controller.abort();
  }, [isValidServerId, numericServerId]);

  const metricHistory = useMetricHistorySeries({
    numericServerId,
    isValidServerId,
    sections,
    range,
    ioDevice,
    mountDevice,
  });

  return {
    nodeView,
    metricSections: sections,
    serverMissing,
    overviewErrorKey,
    ioDeviceOptions,
    ioDevice,
    setIoDevice,
    mountOptions,
    mountDevice,
    setMountDevice,
    range,
    setRange,
    ...metricHistory,
  };
};

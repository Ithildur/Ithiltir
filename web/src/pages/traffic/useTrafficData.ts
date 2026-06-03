import React from 'react';
import type { TrafficDaily, TrafficMonthly, TrafficSummary } from '@app-types/traffic';
import { TRAFFIC_MONTHLY_HISTORY_MONTHS } from '@config/traffic';
import {
  fetchTrafficDaily,
  fetchTrafficIfaces,
  fetchTrafficMonthly,
  fetchTrafficSummary,
} from '@lib/statisticsApi';
import { isCanceledRequestError } from '@utils/errors';
import { createSeqGate } from '@utils/seqGate';
import type { TrafficDetailErrorKey } from './pageModel';

const emptyDaily: TrafficDaily = { items: [] };
const emptyMonthly: TrafficMonthly = { includes_current: true, items: [] };

export const useTrafficData = ({
  serverId,
  isValidServerId,
}: {
  serverId: number;
  isValidServerId: boolean;
}) => {
  const trafficGate = React.useMemo(() => createSeqGate(), []);
  const [ifaces, setIfaces] = React.useState<string[]>([]);
  const [iface, setIface] = React.useState('');
  const [summary, setSummary] = React.useState<TrafficSummary | null>(null);
  const [currentDaily, setCurrentDaily] = React.useState<TrafficDaily>(emptyDaily);
  const [previousDaily, setPreviousDaily] = React.useState<TrafficDaily>(emptyDaily);
  const [monthly, setMonthly] = React.useState<TrafficMonthly>(emptyMonthly);
  const [loading, setLoading] = React.useState(false);
  const [errorKey, setErrorKey] = React.useState<TrafficDetailErrorKey | null>(null);

  const resetTraffic = React.useCallback(() => {
    setLoading(false);
    setSummary(null);
    setCurrentDaily(emptyDaily);
    setPreviousDaily(emptyDaily);
    setMonthly(emptyMonthly);
    setErrorKey(null);
  }, []);

  React.useEffect(() => {
    trafficGate.invalidate();
    setIfaces([]);
    setIface('');
    resetTraffic();

    if (!isValidServerId) return undefined;

    const controller = new AbortController();
    void fetchTrafficIfaces({ serverId, signal: controller.signal })
      .then((items) => {
        const names = items
          .map((item) => item.name.trim())
          .filter((name) => name && name.toLowerCase() !== 'all');
        if (controller.signal.aborted) return;
        setIfaces(names);
        setIface((current) => (current && names.includes(current) ? current : (names[0] ?? '')));
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        if (!controller.signal.aborted) setErrorKey('traffic_error');
      });

    return () => {
      controller.abort();
    };
  }, [isValidServerId, resetTraffic, serverId, trafficGate]);

  const loadTraffic = React.useCallback(
    async (signal?: AbortSignal) => {
      const seq = trafficGate.next();
      if (!isValidServerId) return;

      if (!iface || !ifaces.includes(iface)) {
        resetTraffic();
        return;
      }

      setLoading(true);
      setErrorKey(null);
      try {
        const params = { serverId, iface, signal };
        const [nextSummary, nextMonthly] = await Promise.all([
          fetchTrafficSummary(params),
          fetchTrafficMonthly({ ...params, months: TRAFFIC_MONTHLY_HISTORY_MONTHS }),
        ]);

        let nextCurrentDaily: TrafficDaily = emptyDaily;
        let nextPreviousDaily: TrafficDaily = emptyDaily;
        if (nextSummary.usage_mode === 'billing') {
          [nextCurrentDaily, nextPreviousDaily] = await Promise.all([
            fetchTrafficDaily({ ...params, period: 'current' }),
            fetchTrafficDaily({ ...params, period: 'previous' }),
          ]);
        }

        if (signal?.aborted || !trafficGate.isCurrent(seq)) return;
        setSummary(nextSummary);
        setCurrentDaily(nextCurrentDaily);
        setPreviousDaily(nextPreviousDaily);
        setMonthly(nextMonthly);
      } catch (error) {
        if (isCanceledRequestError(error) || signal?.aborted || !trafficGate.isCurrent(seq)) return;
        setSummary(null);
        setCurrentDaily(emptyDaily);
        setPreviousDaily(emptyDaily);
        setMonthly(emptyMonthly);
        setErrorKey('traffic_error');
      } finally {
        if (!signal?.aborted && trafficGate.isCurrent(seq)) setLoading(false);
      }
    },
    [iface, ifaces, isValidServerId, resetTraffic, serverId, trafficGate],
  );

  React.useEffect(() => {
    const controller = new AbortController();
    void loadTraffic(controller.signal);
    return () => controller.abort();
  }, [loadTraffic]);

  return {
    ifaces,
    iface,
    setIface,
    summary,
    currentDaily,
    previousDaily,
    monthly,
    loading,
    errorKey,
    loadTraffic,
  };
};

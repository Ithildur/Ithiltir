import React from 'react';
import { useAuthStore } from '@stores/authStore';
import { useI18n } from '@i18n';
import { ApiError } from '@lib/api';
import { fetchFrontMetrics } from '@lib/frontApi';
import type { NodeView } from '@app-types/frontMetrics';
import { closeTopBanner, pushTopBanner } from '@runtime/topBannerRuntime';
import { normalizeNodeViews } from './viewModel';
import { DASHBOARD_POLL_INTERVAL_MS, DASHBOARD_REQUEST_TIMEOUT_MS } from '@config/dashboard';
import { isAbortError, isCanceledRequestError } from '@utils/errors';

export const useFrontMetricsPolling = (): {
  nodes: NodeView[];
  isLoading: boolean;
} => {
  const [rawNodes, setRawNodes] = React.useState<NodeView[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const { t } = useI18n();
  const status = useAuthStore((state) => state.status);
  const loadedRef = React.useRef(false);
  const previousStatusRef = React.useRef(status);

  const connectionRef = React.useRef<{ disconnected: boolean; bannerId: number | null }>({
    disconnected: false,
    bannerId: null,
  });

  const markDisconnected = React.useCallback(
    (error: unknown) => {
      if (connectionRef.current.disconnected) return;
      connectionRef.current.disconnected = true;

      const message =
        error instanceof ApiError && error.message
          ? `${t('dashboard_disconnected')}: ${error.message}`
          : t('dashboard_disconnected');

      const bannerId = pushTopBanner(message, { tone: 'error', durationMs: null });
      connectionRef.current.bannerId = bannerId;
    },
    [t],
  );

  const closeDisconnectedBanner = React.useCallback(() => {
    connectionRef.current.disconnected = false;

    const bannerId = connectionRef.current.bannerId;
    connectionRef.current.bannerId = null;
    if (bannerId != null) {
      closeTopBanner(bannerId);
    }
  }, []);

  const markRecovered = React.useCallback(() => {
    if (!connectionRef.current.disconnected) return;
    closeDisconnectedBanner();
    pushTopBanner(t('dashboard_recovered'), { tone: 'info' });
  }, [closeDisconnectedBanner, t]);

  React.useEffect(() => {
    const previousStatus = previousStatusRef.current;
    previousStatusRef.current = status;
    if (previousStatus !== 'authenticated' || status === 'authenticated') return;

    loadedRef.current = false;
    setRawNodes([]);
    setIsLoading(true);
    closeDisconnectedBanner();
  }, [closeDisconnectedBanner, status]);

  React.useEffect(() => {
    if (status === 'unknown' || status === 'bootstrapping') return;

    let isMounted = true;
    const abortRef: { current: AbortController | null } = { current: null };
    let timerId: number | null = null;
    let loopId: number | null = null;

    const loadOnce = async () => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      let timedOut = false;
      const timeoutId = window.setTimeout(() => {
        timedOut = true;
        controller.abort();
      }, DASHBOARD_REQUEST_TIMEOUT_MS);
      if (!loadedRef.current) {
        setIsLoading(true);
      }
      try {
        const nodes = await fetchFrontMetrics({ signal: controller.signal });
        if (!isMounted) return;
        loadedRef.current = true;
        setRawNodes(nodes);
        setIsLoading(false);
        markRecovered();
      } catch (error) {
        if (!isMounted) return;
        loadedRef.current = true;
        setIsLoading(false);
        if (isCanceledRequestError(error) && !(timedOut && isAbortError(error))) return;
        markDisconnected(error);
      } finally {
        window.clearTimeout(timeoutId);
      }
    };

    const loop = async () => {
      await loadOnce();
      if (!isMounted) return;
      loopId = window.setTimeout(loop, DASHBOARD_POLL_INTERVAL_MS);
    };

    timerId = window.setTimeout(loop, 0);
    return () => {
      isMounted = false;
      if (timerId !== null) window.clearTimeout(timerId);
      if (loopId !== null) window.clearTimeout(loopId);
      abortRef.current?.abort();
      closeDisconnectedBanner();
    };
  }, [closeDisconnectedBanner, markDisconnected, markRecovered, status]);

  const nodes = React.useMemo(() => normalizeNodeViews(rawNodes), [rawNodes]);
  return { nodes, isLoading };
};

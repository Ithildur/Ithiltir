import React from 'react';
import { useAuth } from '@context/AuthContext';
import { useI18n } from '@i18n';
import { ApiError } from '@lib/api';
import { fetchFrontMetrics } from '@lib/frontApi';
import type { NodeView } from '@app-types/frontMetrics';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { normalizeNodeViews } from './viewModel';
import { DASHBOARD_POLL_INTERVAL_MS, DASHBOARD_REQUEST_TIMEOUT_MS } from '@config/dashboard';

export const useFrontMetricsPolling = (): {
  nodes: NodeView[];
  isLoading: boolean;
} => {
  const [nodes, setNodes] = React.useState<NodeView[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const { bootstrap, status } = useAuth();

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

      const bannerId = pushBanner(message, { tone: 'error', durationMs: null });
      connectionRef.current.bannerId = bannerId;
    },
    [pushBanner, t],
  );

  const closeDisconnectedBanner = React.useCallback(() => {
    connectionRef.current.disconnected = false;

    const bannerId = connectionRef.current.bannerId;
    connectionRef.current.bannerId = null;
    if (bannerId != null) {
      pushBanner.close(bannerId);
    }
  }, [pushBanner]);

  const markRecovered = React.useCallback(() => {
    if (!connectionRef.current.disconnected) return;
    closeDisconnectedBanner();
    pushBanner(t('dashboard_recovered'), { tone: 'info' });
  }, [closeDisconnectedBanner, pushBanner, t]);

  React.useEffect(() => {
    let isMounted = true;
    const abortRef: { current: AbortController | null } = { current: null };
    let timerId: number | null = null;
    let loopId: number | null = null;

    const ensureAuthReady = async (): Promise<void> => {
      if (status === 'authenticated' || status === 'guest') return;
      await bootstrap();
    };

    const loadOnce = async () => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      const timeoutId = window.setTimeout(() => controller.abort(), DASHBOARD_REQUEST_TIMEOUT_MS);
      try {
        await ensureAuthReady();
        const data = await fetchFrontMetrics({ signal: controller.signal });
        if (!isMounted) return;
        setNodes(normalizeNodeViews(data));
        setIsLoading(false);
        markRecovered();
      } catch (error) {
        if (!isMounted) return;
        markDisconnected(error);
        setIsLoading(false);
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
  }, [bootstrap, closeDisconnectedBanner, markDisconnected, markRecovered, status]);

  return { nodes, isLoading };
};

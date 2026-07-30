import React from 'react';
import { useLocation } from 'react-router';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useAuthStore } from '@stores/authStore';
import { useI18n } from '@i18n';
import {
  isTrafficRebuildBusy,
  resetTrafficRebuild,
  syncTrafficRebuildStatus,
  useTrafficRebuildStore,
} from '@stores/trafficRebuildStore';
import { isCanceledRequestError } from '@utils/errors';

const activePollMs = 3000;

export const TrafficRebuildRuntime: React.FC = () => {
  const location = useLocation();
  const token = useAuthStore((state) => state.accessToken);
  const { t } = useI18n();
  const status = useTrafficRebuildStore((state) => state.status);
  const startingNodeId = useTrafficRebuildStore((state) => state.startingNodeId);
  const watchedNodeRef = React.useRef<number | null>(null);
  const syncErrorShownRef = React.useRef(false);
  const active = status.running || startingNodeId !== null;
  const syncKey = `${location.pathname}${location.search}`;

  React.useEffect(() => {
    if (status.running && status.server_id > 0) {
      watchedNodeRef.current = status.server_id;
      return;
    }
    const watchedNodeId = watchedNodeRef.current;
    if (
      watchedNodeId === null ||
      status.server_id !== watchedNodeId ||
      (status.status !== 'completed' && status.status !== 'failed')
    ) {
      return;
    }

    watchedNodeRef.current = null;
    pushTopBanner(
      t(
        status.status === 'completed'
          ? 'admin_node_traffic_rebuild_completed'
          : 'admin_node_traffic_rebuild_failed',
      ),
      { tone: status.status === 'completed' ? 'info' : 'error' },
    );
  }, [status, t]);

  const syncStatus = React.useCallback(
    async (signal: AbortSignal) => {
      try {
        await syncTrafficRebuildStatus(signal);
        syncErrorShownRef.current = false;
      } catch (error) {
        if (isCanceledRequestError(error) || !isTrafficRebuildBusy()) return;
        if (!syncErrorShownRef.current) {
          syncErrorShownRef.current = true;
          pushTopBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
        }
      }
    },
    [t],
  );

  React.useEffect(() => {
    if (!token) {
      watchedNodeRef.current = null;
      syncErrorShownRef.current = false;
      resetTrafficRebuild();
      return;
    }

    const controller = new AbortController();
    void syncStatus(controller.signal);
    return () => controller.abort();
  }, [syncKey, syncStatus, token]);

  React.useEffect(() => {
    if (!token || !active) return;

    const controller = new AbortController();
    let timer: number | undefined;
    const poll = async () => {
      await syncStatus(controller.signal);
      if (!controller.signal.aborted && isTrafficRebuildBusy()) {
        timer = window.setTimeout(() => void poll(), activePollMs);
      }
    };

    timer = window.setTimeout(() => void poll(), activePollMs);
    return () => {
      controller.abort();
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [active, syncStatus, token]);

  return null;
};

import React from 'react';
import { useLocation } from 'react-router-dom';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useAuthStore } from '@stores/authStore';
import { useI18n } from '@i18n';
import {
  isTrafficRebuildBusy,
  reportTrafficRebuildSyncError,
  resetTrafficRebuild,
  syncTrafficRebuildStatus,
  takeTrafficRebuildEvent,
  useTrafficRebuildStore,
} from '@stores/trafficRebuildStore';
import { isCanceledRequestError } from '@utils/errors';

const activePollMs = 3000;

export const TrafficRebuildRuntime: React.FC = () => {
  const location = useLocation();
  const token = useAuthStore((state) => state.accessToken);
  const { t } = useI18n();
  const event = useTrafficRebuildStore((state) => state.event);
  const active = useTrafficRebuildStore(
    (state) => state.status.running || state.local.phase !== 'idle',
  );
  const syncKey = `${location.pathname}${location.search}`;

  React.useEffect(() => {
    if (!event) return;
    const claimed = takeTrafficRebuildEvent(event.id);
    if (!claimed) return;

    if (claimed.kind === 'completed') {
      pushTopBanner(t('admin_node_traffic_rebuild_completed'), { tone: 'info' });
    } else if (claimed.kind === 'failed') {
      pushTopBanner(t('admin_node_traffic_rebuild_failed'), { tone: 'error' });
    } else {
      pushTopBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
    }
  }, [event, t]);

  React.useEffect(() => {
    if (!token) {
      resetTrafficRebuild();
      return;
    }

    const controller = new AbortController();

    const sync = async () => {
      try {
        await syncTrafficRebuildStatus(controller.signal);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        if (isTrafficRebuildBusy()) {
          reportTrafficRebuildSyncError();
        }
      }
    };

    void sync();
    return () => {
      controller.abort();
    };
  }, [syncKey, token]);

  React.useEffect(() => {
    if (!token || !active) return;

    const controller = new AbortController();
    let timer: number | undefined;

    const poll = async () => {
      try {
        await syncTrafficRebuildStatus(controller.signal);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        if (isTrafficRebuildBusy()) {
          reportTrafficRebuildSyncError();
        }
      } finally {
        if (!controller.signal.aborted && isTrafficRebuildBusy()) {
          timer = window.setTimeout(() => void poll(), activePollMs);
        }
      }
    };

    timer = window.setTimeout(() => void poll(), activePollMs);
    return () => {
      controller.abort();
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [active, token]);

  return null;
};

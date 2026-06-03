import React from 'react';
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
const idlePollMs = 15000;

export const TrafficRebuildRuntime: React.FC = () => {
  const token = useAuthStore((state) => state.accessToken);
  const { t } = useI18n();
  const event = useTrafficRebuildStore((state) => state.event);

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
    let timer: number | undefined;

    const poll = async () => {
      let delay = idlePollMs;
      const wasBusy = isTrafficRebuildBusy();
      try {
        const next = await syncTrafficRebuildStatus(controller.signal);
        if (controller.signal.aborted) return;
        delay = next.running || isTrafficRebuildBusy() ? activePollMs : idlePollMs;
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        if (wasBusy || isTrafficRebuildBusy()) {
          reportTrafficRebuildSyncError();
          delay = activePollMs;
        } else {
          delay = idlePollMs;
        }
      } finally {
        if (!controller.signal.aborted) {
          timer = window.setTimeout(() => void poll(), delay);
        }
      }
    };

    void poll();
    return () => {
      controller.abort();
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [token]);

  return null;
};

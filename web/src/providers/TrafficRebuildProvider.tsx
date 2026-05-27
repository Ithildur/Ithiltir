import React from 'react';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useAuth } from '@context/AuthContext';
import { useI18n } from '@i18n';
import {
  isTrafficRebuildBusy,
  isAbortError,
  useTrafficRebuildStore,
} from '../stores/trafficRebuildStore';

const activePollMs = 3000;
const idlePollMs = 15000;

export const TrafficRebuildProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { token } = useAuth();
  const pushBanner = useTopBanner();
  const { t } = useI18n();
  const refresh = useTrafficRebuildStore((state) => state.refresh);
  const markSyncError = useTrafficRebuildStore((state) => state.markSyncError);
  const reset = useTrafficRebuildStore((state) => state.reset);
  const event = useTrafficRebuildStore((state) => state.event);
  const takeEvent = useTrafficRebuildStore((state) => state.takeEvent);

  React.useEffect(() => {
    if (!event) return;
    const claimed = takeEvent(event.id);
    if (!claimed) return;

    if (claimed.type === 'completed') {
      pushBanner(t('admin_node_traffic_rebuild_completed'), { tone: 'info' });
    } else if (claimed.type === 'failed') {
      pushBanner(t('admin_node_traffic_rebuild_failed'), { tone: 'error' });
    } else {
      pushBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
    }
  }, [event, pushBanner, t, takeEvent]);

  React.useEffect(() => {
    if (!token) {
      reset();
      return;
    }

    const controller = new AbortController();
    let timer: number | undefined;

    const poll = async () => {
      let delay = idlePollMs;
      const wasBusy = isTrafficRebuildBusy();
      try {
        const next = await refresh(controller.signal);
        if (controller.signal.aborted) return;
        delay = next.running || isTrafficRebuildBusy() ? activePollMs : idlePollMs;
      } catch (error) {
        if (isAbortError(error)) return;
        if (wasBusy || isTrafficRebuildBusy()) {
          markSyncError(error);
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
  }, [markSyncError, refresh, reset, token]);

  return <>{children}</>;
};

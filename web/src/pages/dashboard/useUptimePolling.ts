import React from 'react';
import { useAuthStore } from '@stores/authStore';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { fetchUptime } from '@lib/uptimeApi';
import { isCanceledRequestError } from '@utils/errors';
import type { UptimeHistory } from './viewModel';

export const useUptimePolling = (): Record<string, UptimeHistory> | null => {
  const status = useAuthStore((state) => state.status);
  const apiError = useApiErrorHandler();
  const [snapshot, setSnapshot] = React.useState<{
    status: typeof status;
    nodes: Record<string, UptimeHistory>;
  } | null>(null);

  React.useEffect(() => {
    if (status === 'unknown' || status === 'bootstrapping') return;
    const controller = new AbortController();
    let timer: number | undefined;
    let failed = false;
    const refresh = async () => {
      try {
        const uptime = await fetchUptime(controller.signal);
        if (controller.signal.aborted) return;
        const nodes: Record<string, UptimeHistory> = {};
        if (uptime.enabled) {
          for (const node of uptime.nodes) {
            nodes[node.server_id] = {
              days: node.days,
              warningSLA: uptime.warning_sla,
              errorSLA: uptime.error_sla,
              asOf: uptime.generated_at,
            };
          }
        }
        setSnapshot({ status, nodes });
        failed = false;
      } catch (error) {
        if (controller.signal.aborted || isCanceledRequestError(error)) return;
        // Never keep data visible after a failed permissions refresh.
        setSnapshot(null);
        if (!failed) apiError(error, { key: 'dashboard_uptime_fetch_failed' });
        failed = true;
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(refresh, 60_000);
      }
    };
    void refresh();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [apiError, status]);

  return snapshot?.status === status ? snapshot.nodes : null;
};

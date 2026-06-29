import React from 'react';
import type { AlertEventSummary } from '@app-types/admin';
import { fetchAlertEventSummary } from '@lib/adminApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { isCanceledRequestError } from '@utils/errors';

interface OpenAlertSummaryState {
  summaryByServer: ReadonlyMap<number, AlertEventSummary>;
  loaded: boolean;
}

export const useOpenAlertSummary = (enabled = true): OpenAlertSummaryState => {
  const apiError = useApiErrorHandler();
  const [items, setItems] = React.useState<AlertEventSummary[]>([]);
  const [loaded, setLoaded] = React.useState(false);

  React.useEffect(() => {
    if (!enabled) {
      setItems([]);
      setLoaded(false);
      return;
    }
    const controller = new AbortController();
    setItems([]);
    setLoaded(false);
    fetchAlertEventSummary({ signal: controller.signal })
      .then((res) => {
        if (controller.signal.aborted) return;
        setItems(res.items);
        setLoaded(true);
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        setItems([]);
        setLoaded(false);
        apiError(error, { key: 'admin_alerts_summary_fetch_failed' });
      });
    return () => {
      controller.abort();
    };
  }, [apiError, enabled]);

  const summaryByServer = React.useMemo(() => {
    return new Map(items.map((item) => [item.server_id, item]));
  }, [items]);

  return { summaryByServer, loaded };
};

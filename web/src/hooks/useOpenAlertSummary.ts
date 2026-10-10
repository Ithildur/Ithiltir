import React from 'react';
import type { AlertEventSummary } from '@app-types/admin';
import { fetchAlertEventSummary } from '@lib/adminApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { isCanceledRequestError } from '@utils/errors';

interface OpenAlertSummaryState {
  summaryByServer: ReadonlyMap<number, AlertEventSummary>;
  loaded: boolean;
}

const emptySummary: OpenAlertSummaryState = { summaryByServer: new Map(), loaded: false };

export const useOpenAlertSummary = (enabled = true): OpenAlertSummaryState => {
  const apiError = useApiErrorHandler();
  const [summary, setSummary] = React.useState(emptySummary);

  React.useEffect(() => {
    if (!enabled) {
      setSummary(emptySummary);
      return;
    }
    const controller = new AbortController();
    setSummary(emptySummary);
    fetchAlertEventSummary({ signal: controller.signal })
      .then((res) => {
        if (controller.signal.aborted) return;
        setSummary({
          summaryByServer: new Map(res.items.map((item) => [item.server_id, item])),
          loaded: true,
        });
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        setSummary(emptySummary);
        apiError(error, { key: 'admin_alerts_summary_fetch_failed' });
      });
    return () => {
      controller.abort();
    };
  }, [apiError, enabled]);

  return summary;
};

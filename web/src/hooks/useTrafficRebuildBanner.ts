import React from 'react';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useI18n } from '@i18n';
import type { TrafficRebuildStartOutcome } from './useTrafficRebuild';

export const useTrafficRebuildBanner = () => {
  const apiError = useApiErrorHandler();
  const { t } = useI18n();

  return React.useCallback(
    (outcome: TrafficRebuildStartOutcome) => {
      if (outcome.status === 'started') {
        pushTopBanner(t('admin_node_traffic_rebuild_started'), { tone: 'info' });
      } else if (outcome.status === 'running_other') {
        pushTopBanner(t('traffic_rebuild_running_other'), { tone: 'error' });
      } else if (outcome.status === 'sync_failed') {
        pushTopBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
      } else if (outcome.status === 'failed') {
        apiError(outcome.error, t('admin_node_traffic_rebuild_failed'));
      }
    },
    [apiError, t],
  );
};

import React from 'react';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useI18n } from '@i18n';
import type { TrafficRebuildStartOutcome } from './useTrafficRebuild';

export const useTrafficRebuildBanner = () => {
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const { t } = useI18n();

  return React.useCallback(
    (outcome: TrafficRebuildStartOutcome | null) => {
      if (!outcome) return;
      if (outcome.status === 'started') {
        pushBanner(t('admin_node_traffic_rebuild_started'), { tone: 'info' });
      } else if (outcome.status === 'running_other') {
        pushBanner(t('traffic_rebuild_running_other'), { tone: 'error' });
      } else if (outcome.status === 'sync_failed') {
        pushBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
      } else {
        apiError(outcome.error, t('admin_node_traffic_rebuild_failed'));
      }
    },
    [apiError, pushBanner, t],
  );
};

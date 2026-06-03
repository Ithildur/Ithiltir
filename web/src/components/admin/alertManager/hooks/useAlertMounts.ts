import React from 'react';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { loadAlertMounts, setAlertMounts, useAlertMountsStore } from '@stores/alertMountsStore';
import { isCanceledRequestError } from '@utils/errors';

export const useAlertMounts = ({ enabled }: { enabled: boolean }) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const data = useAlertMountsStore((state) => state.data);
  const loading = useAlertMountsStore((state) => state.loading);
  const saving = useAlertMountsStore((state) => state.saving);

  const fetchMounts = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      try {
        await loadAlertMounts(params);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_mounts_fetch_failed' });
      }
    },
    [apiError],
  );

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void fetchMounts({ signal: controller.signal });
    return () => {
      controller.abort();
    };
  }, [enabled, fetchMounts]);

  const setMounts = React.useCallback(
    async (ruleIds: number[], serverIds: number[], mounted: boolean) => {
      try {
        const didUpdate = await setAlertMounts(ruleIds, serverIds, mounted);
        if (!didUpdate) return false;
        pushTopBanner(
          mounted
            ? t('admin_alerts_mounts_apply_success')
            : t('admin_alerts_mounts_cancel_success'),
          { tone: 'info' },
        );
        return true;
      } catch (error) {
        apiError(error, t('admin_alerts_mounts_update_failed'));
        return false;
      }
    },
    [apiError, t],
  );

  return {
    ...data,
    loading,
    saving,
    setMounts,
    refresh: fetchMounts,
  };
};

import React from 'react';
import { useI18n } from '@i18n';
import type { AlertMounts } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';

export const useAlertMounts = ({ enabled }: { enabled: boolean }) => {
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const apiError = useApiErrorHandler();
  const [data, setData] = React.useState<AlertMounts>({ rules: [], nodes: [] });
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  const fetchMounts = React.useCallback(async () => {
    setLoading(true);
    try {
      setData(await adminApi.fetchAlertMounts());
    } catch (error) {
      apiError(error, t('admin_alerts_mounts_fetch_failed'));
    } finally {
      setLoading(false);
    }
  }, [apiError, t]);

  React.useEffect(() => {
    if (!enabled) return;
    void fetchMounts();
  }, [enabled, fetchMounts]);

  const setMounts = React.useCallback(
    async (ruleIds: number[], serverIds: number[], mounted: boolean) => {
      if (saving || ruleIds.length === 0 || serverIds.length === 0) return false;
      try {
        setSaving(true);
        await adminApi.updateAlertMounts({
          rule_ids: ruleIds,
          server_ids: serverIds,
          mounted,
        });
        await fetchMounts();
        pushBanner(
          mounted
            ? t('admin_alerts_mounts_apply_success')
            : t('admin_alerts_mounts_cancel_success'),
          { tone: 'info' },
        );
        return true;
      } catch (error) {
        apiError(error, t('admin_alerts_mounts_update_failed'));
        return false;
      } finally {
        setSaving(false);
      }
    },
    [apiError, fetchMounts, pushBanner, saving, t],
  );

  return {
    ...data,
    loading,
    saving,
    setMounts,
    refresh: fetchMounts,
  };
};

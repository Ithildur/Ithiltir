import React from 'react';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import IOSSwitch from '@components/ui/IOSSwitch';
import Select from '@components/ui/Select';
import SettingRow, { SettingPanel } from '@components/admin/systemManager/SettingRow';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import {
  trafficDirectionModes,
  trafficDirectionLabelKey,
  parseTrafficDirectionMode,
} from '@lib/trafficSettingsModel';
import type { TrafficDirectionMode, TrafficUsageMode } from '@app-types/traffic';
import { useI18n } from '@i18n';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import {
  loadTrafficSettings,
  patchTrafficSettings,
  useTrafficSettingsStore,
} from '@stores/trafficSettingsStore';
import { isCanceledRequestError } from '@utils/errors';

const TrafficSettings: React.FC = () => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const { dialogProps: confirmDialogProps, request: requestConfirm } = useConfirmDialog();
  const trafficSettings = useTrafficSettingsStore((state) => state.settings);
  const loading = useTrafficSettingsStore((state) => state.loading);
  const savingMode = useTrafficSettingsStore((state) => state.savingUsageMode);
  const savingGuestAccess = useTrafficSettingsStore((state) => state.savingGuestAccess);
  const savingDirection = useTrafficSettingsStore((state) => state.savingDirection);

  const load = React.useCallback(
    async (signal: AbortSignal) => {
      try {
        await loadTrafficSettings({ signal });
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_traffic_fetch_failed' });
      }
    },
    [apiError],
  );

  React.useEffect(() => {
    const controller = new AbortController();

    void load(controller.signal);

    return () => {
      controller.abort();
    };
  }, [load]);

  const saveUsageMode = React.useCallback(
    async (mode: TrafficUsageMode) => {
      if (mode === trafficSettings.usage_mode || savingMode) return;
      const enableBilling = mode === 'billing';
      const ok = await requestConfirm({
        title: t(
          enableBilling
            ? 'admin_system_traffic_usage_confirm_billing_title'
            : 'admin_system_traffic_usage_confirm_lite_title',
        ),
        message: t(
          enableBilling
            ? 'admin_system_traffic_usage_confirm_billing_message'
            : 'admin_system_traffic_usage_confirm_lite_message',
        ),
        confirmLabel: t(
          enableBilling
            ? 'admin_system_traffic_usage_confirm_billing_action'
            : 'admin_system_traffic_usage_confirm_lite_action',
        ),
        cancelLabel: t('common_cancel'),
        tone: enableBilling ? 'default' : 'danger',
      });
      if (!ok) return;

      try {
        const didSave = await patchTrafficSettings({ usage_mode: mode }, { kind: 'usageMode' });
        if (!didSave) return;
        pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_system_settings_save_failed'));
      }
    },
    [apiError, requestConfirm, savingMode, t, trafficSettings.usage_mode],
  );

  const saveGuestAccess = React.useCallback(async () => {
    if (loading || savingGuestAccess) return;
    const nextMode = trafficSettings.guest_access_mode === 'by_node' ? 'disabled' : 'by_node';
    try {
      const didSave = await patchTrafficSettings(
        { guest_access_mode: nextMode },
        { kind: 'guestAccess' },
      );
      if (!didSave) return;
      pushTopBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
    } catch (error) {
      apiError(error, { key: 'traffic_settings_save_failed' });
    }
  }, [apiError, loading, savingGuestAccess, t, trafficSettings.guest_access_mode]);

  const saveDirectionMode = React.useCallback(
    async (mode: TrafficDirectionMode) => {
      if (loading || savingDirection || mode === trafficSettings.direction_mode) return;
      try {
        const didSave = await patchTrafficSettings({ direction_mode: mode }, { kind: 'direction' });
        if (!didSave) return;
        pushTopBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, { key: 'traffic_settings_save_failed' });
      }
    },
    [apiError, loading, savingDirection, t, trafficSettings.direction_mode],
  );

  return (
    <section>
      <ConfirmDialog {...confirmDialogProps} />

      <SettingPanel title={t('admin_traffic_settings_title')}>
        <SettingRow
          title={t('admin_system_traffic_usage_mode')}
          description={t('admin_system_traffic_usage_mode_desc')}
        >
          <IOSSwitch
            checked={trafficSettings.usage_mode === 'billing'}
            disabled={loading || savingMode}
            ariaLabel={t('admin_system_traffic_usage_mode')}
            onChange={() =>
              void saveUsageMode(trafficSettings.usage_mode === 'billing' ? 'lite' : 'billing')
            }
          />
        </SettingRow>

        <SettingRow title={t('traffic_guest_access')} description={t('traffic_guest_access_desc')}>
          <IOSSwitch
            checked={trafficSettings.guest_access_mode === 'by_node'}
            disabled={loading || savingGuestAccess}
            ariaLabel={t('traffic_guest_access')}
            onChange={() => void saveGuestAccess()}
          />
        </SettingRow>

        <SettingRow
          title={t('traffic_direction_mode')}
          description={t('traffic_direction_mode_desc')}
        >
          <Select
            value={trafficSettings.direction_mode}
            disabled={loading || savingDirection}
            aria-label={t('traffic_direction_mode')}
            width="auto"
            className="min-w-44"
            onChange={(event) => {
              const mode = parseTrafficDirectionMode(event.target.value);
              if (mode) void saveDirectionMode(mode);
            }}
          >
            {trafficDirectionModes.map((mode) => (
              <option key={mode} value={mode}>
                {t(trafficDirectionLabelKey[mode])}
              </option>
            ))}
          </Select>
        </SettingRow>
      </SettingPanel>
    </section>
  );
};

export default TrafficSettings;

import React from 'react';
import Network from 'lucide-react/dist/esm/icons/network';
import Save from 'lucide-react/dist/esm/icons/save';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import Select from '@components/ui/Select';
import TimezoneSelect from '@components/ui/TimezoneSelect';
import SettingRow from '@components/admin/systemManager/SettingRow';
import { useTopBanner } from '@components/ui/TopBannerStack';
import {
  billingDayFromAnchor,
  clampBillingDay,
  cycleNeedsAnchorDate,
  cycleNeedsBillingStartDay,
  cycleNeedsTimezone,
  defaultTrafficSettings,
  normalizeTrafficCycleFields,
  trafficCycleModes,
  trafficCycleChanged,
  trafficCyclePatchFromFields,
  trafficCycleValid,
  trafficDirectionModes,
  trafficSettingsWithCycleMode,
} from '@lib/trafficSettingsModel';
import type {
  TrafficCycleMode,
  TrafficDirectionMode,
  TrafficSettings as TrafficSettingsView,
  TrafficUsageMode,
} from '@app-types/traffic';
import { useI18n, type TranslationKey } from '@i18n';
import { fetchTrafficSettings, updateTrafficSettings } from '@lib/statisticsApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';

const TrafficSettings: React.FC = () => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const confirmDialog = useConfirmDialog();
  const [trafficSettings, setTrafficSettings] =
    React.useState<TrafficSettingsView>(defaultTrafficSettings);
  const [draft, setDraft] = React.useState<TrafficSettingsView>(defaultTrafficSettings);
  const [loading, setLoading] = React.useState(false);
  const [savingSettings, setSavingSettings] = React.useState(false);
  const [savingMode, setSavingMode] = React.useState(false);
  const [savingGuestAccess, setSavingGuestAccess] = React.useState(false);
  const [savingDirection, setSavingDirection] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const nextTraffic = await fetchTrafficSettings();
      setTrafficSettings(nextTraffic);
      setDraft(nextTraffic);
    } catch (error) {
      apiError(error, t('admin_traffic_fetch_failed'));
    } finally {
      setLoading(false);
    }
  }, [apiError, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const saveUsageMode = React.useCallback(
    async (mode: TrafficUsageMode) => {
      if (mode === trafficSettings.usage_mode || savingMode) return;
      const enableBilling = mode === 'billing';
      const ok = await confirmDialog.request({
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

      const previous = trafficSettings.usage_mode;
      setTrafficSettings((current) => ({ ...current, usage_mode: mode }));
      setDraft((current) => ({ ...current, usage_mode: mode }));
      setSavingMode(true);
      try {
        await updateTrafficSettings({ usage_mode: mode });
        pushBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        setTrafficSettings((current) => ({ ...current, usage_mode: previous }));
        setDraft((current) => ({ ...current, usage_mode: previous }));
        apiError(error, t('admin_system_settings_save_failed'));
      } finally {
        setSavingMode(false);
      }
    },
    [apiError, confirmDialog, pushBanner, savingMode, t, trafficSettings.usage_mode],
  );

  const saveTrafficSettings = React.useCallback(async () => {
    if (savingSettings || !trafficCycleChanged(draft, trafficSettings) || !trafficCycleValid(draft))
      return;
    setSavingSettings(true);
    try {
      const next = normalizeTrafficCycleFields(draft);
      await updateTrafficSettings(trafficCyclePatchFromFields(draft));
      setTrafficSettings((current) => ({ ...current, ...next }));
      setDraft((current) => ({ ...current, ...next }));
      pushBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
    } catch (error) {
      apiError(error, { key: 'traffic_settings_save_failed' });
    } finally {
      setSavingSettings(false);
    }
  }, [apiError, draft, pushBanner, savingSettings, t, trafficSettings]);

  const saveGuestAccess = React.useCallback(async () => {
    if (loading || savingGuestAccess) return;
    const nextMode = draft.guest_access_mode === 'by_node' ? 'disabled' : 'by_node';
    const previous = trafficSettings.guest_access_mode;
    setTrafficSettings((current) => ({ ...current, guest_access_mode: nextMode }));
    setDraft((current) => ({ ...current, guest_access_mode: nextMode }));
    setSavingGuestAccess(true);
    try {
      await updateTrafficSettings({ guest_access_mode: nextMode });
      pushBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
    } catch (error) {
      setTrafficSettings((current) => ({ ...current, guest_access_mode: previous }));
      setDraft((current) => ({ ...current, guest_access_mode: previous }));
      apiError(error, { key: 'traffic_settings_save_failed' });
    } finally {
      setSavingGuestAccess(false);
    }
  }, [
    apiError,
    draft.guest_access_mode,
    loading,
    pushBanner,
    savingGuestAccess,
    t,
    trafficSettings.guest_access_mode,
  ]);

  const saveDirectionMode = React.useCallback(
    async (mode: TrafficDirectionMode) => {
      if (loading || savingDirection || mode === trafficSettings.direction_mode) return;
      const previous = trafficSettings.direction_mode;
      setTrafficSettings((current) => ({ ...current, direction_mode: mode }));
      setDraft((current) => ({ ...current, direction_mode: mode }));
      setSavingDirection(true);
      try {
        await updateTrafficSettings({ direction_mode: mode });
        pushBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
      } catch (error) {
        setTrafficSettings((current) => ({ ...current, direction_mode: previous }));
        setDraft((current) => ({ ...current, direction_mode: previous }));
        apiError(error, { key: 'traffic_settings_save_failed' });
      } finally {
        setSavingDirection(false);
      }
    },
    [apiError, loading, pushBanner, savingDirection, t, trafficSettings.direction_mode],
  );

  const setCycleMode = React.useCallback((mode: TrafficCycleMode) => {
    setDraft((current) => trafficSettingsWithCycleMode(current, mode));
  }, []);

  const changed = trafficCycleChanged(draft, trafficSettings);
  const cycleValid = trafficCycleValid(draft);
  const showBillingStartDay = cycleNeedsBillingStartDay(draft.cycle_mode);
  const showAnchorDate = cycleNeedsAnchorDate(draft.cycle_mode);
  const showTimezone = cycleNeedsTimezone(draft.cycle_mode);

  return (
    <section className="space-y-4">
      <ConfirmDialog {...confirmDialog.dialogProps} />

      <div className="px-1">
        <div className="flex items-center gap-2 text-sm font-semibold text-(--theme-fg-default)">
          <Network className="size-4 text-(--theme-fg-muted)" aria-hidden="true" />
          {t('admin_traffic_settings_title')}
        </div>
      </div>

      <div className="space-y-4">
        <SettingRow
          title={t('admin_system_traffic_usage_mode')}
          description={t('admin_system_traffic_usage_mode_desc')}
        >
          <IOSSwitch
            checked={draft.usage_mode === 'billing'}
            disabled={loading || savingMode}
            ariaLabel={t('admin_system_traffic_usage_mode')}
            onChange={() => void saveUsageMode(draft.usage_mode === 'billing' ? 'lite' : 'billing')}
          />
        </SettingRow>

        <SettingRow title={t('traffic_guest_access')} description={t('traffic_guest_access_desc')}>
          <IOSSwitch
            checked={draft.guest_access_mode === 'by_node'}
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
            value={draft.direction_mode}
            disabled={loading || savingDirection}
            aria-label={t('traffic_direction_mode')}
            width="auto"
            className="min-w-44"
            onChange={(event) => void saveDirectionMode(event.target.value as TrafficDirectionMode)}
          >
            {trafficDirectionModes.map((mode) => (
              <option key={mode} value={mode}>
                {t(`traffic_direction_${mode}` as TranslationKey)}
              </option>
            ))}
          </Select>
        </SettingRow>

        <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-5 shadow-sm transition-[border-color,background-color] hover:border-(--theme-border-hover) hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default) dark:hover:bg-(--theme-canvas-subtle)">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="min-w-0">
              <div className="text-sm font-semibold text-(--theme-fg-default)">
                {t('traffic_cycle_mode')}
              </div>
              <div className="mt-1 max-w-160 text-xs/5 text-(--theme-fg-muted)">
                {t('traffic_cycle_mode_desc')}
              </div>
            </div>
            <Button
              type="button"
              icon={Save}
              disabled={loading || savingSettings || !changed || !cycleValid}
              onClick={() => void saveTrafficSettings()}
            >
              {savingSettings ? t('admin_system_settings_saving') : t('common_save_changes')}
            </Button>
          </div>

          <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <label className="grid gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                {t('traffic_cycle_mode')}
              </span>
              <Select
                value={draft.cycle_mode}
                disabled={loading || savingSettings}
                aria-label={t('traffic_cycle_mode')}
                onChange={(event) => setCycleMode(event.target.value as TrafficCycleMode)}
              >
                {trafficCycleModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {t(`traffic_cycle_${mode}` as TranslationKey)}
                  </option>
                ))}
              </Select>
            </label>

            {showBillingStartDay && (
              <label className="grid gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                  {t('traffic_billing_start_day')}
                </span>
                <Input
                  type="number"
                  min={1}
                  max={31}
                  disabled={loading || savingSettings}
                  aria-label={t('traffic_billing_start_day')}
                  value={draft.billing_start_day}
                  onChange={(event) => {
                    const next = Number(event.target.value);
                    setDraft((current) => ({
                      ...current,
                      billing_start_day: Number.isFinite(next)
                        ? clampBillingDay(next)
                        : current.billing_start_day,
                    }));
                  }}
                />
              </label>
            )}

            {showAnchorDate && (
              <label className="grid gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                  {t('traffic_anchor_date')}
                </span>
                <Input
                  type="date"
                  required
                  disabled={loading || savingSettings}
                  aria-label={t('traffic_anchor_date')}
                  value={draft.billing_anchor_date}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      billing_anchor_date: event.target.value,
                      billing_start_day: billingDayFromAnchor(
                        event.target.value,
                        current.billing_start_day,
                      ),
                    }))
                  }
                />
              </label>
            )}

            {showTimezone && (
              <label className="grid gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                  {t('traffic_billing_timezone')}
                </span>
                <TimezoneSelect
                  value={draft.billing_timezone}
                  disabled={loading || savingSettings}
                  ariaLabel={t('traffic_billing_timezone')}
                  placeholder={t('traffic_billing_timezone_placeholder')}
                  systemLabel={t('traffic_billing_timezone_system')}
                  emptyLabel={t('traffic_billing_timezone_empty')}
                  onChange={(value) =>
                    setDraft((current) => ({
                      ...current,
                      billing_timezone: value,
                    }))
                  }
                />
              </label>
            )}
          </div>
        </div>
      </div>
    </section>
  );
};

export default TrafficSettings;

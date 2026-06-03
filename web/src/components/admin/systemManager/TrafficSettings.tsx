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
import { pushTopBanner } from '@runtime/topBannerRuntime';
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
  trafficCycleLabelKey,
  trafficCyclePatchFromFields,
  trafficCycleValid,
  trafficDirectionModes,
  trafficDirectionLabelKey,
  trafficSettingsWithCycleMode,
  parseTrafficCycleMode,
  parseTrafficDirectionMode,
} from '@lib/trafficSettingsModel';
import type { TrafficSettings as TrafficSettingsView, TrafficUsageMode } from '@app-types/traffic';
import { useI18n } from '@i18n';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import {
  loadTrafficSettings,
  patchTrafficSettings,
  useTrafficSettingsStore,
} from '@stores/trafficSettingsStore';
import { isCanceledRequestError } from '@utils/errors';

type TrafficDraftState = {
  value: TrafficSettingsView;
  dirty: boolean;
};

const initialTrafficDraft: TrafficDraftState = {
  value: defaultTrafficSettings,
  dirty: false,
};

const mergeTrafficDraft = (
  current: TrafficDraftState,
  settings: TrafficSettingsView,
): TrafficDraftState => {
  const value = current.dirty
    ? {
        ...current.value,
        usage_mode: settings.usage_mode,
        guest_access_mode: settings.guest_access_mode,
        direction_mode: settings.direction_mode,
      }
    : settings;

  return {
    value,
    dirty: trafficCycleChanged(value, settings),
  };
};

const TrafficSettings: React.FC = () => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const { dialogProps: confirmDialogProps, request: requestConfirm } = useConfirmDialog();
  const trafficSettings = useTrafficSettingsStore((state) => state.settings);
  const loading = useTrafficSettingsStore((state) => state.loading);
  const savingSettings = useTrafficSettingsStore((state) => state.savingCycle);
  const savingMode = useTrafficSettingsStore((state) => state.savingUsageMode);
  const savingGuestAccess = useTrafficSettingsStore((state) => state.savingGuestAccess);
  const savingDirection = useTrafficSettingsStore((state) => state.savingDirection);
  const [draftState, setDraftState] = React.useState<TrafficDraftState>(initialTrafficDraft);
  const draft = draftState.value;

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

  React.useEffect(() => {
    setDraftState((current) => mergeTrafficDraft(current, trafficSettings));
  }, [trafficSettings]);

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

  const saveTrafficSettings = React.useCallback(async () => {
    if (savingSettings || !trafficCycleChanged(draft, trafficSettings) || !trafficCycleValid(draft))
      return;
    try {
      const next = normalizeTrafficCycleFields(draft);
      const didSave = await patchTrafficSettings(
        trafficCyclePatchFromFields(draft, trafficSettings),
        {
          kind: 'cycle',
          commit: next,
        },
      );
      if (!didSave) return;
      setDraftState({ value: { ...draft, ...next }, dirty: false });
      pushTopBanner(t('admin_traffic_settings_saved'), { tone: 'info' });
    } catch (error) {
      apiError(error, { key: 'traffic_settings_save_failed' });
    }
  }, [apiError, draft, savingSettings, t, trafficSettings]);

  const saveGuestAccess = React.useCallback(async () => {
    if (loading || savingGuestAccess) return;
    const nextMode = draft.guest_access_mode === 'by_node' ? 'disabled' : 'by_node';
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
  }, [apiError, draft.guest_access_mode, loading, savingGuestAccess, t]);

  const saveDirectionMode = React.useCallback(
    async (mode: TrafficSettingsView['direction_mode']) => {
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

  const updateCycleDraft = React.useCallback(
    (update: (current: TrafficSettingsView) => TrafficSettingsView) => {
      setDraftState((current) => ({
        value: update(current.value),
        dirty: true,
      }));
    },
    [],
  );

  const setCycleMode = React.useCallback(
    (mode: TrafficSettingsView['cycle_mode']) => {
      updateCycleDraft((current) => trafficSettingsWithCycleMode(current, mode));
    },
    [updateCycleDraft],
  );

  const changed = trafficCycleChanged(draft, trafficSettings);
  const cycleValid = trafficCycleValid(draft);
  const showBillingStartDay = cycleNeedsBillingStartDay(draft.cycle_mode);
  const showAnchorDate = cycleNeedsAnchorDate(draft.cycle_mode);
  const showTimezone = cycleNeedsTimezone(draft.cycle_mode);

  return (
    <section className="space-y-4">
      <ConfirmDialog {...confirmDialogProps} />

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
                onChange={(event) => {
                  const mode = parseTrafficCycleMode(event.target.value);
                  if (mode) setCycleMode(mode);
                }}
              >
                {trafficCycleModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {t(trafficCycleLabelKey[mode])}
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
                    updateCycleDraft((current) => ({
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
                  onChange={(event) => {
                    updateCycleDraft((current) => ({
                      ...current,
                      billing_anchor_date: event.target.value,
                      billing_start_day: billingDayFromAnchor(
                        event.target.value,
                        current.billing_start_day,
                      ),
                    }));
                  }}
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
                  onChange={(value) => {
                    updateCycleDraft((current) => ({
                      ...current,
                      billing_timezone: value,
                    }));
                  }}
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

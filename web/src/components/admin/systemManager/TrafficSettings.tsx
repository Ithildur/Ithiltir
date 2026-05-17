import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';
import Network from 'lucide-react/dist/esm/icons/network';
import Save from 'lucide-react/dist/esm/icons/save';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import Select from '@components/ui/Select';
import SettingRow from '@components/admin/systemManager/SettingRow';
import { useTopBanner } from '@components/ui/TopBannerStack';
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

const cycleModes: TrafficCycleMode[] = ['calendar_month', 'whmcs_compatible', 'clamp_to_month_end'];
const directionModes: TrafficDirectionMode[] = ['out', 'both', 'max'];
const fallbackTimeZones = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Taipei',
  'Asia/Singapore',
  'Asia/Tokyo',
  'Europe/London',
  'Europe/Berlin',
  'America/New_York',
  'America/Los_Angeles',
];

const defaultTrafficSettings: TrafficSettingsView = {
  guest_access_mode: 'disabled',
  usage_mode: 'lite',
  cycle_mode: 'calendar_month',
  billing_start_day: 1,
  billing_anchor_date: '',
  billing_timezone: '',
  direction_mode: 'out',
};

const getTimeZones = (): string[] => {
  const supportedValuesOf = (
    Intl as typeof Intl & { supportedValuesOf?: (input: string) => string[] }
  ).supportedValuesOf;
  const supported = supportedValuesOf ? supportedValuesOf('timeZone') : fallbackTimeZones;
  return Array.from(new Set([...fallbackTimeZones, ...supported]));
};

type TimezoneSelectProps = {
  value: string;
  disabled?: boolean;
  ariaLabel: string;
  placeholder: string;
  systemLabel: string;
  emptyLabel: string;
  onChange: (value: string) => void;
};

const TimezoneSelect: React.FC<TimezoneSelectProps> = ({
  value,
  disabled,
  ariaLabel,
  placeholder,
  systemLabel,
  emptyLabel,
  onChange,
}) => {
  const listboxId = React.useId();
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState(value);

  React.useEffect(() => {
    if (!open) setQuery(value);
  }, [open, value]);

  const options = React.useMemo(() => {
    const trimmed = value.trim();
    const zones = getTimeZones();
    if (trimmed && !zones.includes(trimmed)) return [trimmed, ...zones];
    return zones;
  }, [value]);

  const normalizedQuery = query.trim().toLowerCase();
  const systemMatched = !normalizedQuery || systemLabel.toLowerCase().includes(normalizedQuery);
  const filteredOptions = React.useMemo(() => {
    if (!normalizedQuery) return options.slice(0, 80);
    return options
      .filter((timeZone) => timeZone.toLowerCase().includes(normalizedQuery))
      .slice(0, 80);
  }, [normalizedQuery, options]);

  const selectValue = React.useCallback(
    (nextValue: string) => {
      onChange(nextValue);
      setQuery(nextValue);
      setOpen(false);
    },
    [onChange],
  );

  return (
    <div
      ref={containerRef}
      className="relative"
      onBlur={(event) => {
        const nextTarget = event.relatedTarget;
        if (!(nextTarget instanceof Node) || !containerRef.current?.contains(nextTarget)) {
          setOpen(false);
        }
      }}
    >
      <div className="relative">
        <input
          value={open ? query : value}
          disabled={disabled}
          aria-label={ariaLabel}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={open}
          role="combobox"
          placeholder={placeholder}
          className="w-full rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) px-3 py-1.25 pr-9 text-sm/5 text-(--theme-fg-default) outline-none transition-[background-color,border-color,box-shadow] placeholder:text-(--theme-fg-subtle) focus:border-(--theme-bg-accent-emphasis) focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
          onFocus={() => {
            setQuery(value);
            setOpen(true);
          }}
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(true);
          }}
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              setOpen(false);
              return;
            }
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setOpen(true);
              return;
            }
            if (event.key === 'Enter' && open) {
              event.preventDefault();
              const nextValue = systemMatched ? '' : filteredOptions[0];
              if (nextValue !== undefined) selectValue(nextValue);
            }
          }}
        />
        <button
          type="button"
          tabIndex={-1}
          disabled={disabled}
          aria-hidden="true"
          className="absolute inset-y-0 right-1.5 grid w-7 place-items-center rounded text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default) disabled:pointer-events-none"
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => setOpen((current) => !current)}
        >
          <ChevronDown className="size-4" aria-hidden="true" />
        </button>
      </div>

      {open && !disabled ? (
        <div
          id={listboxId}
          role="listbox"
          className="absolute z-30 mt-1 max-h-64 w-full min-w-64 overflow-y-auto rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) py-1 text-sm shadow-lg dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
        >
          {systemMatched ? (
            <button
              type="button"
              role="option"
              aria-selected={!value}
              className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectValue('')}
            >
              <span>{systemLabel}</span>
              {!value ? (
                <Check className="size-4 text-(--theme-bg-accent-emphasis)" aria-hidden="true" />
              ) : null}
            </button>
          ) : null}
          {filteredOptions.map((timeZone) => (
            <button
              key={timeZone}
              type="button"
              role="option"
              aria-selected={value === timeZone}
              className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectValue(timeZone)}
            >
              <span className="truncate">{timeZone}</span>
              {value === timeZone ? (
                <Check
                  className="size-4 shrink-0 text-(--theme-bg-accent-emphasis)"
                  aria-hidden="true"
                />
              ) : null}
            </button>
          ))}
          {!systemMatched && filteredOptions.length === 0 ? (
            <div className="px-3 py-2 text-(--theme-fg-muted)">{emptyLabel}</div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
};

const cycleChanged = (draft: TrafficSettingsView, saved: TrafficSettingsView) =>
  draft.cycle_mode !== saved.cycle_mode ||
  draft.billing_start_day !== saved.billing_start_day ||
  draft.billing_anchor_date !== saved.billing_anchor_date ||
  draft.billing_timezone !== saved.billing_timezone;

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
    if (savingSettings || !cycleChanged(draft, trafficSettings)) return;
    setSavingSettings(true);
    try {
      const next = {
        cycle_mode: draft.cycle_mode,
        billing_start_day: draft.billing_start_day,
        billing_anchor_date: draft.billing_anchor_date,
        billing_timezone: draft.billing_timezone,
      };
      await updateTrafficSettings(next);
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
    setDraft((current) => ({
      ...current,
      cycle_mode: mode,
      billing_start_day: mode === 'calendar_month' ? 1 : current.billing_start_day,
      billing_anchor_date: mode === 'whmcs_compatible' ? current.billing_anchor_date : '',
    }));
  }, []);

  const changed = cycleChanged(draft, trafficSettings);
  const cycleLocked = draft.usage_mode === 'lite';

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
            {directionModes.map((mode) => (
              <option key={mode} value={mode}>
                {t(`traffic_direction_${mode}` as TranslationKey)}
              </option>
            ))}
          </Select>
        </SettingRow>

        <div
          className={`rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-5 shadow-sm transition-[border-color,background-color] hover:border-(--theme-border-hover) hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default) dark:hover:bg-(--theme-canvas-subtle) ${
            cycleLocked ? 'opacity-70' : ''
          }`}
        >
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
              disabled={loading || savingSettings || cycleLocked || !changed}
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
                disabled={loading || savingSettings || cycleLocked}
                aria-label={t('traffic_cycle_mode')}
                onChange={(event) => setCycleMode(event.target.value as TrafficCycleMode)}
              >
                {cycleModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {t(`traffic_cycle_${mode}` as TranslationKey)}
                  </option>
                ))}
              </Select>
            </label>

            <label className="grid gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                {t('traffic_billing_start_day')}
              </span>
              <Input
                type="number"
                min={1}
                max={31}
                disabled={
                  loading || savingSettings || cycleLocked || draft.cycle_mode === 'calendar_month'
                }
                aria-label={t('traffic_billing_start_day')}
                value={draft.billing_start_day}
                onChange={(event) => {
                  const next = Number(event.target.value);
                  setDraft((current) => ({
                    ...current,
                    billing_start_day: Number.isFinite(next)
                      ? Math.max(1, Math.min(31, Math.trunc(next)))
                      : current.billing_start_day,
                  }));
                }}
              />
            </label>

            <label className="grid gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                {t('traffic_anchor_date')}
              </span>
              <Input
                type="date"
                disabled={
                  loading ||
                  savingSettings ||
                  cycleLocked ||
                  draft.cycle_mode !== 'whmcs_compatible'
                }
                aria-label={t('traffic_anchor_date')}
                value={draft.billing_anchor_date}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    billing_anchor_date: event.target.value,
                    billing_start_day: event.target.value
                      ? Number(event.target.value.slice(-2))
                      : current.billing_start_day,
                  }))
                }
              />
            </label>

            <label className="grid gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                {t('traffic_billing_timezone')}
              </span>
              <TimezoneSelect
                value={draft.billing_timezone}
                disabled={loading || savingSettings || cycleLocked}
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
          </div>
        </div>
      </div>
    </section>
  );
};

export default TrafficSettings;

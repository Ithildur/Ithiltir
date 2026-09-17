import React from 'react';
import Save from 'lucide-react/dist/esm/icons/save';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import SettingRow, { SettingPanel, SettingPanelFooter } from './SettingRow';
import type { SystemSettings } from '@app-types/admin';
import { useI18n } from '@i18n';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { updateSystemSettings } from '@lib/adminApi';
import { pushTopBanner } from '@runtime/topBannerRuntime';

type UptimeFields = Pick<
  SystemSettings,
  'uptime_guest_visible' | 'uptime_warning_sla' | 'uptime_error_sla'
>;

type Props = {
  settings: SystemSettings | null;
  loading: boolean;
  onSaved: (fields: Partial<UptimeFields>) => void;
};

type Draft = { guestVisible: boolean; warning: string; error: string };

const UptimeSettings: React.FC<Props> = ({ settings, loading, onSaved }) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const [draft, setDraft] = React.useState<Draft | null>(null);
  const [saving, setSaving] = React.useState(false);
  const errorID = React.useId();
  const current = draft ?? {
    guestVisible: settings?.uptime_guest_visible ?? true,
    warning: String(settings?.uptime_warning_sla ?? 99),
    error: String(settings?.uptime_error_sla ?? 95),
  };
  const warning = Number(current.warning);
  const error = Number(current.error);
  const valid =
    current.warning.trim() !== '' &&
    current.error.trim() !== '' &&
    Number.isFinite(warning) &&
    Number.isFinite(error) &&
    error >= 0 &&
    error < warning &&
    warning <= 100;
  const changed =
    settings !== null &&
    (current.guestVisible !== settings.uptime_guest_visible ||
      warning !== settings.uptime_warning_sla ||
      error !== settings.uptime_error_sla);
  const disabled = !settings || loading || saving;

  const save = async () => {
    if (!settings || disabled || !valid || !changed) return;
    const fields: Partial<UptimeFields> = {};
    if (current.guestVisible !== settings.uptime_guest_visible) {
      fields.uptime_guest_visible = current.guestVisible;
    }
    if (warning !== settings.uptime_warning_sla) fields.uptime_warning_sla = warning;
    if (error !== settings.uptime_error_sla) fields.uptime_error_sla = error;
    setSaving(true);
    try {
      await updateSystemSettings(fields);
      onSaved(fields);
      setDraft(null);
      pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
    } catch (failure) {
      apiError(failure, { key: 'admin_system_settings_save_failed' });
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <SettingPanel title={t('admin_uptime_title')} description={t('admin_uptime_description')}>
        <SettingRow
          title={t('admin_uptime_guest_visible')}
          description={t('admin_uptime_guest_hint')}
        >
          <IOSSwitch
            checked={current.guestVisible}
            disabled={disabled}
            ariaLabel={t('admin_uptime_guest_visible')}
            onChange={() => setDraft({ ...current, guestVisible: !current.guestVisible })}
          />
        </SettingRow>
        <SettingRow title={t('admin_uptime_warning')} description={t('admin_uptime_warning_hint')}>
          <Input
            type="number"
            inputMode="decimal"
            min={0}
            max={100}
            step="any"
            required
            wrapperClassName="w-full max-w-40"
            aria-label={t('admin_uptime_warning')}
            aria-invalid={!valid}
            aria-describedby={!valid ? errorID : undefined}
            rightElement={<span aria-hidden="true">%</span>}
            value={current.warning}
            disabled={disabled}
            onChange={(event) => setDraft({ ...current, warning: event.target.value })}
          />
        </SettingRow>
        <SettingRow title={t('admin_uptime_error')} description={t('admin_uptime_error_hint')}>
          <Input
            type="number"
            inputMode="decimal"
            min={0}
            max={100}
            step="any"
            required
            wrapperClassName="w-full max-w-40"
            aria-label={t('admin_uptime_error')}
            aria-invalid={!valid}
            aria-describedby={!valid ? errorID : undefined}
            rightElement={<span aria-hidden="true">%</span>}
            value={current.error}
            disabled={disabled}
            onChange={(event) => setDraft({ ...current, error: event.target.value })}
          />
        </SettingRow>
        {!valid && (
          <p id={errorID} role="alert" className="px-5 py-3 text-sm text-(--theme-fg-danger)">
            {t('admin_uptime_invalid_sla')}
          </p>
        )}
        <SettingPanelFooter>
          <Button type="submit" icon={Save} disabled={disabled || !changed || !valid}>
            {saving ? t('admin_system_settings_saving') : t('common_save_changes')}
          </Button>
        </SettingPanelFooter>
      </SettingPanel>
    </form>
  );
};

export default UptimeSettings;

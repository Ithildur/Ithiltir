import React from 'react';
import Eye from 'lucide-react/dist/esm/icons/eye';
import EyeOff from 'lucide-react/dist/esm/icons/eye-off';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import { useI18n } from '@i18n';
import type { ChannelDrafts } from '@components/admin/alertManager/alertChannelForm';

type EmailDraft = ChannelDrafts['email'];

interface Props {
  draft: EmailDraft;
  showPassword: boolean;
  onTogglePassword: () => void;
  onPatch: (patch: Partial<Omit<EmailDraft, 'kind'>>) => void;
}

const EmailChannelForm: React.FC<Props> = ({ draft, showPassword, onTogglePassword, onPatch }) => {
  const { t } = useI18n();

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
            {t('admin_alerts_channels_label_smtp_host')}
          </label>
          <Input
            value={draft.emailHost}
            onChange={(event) => onPatch({ emailHost: event.target.value })}
            placeholder={t('admin_alerts_channels_placeholder_smtp_host')}
            className="font-mono dark:bg-(--theme-bg-inset)"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
            {t('admin_alerts_channels_label_smtp_port')}
          </label>
          <Input
            value={draft.emailPort}
            onChange={(event) => onPatch({ emailPort: event.target.value })}
            placeholder={t('admin_alerts_channels_placeholder_smtp_port')}
            className="font-mono dark:bg-(--theme-bg-inset)"
          />
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
            {t('admin_alerts_channels_label_smtp_username')}
          </label>
          <Input
            value={draft.emailUsername}
            onChange={(event) => onPatch({ emailUsername: event.target.value })}
            placeholder={t('admin_alerts_channels_placeholder_smtp_username')}
            className="font-mono dark:bg-(--theme-bg-inset)"
          />
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
            {t('admin_alerts_channels_label_smtp_password')}
          </label>
          <Input
            type={showPassword ? 'text' : 'password'}
            value={draft.emailPassword}
            onChange={(event) => onPatch({ emailPassword: event.target.value })}
            placeholder={t('admin_alerts_channels_placeholder_smtp_password')}
            className="font-mono dark:bg-(--theme-bg-inset)"
            rightElement={
              <button
                type="button"
                onClick={onTogglePassword}
                className="text-(--theme-fg-muted-alt) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
                aria-label={showPassword ? 'Hide password' : 'Show password'}
              >
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            }
          />
        </div>
      </div>
      <div className="space-y-1.5">
        <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
          {t('admin_alerts_channels_label_email_from')}
        </label>
        <Input
          value={draft.emailFrom}
          onChange={(event) => onPatch({ emailFrom: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_email_from')}
          className="font-mono dark:bg-(--theme-bg-inset)"
        />
      </div>
      <div className="space-y-1.5">
        <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
          {t('admin_alerts_channels_label_email_recipients')}
        </label>
        <Input
          value={draft.emailRecipients}
          onChange={(event) => onPatch({ emailRecipients: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_email_recipients')}
          className="font-mono dark:bg-(--theme-bg-inset)"
        />
      </div>
      <div className="flex items-center justify-between rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info) dark:bg-(--theme-bg-default) px-4 py-3">
        <div className="space-y-1">
          <p className="text-sm font-medium text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
            {t('admin_alerts_channels_label_email_tls')}
          </p>
          <p className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
            {t('admin_alerts_channels_help_email_tls')}
          </p>
        </div>
        <IOSSwitch
          size="sm"
          checked={draft.emailUseTls}
          onChange={() => onPatch({ emailUseTls: !draft.emailUseTls })}
        />
      </div>
    </div>
  );
};

export default EmailChannelForm;

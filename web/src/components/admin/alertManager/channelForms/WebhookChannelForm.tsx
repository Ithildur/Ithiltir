import React from 'react';
import Eye from 'lucide-react/dist/esm/icons/eye';
import EyeOff from 'lucide-react/dist/esm/icons/eye-off';
import InfoIcon from 'lucide-react/dist/esm/icons/info';
import Input from '@components/ui/Input';
import { useI18n } from '@i18n';
import type { ChannelDrafts } from '@components/admin/alertManager/alertChannelForm';

type WebhookDraft = ChannelDrafts['webhook'];

interface Props {
  draft: WebhookDraft;
  showSecret: boolean;
  onToggleSecret: () => void;
  onPatch: (patch: Partial<Omit<WebhookDraft, 'kind'>>) => void;
}

const WebhookChannelForm: React.FC<Props> = ({ draft, showSecret, onToggleSecret, onPatch }) => {
  const { t } = useI18n();

  return (
    <div className="space-y-5">
      <div className="space-y-1.5">
        <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
          {t('admin_alerts_channels_label_webhook_url')}
        </label>
        <Input
          value={draft.webhookUrl}
          onChange={(event) => onPatch({ webhookUrl: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_webhook_url')}
          className="font-mono dark:bg-(--theme-bg-inset)"
        />
      </div>
      <div className="space-y-1.5">
        <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
          {t('admin_alerts_channels_label_webhook_secret')}
        </label>
        <Input
          type={showSecret ? 'text' : 'password'}
          value={draft.webhookSecret}
          onChange={(event) => onPatch({ webhookSecret: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_webhook_secret')}
          className="font-mono dark:bg-(--theme-bg-inset)"
          rightElement={
            <button
              type="button"
              onClick={onToggleSecret}
              className="text-(--theme-fg-muted-alt) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
              aria-label={showSecret ? 'Hide secret' : 'Show secret'}
            >
              {showSecret ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          }
        />
      </div>
      <div className="flex items-start gap-3 rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info) dark:bg-(--theme-bg-default) p-4">
        <InfoIcon className="text-(--theme-fg-interactive) mt-0.5" size={18} />
        <div className="space-y-1">
          <p className="text-xs font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
            {t('admin_alerts_channels_webhook_hint_title')}
          </p>
          <p className="text-[11px] text-(--theme-fg-muted) dark:text-(--theme-fg-muted) leading-relaxed">
            {t('admin_alerts_channels_webhook_hint_body')}
          </p>
        </div>
      </div>
    </div>
  );
};

export default WebhookChannelForm;

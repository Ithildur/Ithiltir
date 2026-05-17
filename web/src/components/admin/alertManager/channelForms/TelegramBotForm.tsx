import React from 'react';
import Eye from 'lucide-react/dist/esm/icons/eye';
import EyeOff from 'lucide-react/dist/esm/icons/eye-off';
import InfoIcon from 'lucide-react/dist/esm/icons/info';
import Input from '@components/ui/Input';
import { useI18n } from '@i18n';
import type { ChannelDrafts } from '@components/admin/alertManager/alertChannelForm';

type TelegramBotDraft = ChannelDrafts['telegram_bot'];

interface Props {
  draft: TelegramBotDraft;
  showBotToken: boolean;
  onToggleBotToken: () => void;
  onPatch: (patch: Partial<Omit<TelegramBotDraft, 'kind'>>) => void;
}

const TelegramBotForm: React.FC<Props> = ({ draft, showBotToken, onToggleBotToken, onPatch }) => {
  const { t } = useI18n();
  const botTokenId = React.useId();
  const chatIdId = React.useId();

  return (
    <div className="space-y-5">
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <label
            htmlFor={botTokenId}
            className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider"
          >
            {t('admin_alerts_channels_label_bot_token')}
          </label>
          <a
            href="https://t.me/BotFather"
            target="_blank"
            rel="noreferrer"
            className="text-[10px] text-(--theme-fg-interactive) hover:underline font-mono"
          >
            {t('admin_alerts_channels_help_bot_link')}
          </a>
        </div>
        <Input
          id={botTokenId}
          type={showBotToken ? 'text' : 'password'}
          value={draft.botToken}
          onChange={(event) => onPatch({ botToken: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_bot_token')}
          className="font-mono dark:bg-(--theme-bg-inset)"
          rightElement={
            <button
              type="button"
              onClick={onToggleBotToken}
              className="ui-focus-ring rounded text-(--theme-fg-muted-alt) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-control-hover) transition-colors"
              aria-label={showBotToken ? 'Hide token' : 'Show token'}
            >
              {showBotToken ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          }
        />
      </div>

      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <label
            htmlFor={chatIdId}
            className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider"
          >
            {t('admin_alerts_channels_label_chat_id')}
          </label>
          <span className="text-[10px] text-(--theme-fg-muted-alt) font-mono">
            {t('admin_alerts_channels_help_chat_id')}
          </span>
        </div>
        <Input
          id={chatIdId}
          value={draft.chatId}
          onChange={(event) => onPatch({ chatId: event.target.value })}
          placeholder={t('admin_alerts_channels_placeholder_chat_id')}
          className="font-mono dark:bg-(--theme-bg-inset)"
        />
      </div>

      <div className="flex items-start gap-3 rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info) dark:bg-(--theme-bg-default) p-4 border-l-2 border-l-(--theme-fg-interactive)">
        <InfoIcon className="text-(--theme-fg-interactive) mt-0.5" size={18} />
        <div className="space-y-1">
          <p className="text-xs font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
            {t('admin_alerts_channels_info_title')}
          </p>
          <p className="text-[11px] text-(--theme-fg-muted) dark:text-(--theme-fg-muted) leading-relaxed">
            {t('admin_alerts_channels_info_body')}
          </p>
        </div>
      </div>
    </div>
  );
};

export default TelegramBotForm;

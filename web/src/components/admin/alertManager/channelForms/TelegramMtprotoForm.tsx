import React from 'react';
import Eye from 'lucide-react/dist/esm/icons/eye';
import EyeOff from 'lucide-react/dist/esm/icons/eye-off';
import Phone from 'lucide-react/dist/esm/icons/phone';
import Shield from 'lucide-react/dist/esm/icons/shield';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import { useI18n } from '@i18n';
import type { ChannelDrafts } from '@components/admin/alertManager/alertChannelForm';
import type { TelegramMtprotoLoginState } from '@components/admin/alertManager/hooks/useTelegramMtprotoLogin';

type TelegramMtprotoDraft = ChannelDrafts['telegram_mtproto'];

interface Props {
  channelId?: number;
  draft: TelegramMtprotoDraft;
  login: TelegramMtprotoLoginState;
  showApiHash: boolean;
  onToggleApiHash: () => void;
  onPatch: (patch: Partial<Omit<TelegramMtprotoDraft, 'kind'>>) => void;
}

const TelegramMtprotoForm: React.FC<Props> = ({
  channelId,
  draft,
  login,
  showApiHash,
  onToggleApiHash,
  onPatch,
}) => {
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider flex items-center gap-2">
            <Shield size={14} /> {t('admin_alerts_channels_section_app_credentials')}
          </h3>
          <a
            href="https://my.telegram.org"
            target="_blank"
            rel="noreferrer"
            className="text-xs text-(--theme-fg-interactive) hover:underline font-mono"
          >
            my.telegram.org
          </a>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
              {t('admin_alerts_channels_label_api_id')}
            </label>
            <Input
              value={draft.apiId}
              onChange={(event) => onPatch({ apiId: event.target.value })}
              placeholder={t('admin_alerts_channels_placeholder_api_id')}
              className="font-mono dark:bg-(--theme-bg-inset)"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
              {t('admin_alerts_channels_label_api_hash')}
            </label>
            <Input
              type={showApiHash ? 'text' : 'password'}
              value={draft.apiHash}
              onChange={(event) => onPatch({ apiHash: event.target.value })}
              placeholder={t('admin_alerts_channels_placeholder_api_hash')}
              className="font-mono dark:bg-(--theme-bg-inset)"
              rightElement={
                <button
                  type="button"
                  onClick={onToggleApiHash}
                  className="text-(--theme-fg-muted-alt) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
                  aria-label={showApiHash ? 'Hide hash' : 'Show hash'}
                >
                  {showApiHash ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              }
            />
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-4 items-end">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
              {t('admin_alerts_channels_label_phone')}
            </label>
            <Input
              value={draft.phoneNumber}
              onChange={(event) => onPatch({ phoneNumber: event.target.value })}
              placeholder={t('admin_alerts_channels_placeholder_phone')}
              className="font-mono dark:bg-(--theme-bg-inset)"
            />
          </div>
          <Button
            type="button"
            variant="secondary"
            className="w-full md:w-auto"
            disabled={!channelId || login.isRequestingCode}
            onClick={login.requestCode}
          >
            <Phone size={16} />
            {t('admin_alerts_channels_label_request_code')}
          </Button>
        </div>
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
            {t('admin_alerts_channels_label_chat_id')}
          </label>
          <Input
            value={draft.chatId}
            onChange={(event) => onPatch({ chatId: event.target.value })}
            placeholder={t('admin_alerts_channels_placeholder_chat_id')}
            className="font-mono dark:bg-(--theme-bg-inset)"
          />
        </div>
        {login.loginId && login.loginTimeout !== null && (
          <p className="text-xs text-(--theme-fg-muted-alt) font-mono">
            {t('admin_alerts_channels_login_hint', { timeout: login.loginTimeout })}
          </p>
        )}
      </div>

      <div className="relative py-1">
        <div className="absolute inset-0 flex items-center">
          <div className="w-full border-t border-(--theme-border-subtle) dark:border-(--theme-border-default)" />
        </div>
        <div className="relative flex justify-center">
          <span className="bg-(--theme-bg-default) dark:bg-(--theme-bg-inset) px-3 text-[10px] text-(--theme-fg-muted-alt) uppercase font-mono tracking-widest border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-full">
            Step 2
          </span>
        </div>
      </div>

      <div className="space-y-4 rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info) dark:bg-(--theme-bg-default) p-4">
        <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider flex items-center gap-2">
          <Shield size={14} /> {t('admin_alerts_channels_section_login')}
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-4 items-end">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
              {t('admin_alerts_channels_label_login_code')}
            </label>
            <Input
              value={login.loginCode}
              onChange={(event) => login.setLoginCode(event.target.value)}
              placeholder={t('admin_alerts_channels_placeholder_login_code')}
              className="font-mono dark:bg-(--theme-bg-inset)"
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            className="w-full md:w-auto"
            onClick={login.verifyCode}
            disabled={!login.loginId || !login.loginCode.trim() || login.isVerifyingCode}
          >
            {t('admin_alerts_channels_label_verify_code')}
          </Button>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-4 items-end">
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1 uppercase tracking-wider">
              {t('admin_alerts_channels_label_2fa')}
            </label>
            <Input
              type="password"
              value={login.twoFactorPassword}
              onChange={(event) => login.setTwoFactorPassword(event.target.value)}
              placeholder={t('admin_alerts_channels_placeholder_2fa')}
              className="font-mono dark:bg-(--theme-bg-inset)"
            />
          </div>
          <Button
            type="button"
            variant="secondary"
            className="w-full md:w-auto"
            onClick={login.submitPassword}
            disabled={
              !login.passwordRequired ||
              !login.twoFactorPassword.trim() ||
              login.isSubmittingPassword
            }
          >
            {t('admin_alerts_channels_label_submit_password')}
          </Button>
        </div>
        <div className="flex items-center justify-between rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-control) dark:bg-(--theme-bg-inset) px-3 py-2">
          <span className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong)">
            {t('admin_alerts_channels_label_connection_status')}
          </span>
          <span
            className={`inline-flex items-center gap-1.5 text-xs font-semibold ${login.connectionClass}`}
          >
            <span className={`size-2 rounded-full ${login.connectionDotClass}`} />
            {login.connectionLabel}
          </span>
        </div>
        {login.connectionReason && login.connectionStatus === 'invalid' && (
          <p className="text-xs text-(--theme-fg-danger) dark:text-(--theme-fg-danger)">
            {t('admin_alerts_channels_status_reason', { reason: login.connectionReason })}
          </p>
        )}
      </div>
    </div>
  );
};

export default TelegramMtprotoForm;

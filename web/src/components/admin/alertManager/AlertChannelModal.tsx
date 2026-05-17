import React from 'react';
import BellRing from 'lucide-react/dist/esm/icons/bell-ring';
import FlaskConical from 'lucide-react/dist/esm/icons/flask-conical';
import InfoIcon from 'lucide-react/dist/esm/icons/info';
import Mail from 'lucide-react/dist/esm/icons/mail';
import Send from 'lucide-react/dist/esm/icons/send';
import Webhook from 'lucide-react/dist/esm/icons/webhook';
import type { LucideIcon } from 'lucide-react';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import { useI18n } from '@i18n';
import type { AlertChannelType, AlertTelegramMode } from '@app-types/admin';
import type { AlertChannelForm } from './alertChannelForm';
import EmailChannelForm from './channelForms/EmailChannelForm';
import TelegramBotForm from './channelForms/TelegramBotForm';
import TelegramMtprotoForm from './channelForms/TelegramMtprotoForm';
import WebhookChannelForm from './channelForms/WebhookChannelForm';
import { useAlertChannelForm } from './hooks/useAlertChannelForm';
import { useTelegramMtprotoLogin } from './hooks/useTelegramMtprotoLogin';

export type { AlertChannelForm } from './alertChannelForm';

interface Props {
  isOpen: boolean;
  mode: 'add' | 'edit';
  channelId?: number;
  initialForm?: AlertChannelForm;
  isSaving?: boolean;
  onClose: () => void;
  onSave?: (input: AlertChannelForm) => void;
}

type TelegramTab = AlertTelegramMode;

type ChannelTabLabelKey =
  | 'admin_alerts_channels_tab_telegram'
  | 'admin_alerts_channels_tab_email'
  | 'admin_alerts_channels_tab_webhook';

type ChannelTabMeta = {
  key: AlertChannelType;
  labelKey: ChannelTabLabelKey;
  icon: LucideIcon;
};

const channelTabs: ChannelTabMeta[] = [
  { key: 'telegram', labelKey: 'admin_alerts_channels_tab_telegram', icon: Send },
  { key: 'email', labelKey: 'admin_alerts_channels_tab_email', icon: Mail },
  { key: 'webhook', labelKey: 'admin_alerts_channels_tab_webhook', icon: Webhook },
];

const telegramTabs: TelegramTab[] = ['bot', 'mtproto'];

const AlertChannelModal: React.FC<Props> = ({
  isOpen,
  mode,
  channelId,
  initialForm,
  isSaving = false,
  onClose,
  onSave,
}) => {
  const { t } = useI18n();
  const titleId = React.useId();
  const form = useAlertChannelForm({ isOpen, initialForm });
  const telegramLogin = useTelegramMtprotoLogin({
    isOpen,
    channelId,
    channelType: form.channelType,
    telegramMode: form.telegramMode,
  });

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    onSave?.(form.currentForm());
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="max-w-3xl" ariaLabelledby={titleId}>
      <ModalHeader
        title={
          mode === 'add'
            ? t('admin_alerts_channels_modal_add_title')
            : t('admin_alerts_channels_modal_edit_title')
        }
        icon={<BellRing className="text-(--theme-border-underline-nav-active)" size={20} />}
        onClose={onClose}
        id={titleId}
      />
      <form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
        <ModalBody className="space-y-6">
          <div className="space-y-3">
            <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider">
              {t('admin_alerts_channels_section_details')}
            </h3>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1">
                {t('admin_alerts_channels_label_name')}
              </label>
              <Input
                value={form.channelName}
                onChange={(event) => form.setChannelName(event.target.value)}
                placeholder={t('admin_alerts_channels_placeholder_name')}
                className="dark:bg-(--theme-bg-inset)"
              />
            </div>
          </div>

          {mode === 'edit' && (
            <div className="flex items-start gap-3 rounded-lg border border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted)/80 px-4 py-3 text-(--theme-fg-warning-on-muted) dark:border-(--theme-border-warning-soft) dark:bg-(--theme-bg-warning-soft) dark:text-(--theme-fg-warning-on-muted)">
              <InfoIcon className="mt-0.5 shrink-0" size={16} />
              <p className="text-xs/relaxed">{t('admin_alerts_channels_edit_notice')}</p>
            </div>
          )}

          <div className="border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
            <div className="flex flex-wrap gap-6">
              {channelTabs.map(({ key, labelKey, icon: Icon }) => {
                const active = form.channelType === key;
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={() => form.setChannelType(key)}
                    className={`group flex items-center gap-2 pb-3 border-b-2 transition-colors ${
                      active
                        ? 'border-(--theme-border-underline-nav-active) text-(--theme-border-underline-nav-active)'
                        : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
                    }`}
                    aria-pressed={active}
                  >
                    <Icon className="size-4" aria-hidden="true" />
                    <span className="text-sm font-semibold tracking-wide">{t(labelKey)}</span>
                  </button>
                );
              })}
            </div>
          </div>

          {form.channelType === 'telegram' && (
            <div className="space-y-6">
              <div className="inline-flex rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-info) dark:bg-(--theme-bg-default) p-1">
                {telegramTabs.map((modeKey) => {
                  const active = form.telegramMode === modeKey;
                  return (
                    <button
                      key={modeKey}
                      type="button"
                      onClick={() => form.setTelegramMode(modeKey)}
                      className={`px-4 py-1.5 rounded-md text-xs font-semibold tracking-wide transition-colors ${
                        active
                          ? 'bg-(--theme-bg-default) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-strong) shadow-sm'
                          : 'text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
                      }`}
                      aria-pressed={active}
                    >
                      {modeKey === 'bot'
                        ? t('admin_alerts_channels_subtab_bot')
                        : t('admin_alerts_channels_subtab_mtproto')}
                    </button>
                  );
                })}
              </div>

              {form.telegramMode === 'bot' ? (
                <TelegramBotForm
                  draft={form.drafts.telegram_bot}
                  showBotToken={form.visibleSecrets.botToken}
                  onToggleBotToken={() => form.toggleSecret('botToken')}
                  onPatch={form.patchTelegramBot}
                />
              ) : (
                <TelegramMtprotoForm
                  channelId={channelId}
                  draft={form.drafts.telegram_mtproto}
                  login={telegramLogin}
                  showApiHash={form.visibleSecrets.apiHash}
                  onToggleApiHash={() => form.toggleSecret('apiHash')}
                  onPatch={form.patchTelegramMtproto}
                />
              )}
            </div>
          )}

          {form.channelType === 'email' && (
            <EmailChannelForm
              draft={form.drafts.email}
              showPassword={form.visibleSecrets.emailPassword}
              onTogglePassword={() => form.toggleSecret('emailPassword')}
              onPatch={form.patchEmail}
            />
          )}

          {form.channelType === 'webhook' && (
            <WebhookChannelForm
              draft={form.drafts.webhook}
              showSecret={form.visibleSecrets.webhookSecret}
              onToggleSecret={() => form.toggleSecret('webhookSecret')}
              onPatch={form.patchWebhook}
            />
          )}
        </ModalBody>
        <ModalFooter className="justify-between">
          <Button
            type="button"
            variant="secondary"
            icon={FlaskConical}
            onClick={telegramLogin.testConnection}
            disabled={telegramLogin.isTesting}
          >
            {t('admin_alerts_channels_button_test')}
          </Button>
          <div className="flex items-center gap-2">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSaving}>
              {t('common_cancel')}
            </Button>
            <Button type="submit" variant="primary" disabled={isSaving}>
              {isSaving
                ? t('admin_alerts_channels_button_saving')
                : t('admin_alerts_channels_button_save')}
            </Button>
          </div>
        </ModalFooter>
      </form>
    </Modal>
  );
};

export default AlertChannelModal;

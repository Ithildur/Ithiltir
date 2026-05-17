import React from 'react';
import type { AlertChannelType, AlertTelegramMode } from '@app-types/admin';
import {
  DEFAULT_CHANNEL_KIND,
  channelTypeFromKind,
  draftsFor,
  telegramModeFromKind,
  type AlertChannelForm,
  type ChannelDrafts,
  type AlertChannelFormKind,
} from '@components/admin/alertManager/alertChannelForm';

type SecretKey = 'botToken' | 'apiHash' | 'emailPassword' | 'webhookSecret';

const createVisibleSecrets = (): Record<SecretKey, boolean> => ({
  botToken: false,
  apiHash: false,
  emailPassword: false,
  webhookSecret: false,
});

export const useAlertChannelForm = ({
  isOpen,
  initialForm,
}: {
  isOpen: boolean;
  initialForm?: AlertChannelForm;
}) => {
  const [channelName, setChannelName] = React.useState(() => initialForm?.name ?? '');
  const [kind, setKind] = React.useState<AlertChannelFormKind>(
    () => initialForm?.kind ?? DEFAULT_CHANNEL_KIND,
  );
  const [drafts, setDrafts] = React.useState<ChannelDrafts>(() => draftsFor(initialForm));
  const [visibleSecrets, setVisibleSecrets] = React.useState(createVisibleSecrets);

  React.useEffect(() => {
    if (!isOpen) return;
    setChannelName(initialForm?.name ?? '');
    setKind(initialForm?.kind ?? DEFAULT_CHANNEL_KIND);
    setDrafts(draftsFor(initialForm));
    setVisibleSecrets(createVisibleSecrets());
  }, [initialForm, isOpen]);

  const setChannelType = React.useCallback((nextType: AlertChannelType) => {
    setKind((current) => {
      if (nextType !== 'telegram') return nextType;
      return current === 'telegram_mtproto' ? 'telegram_mtproto' : 'telegram_bot';
    });
  }, []);

  const setTelegramMode = React.useCallback((nextMode: AlertTelegramMode) => {
    setKind(nextMode === 'mtproto' ? 'telegram_mtproto' : 'telegram_bot');
  }, []);

  const patchTelegramBot = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['telegram_bot'], 'kind'>>) => {
      setDrafts((prev) => ({
        ...prev,
        telegram_bot: { ...prev.telegram_bot, ...patch },
      }));
    },
    [],
  );

  const patchTelegramMtproto = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['telegram_mtproto'], 'kind'>>) => {
      setDrafts((prev) => ({
        ...prev,
        telegram_mtproto: { ...prev.telegram_mtproto, ...patch },
      }));
    },
    [],
  );

  const patchEmail = React.useCallback((patch: Partial<Omit<ChannelDrafts['email'], 'kind'>>) => {
    setDrafts((prev) => ({
      ...prev,
      email: { ...prev.email, ...patch },
    }));
  }, []);

  const patchWebhook = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['webhook'], 'kind'>>) => {
      setDrafts((prev) => ({
        ...prev,
        webhook: { ...prev.webhook, ...patch },
      }));
    },
    [],
  );

  const toggleSecret = React.useCallback((key: SecretKey) => {
    setVisibleSecrets((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  const channelType = channelTypeFromKind(kind);
  const telegramMode = telegramModeFromKind(kind);

  const currentForm = React.useCallback((): AlertChannelForm => {
    if (kind === 'telegram_bot') {
      return { name: channelName, ...drafts.telegram_bot };
    }
    if (kind === 'telegram_mtproto') {
      return { name: channelName, ...drafts.telegram_mtproto };
    }
    if (kind === 'email') {
      return { name: channelName, ...drafts.email };
    }
    return { name: channelName, ...drafts.webhook };
  }, [channelName, drafts, kind]);

  return {
    channelName,
    drafts,
    kind,
    visibleSecrets,
    channelType,
    telegramMode,
    setChannelName,
    setChannelType,
    setTelegramMode,
    patchTelegramBot,
    patchTelegramMtproto,
    patchEmail,
    patchWebhook,
    toggleSecret,
    currentForm,
  };
};

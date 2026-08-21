import React from 'react';
import type { AlertChannelLanguage, AlertChannelType, AlertTelegramMode } from '@app-types/admin';
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

type FormState = {
  sourceKey: string;
  dirty: boolean;
  channelName: string;
  language: AlertChannelLanguage;
  kind: AlertChannelFormKind;
  drafts: ChannelDrafts;
  visibleSecrets: Record<SecretKey, boolean>;
};

const createFormState = (sourceKey: string, form?: AlertChannelForm): FormState => ({
  sourceKey,
  dirty: false,
  channelName: form?.name ?? '',
  language: form?.language ?? 'system',
  kind: form?.kind ?? DEFAULT_CHANNEL_KIND,
  drafts: draftsFor(form),
  visibleSecrets: createVisibleSecrets(),
});

export const useAlertChannelForm = ({
  isOpen,
  sourceKey,
  initialForm,
}: {
  isOpen: boolean;
  sourceKey: string;
  initialForm?: AlertChannelForm;
}) => {
  const [formState, setFormState] = React.useState(() => createFormState(sourceKey, initialForm));
  const wasOpenRef = React.useRef(isOpen);

  React.useEffect(() => {
    const justOpened = isOpen && !wasOpenRef.current;
    wasOpenRef.current = isOpen;

    if (!isOpen) return;
    setFormState((current) => {
      if (justOpened || current.sourceKey !== sourceKey) {
        return createFormState(sourceKey, initialForm);
      }
      if (current.dirty) return current;
      return createFormState(sourceKey, initialForm);
    });
  }, [initialForm, isOpen, sourceKey]);

  const setChannelNameDraft = React.useCallback((nextName: string) => {
    setFormState((current) => ({ ...current, dirty: true, channelName: nextName }));
  }, []);

  const setLanguage = React.useCallback((language: AlertChannelLanguage) => {
    setFormState((current) => ({ ...current, dirty: true, language }));
  }, []);

  const setChannelType = React.useCallback((nextType: AlertChannelType) => {
    setFormState((current) => {
      const nextKind =
        nextType !== 'telegram'
          ? nextType
          : current.kind === 'telegram_mtproto'
            ? 'telegram_mtproto'
            : 'telegram_bot';
      return { ...current, dirty: true, kind: nextKind };
    });
  }, []);

  const setTelegramMode = React.useCallback((nextMode: AlertTelegramMode) => {
    setFormState((current) => ({
      ...current,
      dirty: true,
      kind: nextMode === 'mtproto' ? 'telegram_mtproto' : 'telegram_bot',
    }));
  }, []);

  const patchTelegramBot = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['telegram_bot'], 'kind'>>) => {
      setFormState((current) => ({
        ...current,
        dirty: true,
        drafts: {
          ...current.drafts,
          telegram_bot: { ...current.drafts.telegram_bot, ...patch },
        },
      }));
    },
    [],
  );

  const patchTelegramMtproto = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['telegram_mtproto'], 'kind'>>) => {
      setFormState((current) => ({
        ...current,
        dirty: true,
        drafts: {
          ...current.drafts,
          telegram_mtproto: { ...current.drafts.telegram_mtproto, ...patch },
        },
      }));
    },
    [],
  );

  const patchEmail = React.useCallback((patch: Partial<Omit<ChannelDrafts['email'], 'kind'>>) => {
    setFormState((current) => ({
      ...current,
      dirty: true,
      drafts: {
        ...current.drafts,
        email: { ...current.drafts.email, ...patch },
      },
    }));
  }, []);

  const patchWebhook = React.useCallback(
    (patch: Partial<Omit<ChannelDrafts['webhook'], 'kind'>>) => {
      setFormState((current) => ({
        ...current,
        dirty: true,
        drafts: {
          ...current.drafts,
          webhook: { ...current.drafts.webhook, ...patch },
        },
      }));
    },
    [],
  );

  const toggleSecret = React.useCallback((key: SecretKey) => {
    setFormState((current) => ({
      ...current,
      visibleSecrets: {
        ...current.visibleSecrets,
        [key]: !current.visibleSecrets[key],
      },
    }));
  }, []);

  const { channelName, language, drafts, kind, visibleSecrets } = formState;
  const channelType = channelTypeFromKind(kind);
  const telegramMode = telegramModeFromKind(kind);

  const currentForm = React.useCallback((): AlertChannelForm => {
    if (kind === 'telegram_bot') {
      return { name: channelName, language, ...drafts.telegram_bot };
    }
    if (kind === 'telegram_mtproto') {
      return { name: channelName, language, ...drafts.telegram_mtproto };
    }
    if (kind === 'email') {
      return { name: channelName, language, ...drafts.email };
    }
    return { name: channelName, language, ...drafts.webhook };
  }, [channelName, drafts, kind, language]);

  return {
    channelName,
    language,
    drafts,
    kind,
    visibleSecrets,
    channelType,
    telegramMode,
    setChannelName: setChannelNameDraft,
    setLanguage,
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

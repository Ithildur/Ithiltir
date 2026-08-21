import type {
  AlertChannelLanguage,
  AlertChannelType,
  AlertTelegramMode,
  ValidAlertChannel,
} from '@app-types/admin';
import type { AlertChannelInput } from '@lib/adminApi';

export type AlertChannelFormKind = 'telegram_bot' | 'telegram_mtproto' | 'email' | 'webhook';

interface Base {
  name: string;
  language: AlertChannelLanguage;
}

interface TelegramBotChannelForm extends Base {
  kind: 'telegram_bot';
  botToken: string;
  chatId: string;
}

interface TelegramMtprotoChannelForm extends Base {
  kind: 'telegram_mtproto';
  apiId: string;
  apiHash: string;
  phoneNumber: string;
  chatId: string;
}

interface EmailChannelForm extends Base {
  kind: 'email';
  emailHost: string;
  emailPort: string;
  emailUsername: string;
  emailPassword: string;
  emailFrom: string;
  emailRecipients: string;
  emailUseTls: boolean;
}

interface WebhookChannelForm extends Base {
  kind: 'webhook';
  webhookUrl: string;
  webhookSecret: string;
}

export type AlertChannelForm =
  TelegramBotChannelForm | TelegramMtprotoChannelForm | EmailChannelForm | WebhookChannelForm;

export type AlertChannelFormIssue = 'name' | 'telegram_api_id' | 'email_port';

type AlertChannelFormResult =
  { ok: true; input: AlertChannelInput } | { ok: false; issue: AlertChannelFormIssue };

export interface ChannelDrafts {
  telegram_bot: Omit<TelegramBotChannelForm, 'name' | 'language'>;
  telegram_mtproto: Omit<TelegramMtprotoChannelForm, 'name' | 'language'>;
  email: Omit<EmailChannelForm, 'name' | 'language'>;
  webhook: Omit<WebhookChannelForm, 'name' | 'language'>;
}

export const DEFAULT_CHANNEL_KIND: AlertChannelFormKind = 'telegram_bot';

const parseInteger = (value: string, min: number, max = Number.MAX_SAFE_INTEGER): number | null => {
  const text = value.trim();
  if (!/^\d+$/.test(text)) return null;
  const parsed = Number(text);
  if (!Number.isSafeInteger(parsed) || parsed < min || parsed > max) return null;
  return parsed;
};

const splitRecipients = (value: string): string[] =>
  value
    .split(/[\n,\uFF0C]/)
    .map((item) => item.trim())
    .filter(Boolean);

const emptyDrafts = (): ChannelDrafts => ({
  telegram_bot: {
    kind: 'telegram_bot',
    botToken: '',
    chatId: '',
  },
  telegram_mtproto: {
    kind: 'telegram_mtproto',
    apiId: '',
    apiHash: '',
    phoneNumber: '',
    chatId: '',
  },
  email: {
    kind: 'email',
    emailHost: '',
    emailPort: '',
    emailUsername: '',
    emailPassword: '',
    emailFrom: '',
    emailRecipients: '',
    emailUseTls: true,
  },
  webhook: {
    kind: 'webhook',
    webhookUrl: '',
    webhookSecret: '',
  },
});

export const draftsFor = (form?: AlertChannelForm): ChannelDrafts => {
  const drafts = emptyDrafts();
  if (!form) return drafts;

  if (form.kind === 'telegram_bot') {
    drafts.telegram_bot = {
      kind: 'telegram_bot',
      botToken: form.botToken,
      chatId: form.chatId,
    };
  } else if (form.kind === 'telegram_mtproto') {
    drafts.telegram_mtproto = {
      kind: 'telegram_mtproto',
      apiId: form.apiId,
      apiHash: form.apiHash,
      phoneNumber: form.phoneNumber,
      chatId: form.chatId,
    };
  } else if (form.kind === 'email') {
    drafts.email = {
      kind: 'email',
      emailHost: form.emailHost,
      emailPort: form.emailPort,
      emailUsername: form.emailUsername,
      emailPassword: form.emailPassword,
      emailFrom: form.emailFrom,
      emailRecipients: form.emailRecipients,
      emailUseTls: form.emailUseTls,
    };
  } else {
    drafts.webhook = {
      kind: 'webhook',
      webhookUrl: form.webhookUrl,
      webhookSecret: form.webhookSecret,
    };
  }

  return drafts;
};

export const channelTypeFromKind = (kind: AlertChannelFormKind): AlertChannelType =>
  kind === 'telegram_bot' || kind === 'telegram_mtproto' ? 'telegram' : kind;

export const telegramModeFromKind = (kind: AlertChannelFormKind): AlertTelegramMode =>
  kind === 'telegram_mtproto' ? 'mtproto' : 'bot';

export const channelInputFromForm = (
  form: AlertChannelForm,
  enabled: boolean,
): AlertChannelFormResult => {
  const name = form.name.trim();
  if (!name) return { ok: false, issue: 'name' };

  if (form.kind === 'telegram_mtproto') {
    const apiId = parseInteger(form.apiId, 1);
    if (apiId === null) return { ok: false, issue: 'telegram_api_id' };

    return {
      ok: true,
      input: {
        name,
        type: 'telegram',
        enabled,
        config: {
          language: form.language,
          mode: 'mtproto',
          api_id: apiId,
          api_hash: form.apiHash,
          phone: form.phoneNumber.trim(),
          chat_id: form.chatId.trim(),
        },
      },
    };
  }

  if (form.kind === 'telegram_bot') {
    return {
      ok: true,
      input: {
        name,
        type: 'telegram',
        enabled,
        config: {
          language: form.language,
          mode: 'bot',
          bot_token: form.botToken,
          chat_id: form.chatId.trim(),
        },
      },
    };
  }

  if (form.kind === 'email') {
    const smtpPort = parseInteger(form.emailPort, 1, 65535);
    if (smtpPort === null) return { ok: false, issue: 'email_port' };

    return {
      ok: true,
      input: {
        name,
        type: 'email',
        enabled,
        config: {
          language: form.language,
          smtp_host: form.emailHost.trim(),
          smtp_port: smtpPort,
          username: form.emailUsername.trim(),
          password: form.emailPassword,
          from: form.emailFrom.trim(),
          to: splitRecipients(form.emailRecipients),
          use_tls: form.emailUseTls,
        },
      },
    };
  }

  const secret = form.webhookSecret;
  return {
    ok: true,
    input: {
      name,
      type: 'webhook',
      enabled,
      config: {
        language: form.language,
        url: form.webhookUrl.trim(),
        ...(secret !== '' ? { secret } : {}),
      },
    },
  };
};

export const formFromChannel = (channel: ValidAlertChannel): AlertChannelForm => {
  const base = { name: channel.name, language: channel.config.language };

  if (channel.type === 'telegram') {
    const config = channel.config;
    if (config.mode === 'mtproto') {
      return {
        ...base,
        kind: 'telegram_mtproto',
        apiId: String(config.api_id),
        apiHash: '',
        phoneNumber: config.phone,
        chatId: config.chat_id,
      };
    }
    return {
      ...base,
      kind: 'telegram_bot',
      botToken: '',
      chatId: config.chat_id,
    };
  }

  if (channel.type === 'email') {
    const config = channel.config;
    return {
      ...base,
      kind: 'email',
      emailHost: config.smtp_host,
      emailPort: String(config.smtp_port),
      emailUsername: config.username,
      emailPassword: '',
      emailFrom: config.from,
      emailRecipients: config.to.join(', '),
      emailUseTls: config.use_tls,
    };
  }

  const config = channel.config;
  return {
    ...base,
    kind: 'webhook',
    webhookUrl: config.url,
    webhookSecret: '',
  };
};

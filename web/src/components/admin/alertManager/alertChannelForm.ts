import type {
  EmailViewConfig,
  AlertChannel,
  TelegramBotViewConfig,
  TelegramMtprotoViewConfig,
  AlertChannelType,
  WebhookViewConfig,
  AlertTelegramMode,
} from '@app-types/admin';
import type { AlertChannelInput } from '@lib/adminApi';

export type AlertChannelFormKind = 'telegram_bot' | 'telegram_mtproto' | 'email' | 'webhook';

interface Base {
  name: string;
}

export interface TelegramBotChannelForm extends Base {
  kind: 'telegram_bot';
  botToken: string;
  chatId: string;
}

export interface TelegramMtprotoChannelForm extends Base {
  kind: 'telegram_mtproto';
  apiId: string;
  apiHash: string;
  phoneNumber: string;
  chatId: string;
}

export interface EmailChannelForm extends Base {
  kind: 'email';
  emailHost: string;
  emailPort: string;
  emailUsername: string;
  emailPassword: string;
  emailFrom: string;
  emailRecipients: string;
  emailUseTls: boolean;
}

export interface WebhookChannelForm extends Base {
  kind: 'webhook';
  webhookUrl: string;
  webhookSecret: string;
}

export type AlertChannelForm =
  | TelegramBotChannelForm
  | TelegramMtprotoChannelForm
  | EmailChannelForm
  | WebhookChannelForm;

export interface ChannelDrafts {
  telegram_bot: Omit<TelegramBotChannelForm, 'name'>;
  telegram_mtproto: Omit<TelegramMtprotoChannelForm, 'name'>;
  email: Omit<EmailChannelForm, 'name'>;
  webhook: Omit<WebhookChannelForm, 'name'>;
}

export const DEFAULT_CHANNEL_KIND: AlertChannelFormKind = 'telegram_bot';

const toNumber = (value: string): number => {
  const parsed = Number.parseInt(value, 10);
  return Number.isNaN(parsed) ? 0 : parsed;
};

const splitRecipients = (value: string): string[] =>
  value
    .split(/[\n,\uFF0C]/)
    .map((item) => item.trim())
    .filter(Boolean);

export const emptyDrafts = (): ChannelDrafts => ({
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
): AlertChannelInput => {
  const name = form.name.trim();

  if (form.kind === 'telegram_mtproto') {
    return {
      name,
      type: 'telegram',
      enabled,
      config: {
        mode: 'mtproto',
        api_id: toNumber(form.apiId),
        api_hash: form.apiHash.trim(),
        phone: form.phoneNumber.trim(),
        chat_id: form.chatId.trim(),
      },
    };
  }

  if (form.kind === 'telegram_bot') {
    return {
      name,
      type: 'telegram',
      enabled,
      config: {
        mode: 'bot',
        bot_token: form.botToken.trim(),
        chat_id: form.chatId.trim(),
      },
    };
  }

  if (form.kind === 'email') {
    return {
      name,
      type: 'email',
      enabled,
      config: {
        smtp_host: form.emailHost.trim(),
        smtp_port: toNumber(form.emailPort),
        username: form.emailUsername.trim(),
        password: form.emailPassword,
        from: form.emailFrom.trim(),
        to: splitRecipients(form.emailRecipients),
        use_tls: form.emailUseTls,
      },
    };
  }

  const secret = form.webhookSecret.trim();
  return {
    name,
    type: 'webhook',
    enabled,
    config: {
      url: form.webhookUrl.trim(),
      ...(secret !== '' ? { secret } : {}),
    },
  };
};

export const formFromChannel = (channel: AlertChannel): AlertChannelForm => {
  const base = { name: channel.name };

  if (channel.type === 'telegram') {
    const config = channel.config as TelegramBotViewConfig | TelegramMtprotoViewConfig;
    if (config.mode === 'mtproto') {
      return {
        ...base,
        kind: 'telegram_mtproto',
        apiId: String(config.api_id ?? ''),
        apiHash: '',
        phoneNumber: config.phone ?? '',
        chatId: config.chat_id ?? '',
      };
    }
    return {
      ...base,
      kind: 'telegram_bot',
      botToken: '',
      chatId: config.chat_id ?? '',
    };
  }

  if (channel.type === 'email') {
    const config = channel.config as EmailViewConfig;
    return {
      ...base,
      kind: 'email',
      emailHost: config.smtp_host ?? '',
      emailPort: config.smtp_port ? String(config.smtp_port) : '',
      emailUsername: config.username ?? '',
      emailPassword: '',
      emailFrom: config.from ?? '',
      emailRecipients: config.to?.join(', ') ?? '',
      emailUseTls: config.use_tls ?? true,
    };
  }

  const config = channel.config as WebhookViewConfig;
  return {
    ...base,
    kind: 'webhook',
    webhookUrl: config.url ?? '',
    webhookSecret: '',
  };
};

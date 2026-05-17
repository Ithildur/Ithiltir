import type { NodeVersion } from './api';
import type { SiteBrand } from './site';
import type { TrafficCycleMode } from './traffic';

export type DashboardTab = 'nodes' | 'groups' | 'alerts' | 'system';

export type ISODateString = string;

export type ThemeShell = 'sidebar' | 'topbar';
export type ThemeFrame = 'layered' | 'flat';
export type ThemeSummary = 'cards' | 'strip';
export type ThemeDensity = 'comfortable' | 'compact';

export interface ThemeAdminSpec {
  shell: ThemeShell;
  frame: ThemeFrame;
}

export interface ThemeDashboardSpec {
  summary: ThemeSummary;
  density: ThemeDensity;
}

export interface ThemeSpec {
  admin: ThemeAdminSpec;
  dashboard: ThemeDashboardSpec;
}

export interface ThemeManifest {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  skin: ThemeSpec;
}

export interface ThemePackage {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  skin: ThemeSpec;
  built_in: boolean;
  active: boolean;
  deletable: boolean;
  missing?: boolean;
  broken?: boolean;
  has_preview: boolean;
  created_at: ISODateString | null;
  updated_at: ISODateString | null;
}

export type HistoryGuestAccessMode = 'disabled' | 'by_node';

export interface SystemSettings extends SiteBrand {
  history_guest_access_mode: HistoryGuestAccessMode;
}

export interface NodeRow {
  id: number;
  name: string;
  hostname: string;
  ip?: string | null;
  groupIds: number[];
  groupNames: string[];
  secret: string;
  tags: string[];
  version: NodeVersion;
  guestVisible: boolean;
  trafficP95Enabled: boolean;
  trafficCycleMode: 'default' | TrafficCycleMode;
  trafficBillingStartDay: number;
  trafficBillingAnchorDate: string;
  trafficBillingTimezone: string;
  displayOrder: number;
}

export type AlertRuleOperator = '>' | '>=' | '<' | '<=' | '==' | '!=';
export type AlertRuleThresholdMode = 'static' | 'core_plus';

export interface AlertRuleInput {
  name: string;
  enabled: boolean;
  metric: string;
  operator: AlertRuleOperator;
  threshold: number;
  duration_sec: number;
  cooldown_min: number;
  threshold_mode: AlertRuleThresholdMode;
  threshold_offset: number;
}

export interface AlertRule extends AlertRuleInput {
  id: number;
  created_at: ISODateString;
  updated_at: ISODateString;
}

export type AlertChannelType = 'telegram' | 'email' | 'webhook';
export type AlertTelegramMode = 'bot' | 'mtproto';

export interface AlertMountRule {
  id: number;
  name: string;
  metric: string;
  builtin: boolean;
  enabled: boolean;
  default_mounted: boolean;
}

export interface AlertMountState {
  rule_id: number;
  mounted: boolean;
}

export interface AlertMountNode {
  id: number;
  name: string;
  hostname: string;
  ip?: string | null;
  group_ids: number[];
  mounts: AlertMountState[];
}

export interface AlertMounts {
  rules: AlertMountRule[];
  nodes: AlertMountNode[];
}

export interface TelegramBotConfig {
  mode?: 'bot';
  chat_id: string;
  bot_token: string;
}

export interface TelegramMtprotoConfig {
  mode: 'mtproto';
  api_id: number;
  phone: string;
  chat_id: string;
  api_hash: string;
  session?: string;
  username?: string;
}

export interface EmailConfig {
  smtp_host: string;
  smtp_port: number;
  username: string;
  password: string;
  from: string;
  to: string[];
  use_tls: boolean;
}

export interface WebhookConfig {
  url: string;
  secret?: string;
}

export type ChannelConfigInput =
  | TelegramBotConfig
  | TelegramMtprotoConfig
  | EmailConfig
  | WebhookConfig;

export interface TelegramBotViewConfig {
  mode?: 'bot';
  chat_id: string;
}

export interface TelegramMtprotoViewConfig {
  mode: 'mtproto';
  api_id: number;
  phone: string;
  chat_id: string;
  username?: string;
}

export interface EmailViewConfig {
  smtp_host: string;
  smtp_port: number;
  username: string;
  from: string;
  to: string[];
  use_tls: boolean;
}

export interface WebhookViewConfig {
  url: string;
}

export type ChannelConfig =
  | TelegramBotViewConfig
  | TelegramMtprotoViewConfig
  | EmailViewConfig
  | WebhookViewConfig;

export interface AlertChannel {
  id: number;
  name: string;
  type: AlertChannelType;
  config: ChannelConfig;
  enabled: boolean;
  created_at: ISODateString;
  updated_at: ISODateString;
}

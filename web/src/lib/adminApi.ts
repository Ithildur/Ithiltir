import type { Group, NodeDeploy, ManagedNode, UpdateNodeInput } from '@app-types/api';
import type {
  AlertChannel,
  AlertRule,
  AlertRuleInput,
  AlertMounts,
  EmailConfig,
  SystemSettings,
  TelegramBotConfig,
  TelegramMtprotoConfig,
  ThemePackage,
  WebhookConfig,
} from '@app-types/admin';
import { apiFetch } from './api';

export const fetchGroupList = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<Group[]>('/admin/groups', {
    method: 'GET',
    signal: params.signal,
  });

export const createGroup = (input: { name: string; remark?: string }) =>
  apiFetch('/admin/groups', {
    method: 'POST',
    json: input,
    responseType: 'empty',
  });

export const updateGroup = (id: number, input: { name?: string; remark?: string }) =>
  apiFetch(`/admin/groups/${id}`, {
    method: 'PATCH',
    json: input,
    responseType: 'empty',
  });

export const deleteGroup = (id: number) =>
  apiFetch(`/admin/groups/${id}`, {
    method: 'DELETE',
    responseType: 'empty',
  });

export const fetchNodes = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<ManagedNode[]>('/admin/nodes', {
    method: 'GET',
    signal: params.signal,
  });

export const createNode = () =>
  apiFetch('/admin/nodes', {
    method: 'POST',
    responseType: 'empty',
  });

export const updateNode = (id: number, input: UpdateNodeInput) =>
  apiFetch(`/admin/nodes/${id}`, {
    method: 'PATCH',
    json: input,
    responseType: 'empty',
  });

export const requestNodeUpgrade = (id: number) =>
  apiFetch(`/admin/nodes/${id}/upgrade`, {
    method: 'POST',
    responseType: 'empty',
  });

export interface NodeTrafficRebuildStatus {
  server_id: number;
  status: 'idle' | 'running' | 'completed' | 'failed';
  running: boolean;
  code?: string;
  started_at?: string;
  finished_at?: string;
  error?: string;
}

export const fetchTrafficRebuild = (signal?: AbortSignal) =>
  apiFetch<NodeTrafficRebuildStatus>('/admin/nodes/traffic/rebuild', {
    method: 'GET',
    signal,
  });

export const rebuildNodeTraffic = (id: number, signal?: AbortSignal) =>
  apiFetch<NodeTrafficRebuildStatus>(`/admin/nodes/${id}/traffic/rebuild`, {
    method: 'POST',
    signal,
  });

export const deleteNode = (id: number) =>
  apiFetch(`/admin/nodes/${id}`, {
    method: 'DELETE',
    responseType: 'empty',
  });

export const fetchNodeDeploy = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<NodeDeploy>('/admin/nodes/deploy', {
    method: 'GET',
    signal: params.signal,
  });

export const updateNodesDisplayOrder = (ids: number[]) =>
  apiFetch('/admin/nodes/display-order', {
    method: 'PUT',
    json: { ids },
    responseType: 'empty',
  });

export const updateNodesTrafficP95 = (ids: number[], enabled: boolean) =>
  apiFetch('/admin/nodes/traffic-p95', {
    method: 'PATCH',
    json: { ids, enabled },
    responseType: 'empty',
  });

export const fetchAlertRules = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<AlertRule[]>('/admin/alerts/rules', {
    method: 'GET',
    signal: params.signal,
  });

export type CreateAlertRuleInput = AlertRuleInput;
export type UpdateAlertRuleInput = Partial<AlertRuleInput>;

export const createAlertRule = (input: CreateAlertRuleInput) =>
  apiFetch('/admin/alerts/rules', {
    method: 'POST',
    json: input,
    responseType: 'empty',
  });

export const updateAlertRule = (id: number, input: UpdateAlertRuleInput) =>
  apiFetch(`/admin/alerts/rules/${id}`, {
    method: 'PATCH',
    json: input,
    responseType: 'empty',
  });

export const deleteAlertRule = (id: number) =>
  apiFetch(`/admin/alerts/rules/${id}`, {
    method: 'DELETE',
    responseType: 'empty',
  });

export const fetchAlertMounts = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<AlertMounts>('/admin/alerts/mounts', {
    method: 'GET',
    signal: params.signal,
  });

export const updateAlertMounts = (input: {
  rule_ids: number[];
  server_ids: number[];
  mounted: boolean;
}) =>
  apiFetch('/admin/alerts/mounts', {
    method: 'PUT',
    json: input,
    responseType: 'empty',
  });

interface BaseAlertChannelInput {
  name: string;
  enabled: boolean;
}

export type AlertChannelInput =
  | (BaseAlertChannelInput & {
      type: 'telegram';
      config: TelegramBotConfig | TelegramMtprotoConfig;
    })
  | (BaseAlertChannelInput & {
      type: 'email';
      config: EmailConfig;
    })
  | (BaseAlertChannelInput & {
      type: 'webhook';
      config: WebhookConfig;
    });

export const fetchAlertChannels = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<AlertChannel[]>('/admin/alerts/channels', {
    method: 'GET',
    signal: params.signal,
  });

export const createAlertChannel = (input: AlertChannelInput) =>
  apiFetch('/admin/alerts/channels', {
    method: 'POST',
    json: input,
    responseType: 'empty',
  });

export const updateAlertChannel = (id: number, input: AlertChannelInput) =>
  apiFetch(`/admin/alerts/channels/${id}`, {
    method: 'PUT',
    json: input,
    responseType: 'empty',
  });

export const updateAlertChannelEnabled = (id: number, input: { enabled: boolean }) =>
  apiFetch(`/admin/alerts/channels/${id}/enabled`, {
    method: 'PUT',
    json: input,
    responseType: 'empty',
  });

export const testAlertChannel = (id: number, input: { title?: string; message?: string } = {}) =>
  apiFetch(`/admin/alerts/channels/${id}/test`, {
    method: 'POST',
    json: input,
    responseType: 'empty',
  });

export const deleteAlertChannel = (id: number) =>
  apiFetch(`/admin/alerts/channels/${id}`, {
    method: 'DELETE',
    responseType: 'empty',
  });

export interface AlertMtprotoCodeResult {
  login_id: string;
  timeout: number;
}

export interface AlertMtprotoVerifyResult {
  password_required: boolean;
}

export interface AlertMtprotoPingResult {
  valid: boolean;
  reason?: 'not_logged_in' | 'invalid_session';
}

export const requestAlertMtprotoCode = (channelId: number) =>
  apiFetch<AlertMtprotoCodeResult>('/admin/alerts/channels/telegram/mtproto/code', {
    method: 'POST',
    json: { channel_id: channelId },
  });

export const verifyAlertMtprotoCode = (input: { login_id: string; code: string }) =>
  apiFetch<AlertMtprotoVerifyResult>('/admin/alerts/channels/telegram/mtproto/verify', {
    method: 'POST',
    json: input,
    responseType: 'jsonOrEmpty',
  });

export const submitAlertMtprotoPassword = (input: { login_id: string; password: string }) =>
  apiFetch('/admin/alerts/channels/telegram/mtproto/password', {
    method: 'POST',
    json: input,
    responseType: 'empty',
  });

export const pingAlertMtproto = (channelId: number) =>
  apiFetch<AlertMtprotoPingResult>('/admin/alerts/channels/telegram/mtproto/ping', {
    method: 'POST',
    json: { channel_id: channelId },
  });

export const fetchSystemSettings = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<SystemSettings>('/admin/system/settings', {
    method: 'GET',
    signal: params.signal,
  });

export const updateSystemSettings = (input: Partial<SystemSettings>) =>
  apiFetch('/admin/system/settings', {
    method: 'PATCH',
    json: input,
    responseType: 'empty',
  });

export const fetchThemePackages = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<ThemePackage[]>('/admin/system/themes', {
    method: 'GET',
    signal: params.signal,
  });

export const uploadThemePackage = (file: File) => {
  const form = new FormData();
  form.append('file', file);
  return apiFetch<ThemePackage>('/admin/system/themes/upload', {
    method: 'POST',
    body: form,
  });
};

export const applyThemePackage = (id: string) =>
  apiFetch(`/admin/system/themes/${id}/apply`, {
    method: 'POST',
    responseType: 'empty',
  });

export const deleteThemePackage = (id: string) =>
  apiFetch(`/admin/system/themes/${id}`, {
    method: 'DELETE',
    responseType: 'empty',
  });

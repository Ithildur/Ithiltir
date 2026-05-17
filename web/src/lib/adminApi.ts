import type { Group, NodeDeploy, ManagedNode } from '@app-types/api';
import type {
  AlertChannel,
  AlertChannelType,
  ChannelConfigInput,
  AlertRule,
  AlertRuleInput,
  AlertMounts,
  SystemSettings,
  ThemePackage,
} from '@app-types/admin';
import type { TrafficCycleMode } from '@app-types/traffic';
import { apiFetch } from './api';

export const fetchGroupList = () =>
  apiFetch<Group[]>('/admin/groups', {
    method: 'GET',
  });

export const createGroup = (input: { name: string; remark?: string }) =>
  apiFetch<void>('/admin/groups', {
    method: 'POST',
    json: input,
  });

export const updateGroup = (id: number, input: { name?: string; remark?: string }) =>
  apiFetch<void>(`/admin/groups/${id}`, {
    method: 'PATCH',
    json: input,
  });

export const deleteGroup = (id: number) =>
  apiFetch<void>(`/admin/groups/${id}`, {
    method: 'DELETE',
  });

export const fetchNodes = () =>
  apiFetch<ManagedNode[]>('/admin/nodes', {
    method: 'GET',
  });

export interface UpdateNodeInput {
  name?: string;
  is_guest_visible?: boolean;
  traffic_p95_enabled?: boolean;
  traffic_cycle_mode?: 'default' | TrafficCycleMode;
  traffic_billing_start_day?: number;
  traffic_billing_anchor_date?: string;
  traffic_billing_timezone?: string;
  display_order?: number;
  tags?: string[];
  secret?: string;
  group_ids?: number[];
}

export const createNode = () =>
  apiFetch<void>('/admin/nodes', {
    method: 'POST',
  });

export const updateNode = (id: number, input: UpdateNodeInput) =>
  apiFetch<void>(`/admin/nodes/${id}`, {
    method: 'PATCH',
    json: input,
  });

export const requestNodeUpgrade = (id: number) =>
  apiFetch<void>(`/admin/nodes/${id}/upgrade`, {
    method: 'POST',
  });

export const deleteNode = (id: number) =>
  apiFetch<void>(`/admin/nodes/${id}`, {
    method: 'DELETE',
  });

export const fetchNodeDeploy = () =>
  apiFetch<NodeDeploy>('/admin/nodes/deploy', {
    method: 'GET',
  });

export const updateNodesDisplayOrder = (ids: number[]) =>
  apiFetch<void>('/admin/nodes/display-order', {
    method: 'PUT',
    json: { ids },
  });

export const updateNodesTrafficP95 = (ids: number[], enabled: boolean) =>
  apiFetch<void>('/admin/nodes/traffic-p95', {
    method: 'PATCH',
    json: { ids, enabled },
  });

export const fetchAlertRules = () =>
  apiFetch<AlertRule[]>('/admin/alerts/rules', {
    method: 'GET',
  });

export type CreateAlertRuleInput = AlertRuleInput;
export type UpdateAlertRuleInput = Partial<AlertRuleInput>;

export const createAlertRule = (input: CreateAlertRuleInput) =>
  apiFetch<void>('/admin/alerts/rules', {
    method: 'POST',
    json: input,
  });

export const updateAlertRule = (id: number, input: UpdateAlertRuleInput) =>
  apiFetch<void>(`/admin/alerts/rules/${id}`, {
    method: 'PATCH',
    json: input,
  });

export const deleteAlertRule = (id: number) =>
  apiFetch<void>(`/admin/alerts/rules/${id}`, {
    method: 'DELETE',
  });

export const fetchAlertMounts = () =>
  apiFetch<AlertMounts>('/admin/alerts/mounts', {
    method: 'GET',
  });

export const updateAlertMounts = (input: {
  rule_ids: number[];
  server_ids: number[];
  mounted: boolean;
}) =>
  apiFetch<void>('/admin/alerts/mounts', {
    method: 'PUT',
    json: input,
  });

export interface AlertChannelInput {
  name: string;
  type: AlertChannelType;
  config: ChannelConfigInput;
  enabled: boolean;
}

export const fetchAlertChannels = () =>
  apiFetch<AlertChannel[]>('/admin/alerts/channels', {
    method: 'GET',
  });

export const fetchAlertChannel = (id: number) =>
  apiFetch<AlertChannel>(`/admin/alerts/channels/${id}`, {
    method: 'GET',
  });

export const createAlertChannel = (input: AlertChannelInput) =>
  apiFetch<void>('/admin/alerts/channels', {
    method: 'POST',
    json: input,
  });

export const updateAlertChannel = (id: number, input: AlertChannelInput) =>
  apiFetch<void>(`/admin/alerts/channels/${id}`, {
    method: 'PUT',
    json: input,
  });

export const updateAlertChannelEnabled = (id: number, input: { enabled: boolean }) =>
  apiFetch<void>(`/admin/alerts/channels/${id}/enabled`, {
    method: 'PUT',
    json: input,
  });

export const testAlertChannel = (id: number, input: { title?: string; message?: string } = {}) =>
  apiFetch<void>(`/admin/alerts/channels/${id}/test`, {
    method: 'POST',
    json: input,
  });

export const deleteAlertChannel = (id: number) =>
  apiFetch<void>(`/admin/alerts/channels/${id}`, {
    method: 'DELETE',
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
  apiFetch<AlertMtprotoVerifyResult | void>('/admin/alerts/channels/telegram/mtproto/verify', {
    method: 'POST',
    json: input,
  });

export const submitAlertMtprotoPassword = (input: { login_id: string; password: string }) =>
  apiFetch<void>('/admin/alerts/channels/telegram/mtproto/password', {
    method: 'POST',
    json: input,
  });

export const pingAlertMtproto = (channelId: number) =>
  apiFetch<AlertMtprotoPingResult>('/admin/alerts/channels/telegram/mtproto/ping', {
    method: 'POST',
    json: { channel_id: channelId },
  });

export const fetchSystemSettings = () =>
  apiFetch<SystemSettings>('/admin/system/settings', {
    method: 'GET',
  });

export const updateSystemSettings = (input: Partial<SystemSettings>) =>
  apiFetch<void>('/admin/system/settings', {
    method: 'PATCH',
    json: input,
  });

export const fetchThemePackages = () =>
  apiFetch<ThemePackage[]>('/admin/system/themes', {
    method: 'GET',
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
  apiFetch<void>(`/admin/system/themes/${id}/apply`, {
    method: 'POST',
  });

export const deleteThemePackage = (id: string) =>
  apiFetch<void>(`/admin/system/themes/${id}`, {
    method: 'DELETE',
  });

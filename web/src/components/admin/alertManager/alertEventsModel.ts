import type { AlertEventStatusFilter, ISODateString } from '@app-types/admin';
import { formatLocalDateTime } from '@utils/time';

export type AlertEventRange = '24h' | '7d' | '30d' | 'all' | 'custom';

export interface AlertEventFilter {
  serverId: number;
  status: AlertEventStatusFilter;
  metric: string;
  range: AlertEventRange;
  fromLocal: string;
  toLocal: string;
}

export const defaultAlertEventFilter: AlertEventFilter = {
  serverId: 0,
  status: 'open',
  metric: '',
  range: '30d',
  fromLocal: '',
  toLocal: '',
};

const rangeMs: Record<Exclude<AlertEventRange, 'all' | 'custom'>, number> = {
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000,
};

const parsePositiveInt = (raw: string | null): number => {
  if (!raw) return 0;
  if (!/^\d+$/.test(raw)) return 0;
  const parsed = Number(raw);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 0;
};

const parseStatus = (raw: string | null): AlertEventStatusFilter => {
  return raw === 'closed' || raw === 'all' ? raw : 'open';
};

const parseRange = (raw: string | null): AlertEventRange => {
  return raw === '24h' || raw === '7d' || raw === '30d' || raw === 'all' || raw === 'custom'
    ? raw
    : '30d';
};

const toLocalInput = (value: string | null): string => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
};

const isoFromLocal = (value: string): ISODateString | undefined => {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
};

const timeRangeReady = (from: ISODateString | undefined, to: ISODateString | undefined): boolean =>
  Boolean(from && to && new Date(from).getTime() <= new Date(to).getTime());

export const alertEventFilterFromParams = (params: URLSearchParams): AlertEventFilter => {
  const range = parseRange(params.get('alert_range'));
  return {
    serverId: parsePositiveInt(params.get('alert_server_id')),
    status: parseStatus(params.get('alert_status')),
    metric: params.get('alert_metric')?.trim() ?? '',
    range,
    fromLocal: range === 'custom' ? toLocalInput(params.get('alert_from')) : '',
    toLocal: range === 'custom' ? toLocalInput(params.get('alert_to')) : '',
  };
};

export const alertEventFilterKey = (filter: AlertEventFilter): string =>
  [
    filter.serverId,
    filter.status,
    filter.metric,
    filter.range,
    filter.fromLocal,
    filter.toLocal,
  ].join('|');

export const urlParamsForAlertEventFilter = (
  current: URLSearchParams,
  filter: AlertEventFilter,
): URLSearchParams => {
  const next = new URLSearchParams(current);
  next.set('tab', 'alerts');
  next.set('alerts_tab', 'events');
  if (filter.serverId > 0) next.set('alert_server_id', String(filter.serverId));
  else next.delete('alert_server_id');
  next.set('alert_status', filter.status);
  if (filter.metric) next.set('alert_metric', filter.metric);
  else next.delete('alert_metric');
  next.set('alert_range', filter.range);
  const from = filter.range === 'custom' ? isoFromLocal(filter.fromLocal) : undefined;
  const to = filter.range === 'custom' ? isoFromLocal(filter.toLocal) : undefined;
  if (from) next.set('alert_from', from);
  else next.delete('alert_from');
  if (to) next.set('alert_to', to);
  else next.delete('alert_to');
  return next;
};

export const alertEventRequestBounds = (
  filter: AlertEventFilter,
): { from?: ISODateString; to?: ISODateString } => {
  const now = new Date();
  if (filter.range === 'all') return {};
  if (filter.range !== 'custom') {
    return {
      from: new Date(now.getTime() - rangeMs[filter.range]).toISOString(),
      to: now.toISOString(),
    };
  }
  return {
    from: isoFromLocal(filter.fromLocal),
    to: isoFromLocal(filter.toLocal),
  };
};

export const alertEventFilterReady = (filter: AlertEventFilter): boolean => {
  if (filter.range !== 'custom') return true;
  return timeRangeReady(isoFromLocal(filter.fromLocal), isoFromLocal(filter.toLocal));
};

export const alertEventCustomRangeLabel = (
  filter: AlertEventFilter,
  lang: 'zh' | 'en',
): string => {
  if (filter.range !== 'custom') return '';
  const from = isoFromLocal(filter.fromLocal);
  const to = isoFromLocal(filter.toLocal);
  if (!from || !to || !timeRangeReady(from, to)) return '';
  const options: Intl.DateTimeFormatOptions = {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  };
  return `${formatLocalDateTime(from, lang, options)} - ${formatLocalDateTime(to, lang, options)}`;
};

import { apiFetch } from './api';
import type {
  StatisticsAccess,
  TrafficDaily,
  TrafficIface,
  TrafficMonthly,
  TrafficPeriod,
  TrafficSettings,
  TrafficSummary,
} from '@app-types/traffic';

export const fetchStatisticsAccess = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<StatisticsAccess>('/statistics/access', {
    method: 'GET',
    signal: params.signal,
  });

export const fetchTrafficSettings = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<TrafficSettings>('/statistics/traffic/settings', {
    method: 'GET',
    signal: params.signal,
  });

export const updateTrafficSettings = (input: Partial<TrafficSettings>) =>
  apiFetch<void>('/statistics/traffic/settings', {
    method: 'PATCH',
    json: input,
  });

export const fetchTrafficIfaces = (params: { serverId: number; signal?: AbortSignal }) => {
  const search = new URLSearchParams({ server_id: String(params.serverId) });
  return apiFetch<TrafficIface[]>(`/statistics/traffic/ifaces?${search.toString()}`, {
    method: 'GET',
    signal: params.signal,
  });
};

export interface TrafficSummaryParams {
  serverId: number;
  iface: string;
  signal?: AbortSignal;
}

export interface TrafficPeriodParams extends TrafficSummaryParams {
  period?: TrafficPeriod;
}

const trafficSearch = (params: TrafficSummaryParams) =>
  new URLSearchParams({
    server_id: String(params.serverId),
    iface: params.iface,
  });

export const fetchTrafficSummary = (params: TrafficSummaryParams) =>
  apiFetch<TrafficSummary>(`/statistics/traffic/summary?${trafficSearch(params).toString()}`, {
    method: 'GET',
    signal: params.signal,
  });

export const fetchTrafficDaily = (params: TrafficPeriodParams) => {
  const search = trafficSearch(params);
  if (params.period !== undefined) search.set('period', params.period);
  return apiFetch<TrafficDaily>(`/statistics/traffic/daily?${search.toString()}`, {
    method: 'GET',
    signal: params.signal,
  });
};

export const fetchTrafficMonthly = (params: TrafficPeriodParams & { months?: number }) => {
  const search = trafficSearch(params);
  if (params.months !== undefined) search.set('months', String(params.months));
  if (params.period !== undefined) search.set('period', params.period);
  return apiFetch<TrafficMonthly>(`/statistics/traffic/monthly?${search.toString()}`, {
    method: 'GET',
    signal: params.signal,
  });
};

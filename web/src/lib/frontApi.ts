import { apiFetch } from './api';
import type { FrontMetricsResponse, GroupView, NodeView } from '@app-types/frontMetrics';

export const fetchFrontMetrics = (params: { signal?: AbortSignal } = {}) => {
  const path = '/front/metrics';
  return apiFetch<FrontMetricsResponse>(path, {
    method: 'GET',
    signal: params.signal,
  });
};

export const fetchFrontGroups = (params: { signal?: AbortSignal } = {}) => {
  return apiFetch<GroupView[]>('/front/groups', {
    method: 'GET',
    signal: params.signal,
  });
};

export const fetchFrontNode = (serverId: number, signal: AbortSignal) =>
  apiFetch<NodeView>(`/front/metrics/${serverId}`, { signal });

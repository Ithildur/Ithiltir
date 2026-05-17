import { apiFetch } from './api';
import type { GroupView, NodeView } from '@app-types/frontMetrics';

export const fetchFrontMetrics = (params: { signal?: AbortSignal } = {}) => {
  const path = '/front/metrics';
  return apiFetch<NodeView[]>(path, {
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

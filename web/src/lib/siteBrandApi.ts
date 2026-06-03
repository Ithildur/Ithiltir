import type { SiteBrand } from '@app-types/site';
import { apiFetch } from './api';

export const fetchSiteBrand = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<SiteBrand>('/front/brand', {
    method: 'GET',
    auth: 'none',
    retryOn401: false,
    signal: params.signal,
  });

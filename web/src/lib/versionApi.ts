import type { AppVersion } from '@app-types/api';
import { apiFetch } from './api';

export const fetchAppVersion = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<AppVersion>('/version', {
    method: 'GET',
    auth: 'none',
    retryOn401: false,
    signal: params.signal,
  });

import type { AppVersion } from '@app-types/api';
import { apiControlTimeoutMs, apiFetch } from './api';

export const fetchAppVersion = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<AppVersion>('/version', {
    method: 'GET',
    auth: 'none',
    retryOn401: false,
    signal: params.signal,
    timeoutMs: apiControlTimeoutMs,
  });

import type { AppVersion } from '@app-types/api';
import { apiControlTimeoutMs, apiFetch } from './api';

export const fetchAppVersion = (params: { signal?: AbortSignal; timeoutMs?: number } = {}) =>
  apiFetch<AppVersion>('/version', {
    method: 'GET',
    auth: 'none',
    retryOn401: false,
    signal: params.signal,
    timeoutMs: params.timeoutMs ?? apiControlTimeoutMs,
  });

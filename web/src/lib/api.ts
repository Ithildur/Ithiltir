import type { ErrorResponse } from '@app-types/api';
import { clearAuthState, getAuthState, getCsrfToken, patchAuthState } from './authStore';

const API_BASE = '/api';
const API_WARNING_HEADER = 'X-Dash-Warning';

export const API_WARNING_EVENT = 'dash:warning';

export interface ApiWarningDetail {
  code: string;
}

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: unknown;

  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export interface ApiRequestOptions extends RequestInit {
  json?: unknown;
  auth?: 'auto' | 'none';
  csrf?: 'auto' | 'none';
  retryOn401?: boolean;
}

const buildUrl = (path: string): string => {
  if (path.startsWith('http://') || path.startsWith('https://')) return path;
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${API_BASE}${normalized}`;
};

const normalizePath = (path: string): string => (path.startsWith('/') ? path : `/${path}`);
const isAbsoluteUrl = (path: string): boolean =>
  path.startsWith('http://') || path.startsWith('https://');

const shouldInjectCsrfHeader = (path: string): boolean => {
  if (isAbsoluteUrl(path)) return false;
  const normalized = normalizePath(path);
  return normalized === '/auth' || normalized.startsWith('/auth/');
};

const parseJsonTextSafe = (rawText: string): unknown | undefined => {
  if (!rawText.trim()) return undefined;
  try {
    return JSON.parse(rawText) as unknown;
  } catch {
    return undefined;
  }
};

const parseJsonText = (rawText: string): unknown => {
  if (!rawText.trim()) {
    throw new SyntaxError('empty JSON response');
  }
  return JSON.parse(rawText) as unknown;
};

const shouldTreatAsJson = (contentType: string): boolean =>
  contentType.toLowerCase().includes('application/json');

type RefreshResponse = {
  access_token: string;
  expires_at: string;
  csrf_token: string;
};

let refreshInFlight: Promise<void> | null = null;

const emitApiWarning = (code: string): void => {
  if (typeof window === 'undefined') return;

  const normalized = code.trim();
  if (!normalized) return;

  window.setTimeout(() => {
    window.dispatchEvent(
      new CustomEvent<ApiWarningDetail>(API_WARNING_EVENT, {
        detail: { code: normalized },
      }),
    );
  }, 0);
};

export const refreshSession = async (
  reason: 'bootstrap' | 'retry401' = 'retry401',
): Promise<void> => {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    if (reason === 'bootstrap') {
      const current = getAuthState();
      if (current.status === 'unknown') {
        patchAuthState({ status: 'bootstrapping' });
      }
    }

    try {
      const csrfToken = getCsrfToken();
      if (!csrfToken && import.meta.env.DEV) {
        // If csrf cookie is scoped to /api (or HttpOnly), document.cookie won't expose it,
        // and CSRF-protected endpoints like /api/auth/refresh will fail.
        console.warn(
          '[auth] Missing CSRF token for /api/auth/refresh. Ensure csrf cookie is readable by JS (not HttpOnly) and scoped to Path=/.',
        );
      }
      const headers = new Headers({ Accept: 'application/json' });
      if (csrfToken) headers.set('X-CSRF-Token', csrfToken);

      const response = await fetch(buildUrl('/auth/refresh'), {
        method: 'POST',
        credentials: 'include',
        headers,
      });

      const contentType = response.headers.get('content-type') ?? '';
      const rawText = await response.text();
      const parsed = shouldTreatAsJson(contentType)
        ? (parseJsonTextSafe(rawText) as unknown)
        : undefined;

      if (!response.ok) {
        const errorDetails = parsed ?? rawText;
        const errorMessage =
          typeof parsed === 'object' && parsed && 'message' in parsed
            ? String((parsed as { message?: unknown }).message ?? response.statusText)
            : (response.statusText ?? 'Request failed');
        throw new ApiError(errorMessage, response.status, undefined, errorDetails);
      }

      const data = parsed as RefreshResponse | undefined;
      if (!data?.access_token) {
        throw new ApiError('Invalid refresh response', 500, 'invalid_refresh_response', parsed);
      }

      patchAuthState({
        status: 'authenticated',
        accessToken: data.access_token,
        expiresAt: data.expires_at ?? null,
        // CSRF is rotated and stored in cookie; avoid caching in memory.
        csrfToken: null,
      });
    } catch (error) {
      // Only treat refresh 401 as "session is gone" and hard logout.
      if (error instanceof ApiError && error.status === 401) {
        clearAuthState();
      } else if (reason === 'bootstrap') {
        // Avoid getting stuck in "bootstrapping" state on transient errors.
        patchAuthState({ status: 'guest' });
      }
      throw error;
    }
  })().finally(() => {
    refreshInFlight = null;
  });

  return refreshInFlight;
};

const shouldAttemptRefresh = (path: string): boolean => {
  const normalized = normalizePath(path);
  if (
    normalized === '/auth/login' ||
    normalized === '/auth/refresh' ||
    normalized === '/auth/logout'
  )
    return false;
  return Boolean(getAuthState().accessToken);
};

const buildCredentials = (path: string, credentials: RequestCredentials | undefined) => {
  if (credentials) return credentials;
  return isAbsoluteUrl(path) ? 'same-origin' : 'include';
};

export const apiFetch = async <T = void>(
  path: string,
  options: ApiRequestOptions = {},
): Promise<T> => {
  const {
    json,
    auth,
    csrf,
    retryOn401,
    headers: headersInit,
    credentials,
    body,
    ...requestInit
  } = options;

  const doFetch = async (): Promise<Response> => {
    const headers = new Headers(headersInit ?? {});
    headers.set('Accept', 'application/json');

    if (json !== undefined) {
      headers.set('Content-Type', 'application/json');
    }

    const authMode = auth ?? 'auto';
    if (authMode === 'auto') {
      const { accessToken } = getAuthState();
      if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
    }

    const csrfMode = csrf ?? 'auto';
    if (csrfMode === 'auto') {
      if (shouldInjectCsrfHeader(path)) {
        const csrfToken = getCsrfToken();
        if (csrfToken) headers.set('X-CSRF-Token', csrfToken);
      }
    }

    return fetch(buildUrl(path), {
      ...requestInit,
      headers,
      credentials: buildCredentials(path, credentials),
      body: json !== undefined ? JSON.stringify(json) : body,
    });
  };

  let response = await doFetch();

  const shouldRetryOn401 = retryOn401 ?? true;
  if (shouldRetryOn401 && response.status === 401 && shouldAttemptRefresh(path)) {
    await refreshSession('retry401');
    response = await doFetch();
  }

  if (response.status === 204) {
    emitApiWarning(response.headers.get(API_WARNING_HEADER) ?? '');
    return undefined as T;
  }

  const contentType = response.headers.get('content-type') ?? '';
  const rawText = await response.text();

  if (!response.ok) {
    const parsed = shouldTreatAsJson(contentType)
      ? (parseJsonTextSafe(rawText) as ErrorResponse | undefined)
      : undefined;
    const message = parsed?.message ?? response.statusText ?? 'Request failed';
    throw new ApiError(message, response.status, parsed?.code, parsed ?? rawText);
  }

  emitApiWarning(response.headers.get(API_WARNING_HEADER) ?? '');

  if (shouldTreatAsJson(contentType)) {
    try {
      return parseJsonText(rawText) as T;
    } catch (error) {
      throw new ApiError('Invalid JSON response', response.status, 'invalid_json_response', {
        body: rawText,
        cause: error instanceof Error ? error.message : String(error),
      });
    }
  }

  return rawText as unknown as T;
};

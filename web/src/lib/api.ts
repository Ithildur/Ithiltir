import type { ErrorResponse } from '@app-types/api';
import type { AuthState } from '@app-types/auth';
import { getCsrfToken } from './authSession';

const API_BASE = '/api';
const API_WARNING_HEADER = 'X-Dash-Warning';

export const API_WARNING_EVENT = 'dash:warning';
export const apiControlTimeoutMs = 15_000;

interface ApiWarningDetail {
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

class ApiAuthStaleError extends Error {
  constructor() {
    super('Auth session changed');
    this.name = 'ApiAuthStaleError';
  }
}

export const isApiAuthStaleError = (error: unknown): error is ApiAuthStaleError =>
  error instanceof ApiAuthStaleError;

class ApiRuntimeError extends Error {
  code: string;

  constructor(message: string, code: string) {
    super(message);
    this.name = 'ApiRuntimeError';
    this.code = code;
  }
}

interface RequestTimeout {
  signal: AbortSignal | undefined;
  clear: () => void;
}

const withRequestTimeout = (
  signal: AbortSignal | null | undefined,
  timeoutMs: number | undefined,
): RequestTimeout => {
  if (timeoutMs === undefined) {
    return { signal: signal ?? undefined, clear: () => undefined };
  }
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    throw new ApiRuntimeError('API request timeout must be positive', 'api_invalid_timeout');
  }

  const controller = new AbortController();
  const abortFromCaller = () => controller.abort(signal?.reason);
  if (signal?.aborted) {
    abortFromCaller();
  } else {
    signal?.addEventListener('abort', abortFromCaller, { once: true });
  }
  const timeout = window.setTimeout(() => {
    controller.abort(new DOMException('Request timed out', 'TimeoutError'));
  }, timeoutMs);

  return {
    signal: controller.signal,
    clear: () => {
      window.clearTimeout(timeout);
      signal?.removeEventListener('abort', abortFromCaller);
    },
  };
};

type ApiRequestOptions = Omit<RequestInit, 'credentials'> & {
  credentials?: never;
  json?: unknown;
  auth?: 'auto' | 'none';
  csrf?: 'auto' | 'none';
  retryOn401?: boolean;
  responseType?: 'json' | 'text' | 'empty' | 'jsonOrEmpty';
  timeoutMs?: number;
};

const buildUrl = (path: string): string => {
  const normalized = path.startsWith('/') ? path : `/${path}`;
  return `${API_BASE}${normalized}`;
};

const normalizePath = (path: string): string => (path.startsWith('/') ? path : `/${path}`);

const shouldInjectCsrfHeader = (path: string): boolean => {
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

interface ApiAuthSession {
  getState: () => AuthState;
  patch: (patch: Partial<AuthState>) => void;
  expire: () => void;
  currentGeneration: () => number;
  isGenerationCurrent: (generation: number) => boolean;
}

let authSession: ApiAuthSession | null = null;

export const bindApiAuthSession = (session: ApiAuthSession): void => {
  authSession = session;
};

const requireAuthSession = (): ApiAuthSession => {
  if (!authSession) {
    throw new ApiRuntimeError('API auth session is not bound', 'api_auth_session_unbound');
  }
  return authSession;
};

const isSafeMethod = (method: string): boolean =>
  method === 'GET' || method === 'HEAD' || method === 'OPTIONS';

const needsBoundAuthSession = (path: string, method: string): boolean => {
  const normalized = normalizePath(path);
  if (normalized.startsWith('/admin')) return true;
  if (normalized === '/auth/login') return false;
  return !isSafeMethod(method);
};

let refreshInFlight: { generation: number; promise: Promise<void> } | null = null;

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
  const session = requireAuthSession();
  const generation = session.currentGeneration();
  if (refreshInFlight?.generation === generation) return refreshInFlight.promise;

  const promise = (async () => {
    if (reason === 'bootstrap') {
      const current = session.getState();
      if (session.isGenerationCurrent(generation) && current.status === 'unknown') {
        session.patch({ status: 'bootstrapping' });
      }
    }

    try {
      const csrfToken = getCsrfToken();
      const headers = new Headers({ Accept: 'application/json' });
      if (csrfToken) headers.set('X-CSRF-Token', csrfToken);

      const timeout = withRequestTimeout(undefined, apiControlTimeoutMs);
      let response: Response;
      let rawText: string;
      try {
        response = await fetch(buildUrl('/auth/refresh'), {
          method: 'POST',
          credentials: 'include',
          headers,
          signal: timeout.signal,
        });
        rawText = await response.text();
      } finally {
        timeout.clear();
      }

      const contentType = response.headers.get('content-type') ?? '';
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

      if (!session.isGenerationCurrent(generation)) return;
      session.patch({
        status: 'authenticated',
        accessToken: data.access_token,
        expiresAt: data.expires_at ?? null,
      });
    } catch (error) {
      if (!session.isGenerationCurrent(generation)) return;
      // Only treat refresh 401 as "session is gone" and hard logout.
      if (error instanceof ApiError && error.status === 401) {
        session.expire();
      } else if (reason === 'bootstrap') {
        // Avoid getting stuck in "bootstrapping" state on transient errors.
        session.patch({ status: 'guest' });
      }
      throw error;
    }
  })().finally(() => {
    if (refreshInFlight?.generation === generation) refreshInFlight = null;
  });

  refreshInFlight = { generation, promise };
  return promise;
};

const shouldAttemptRefresh = (path: string, session: ApiAuthSession): boolean => {
  const normalized = normalizePath(path);
  if (
    normalized === '/auth/login' ||
    normalized === '/auth/refresh' ||
    normalized === '/auth/logout'
  )
    return false;
  return Boolean(session.getState().accessToken);
};

const shouldAttachAuthHeader = (path: string): boolean => {
  const normalized = normalizePath(path);
  return (
    normalized !== '/auth/login' && normalized !== '/auth/refresh' && normalized !== '/auth/logout'
  );
};

type ApiResponseType = NonNullable<ApiRequestOptions['responseType']>;

interface ApiAuthContext {
  shouldUseAuth: boolean;
  session: ApiAuthSession | null;
  generation?: number;
}

interface ApiRequestContext {
  path: string;
  json: unknown;
  csrf: ApiRequestOptions['csrf'];
  headersInit: HeadersInit | undefined;
  body: BodyInit | null | undefined;
  requestInit: RequestInit;
  session: ApiAuthSession | null;
}

const resolveAuthContext = (
  path: string,
  method: string,
  authMode: NonNullable<ApiRequestOptions['auth']>,
): ApiAuthContext => {
  const shouldUseAuth = authMode === 'auto';
  const session =
    shouldUseAuth && needsBoundAuthSession(path, method)
      ? requireAuthSession()
      : shouldUseAuth
        ? authSession
        : null;

  return {
    shouldUseAuth,
    session,
    generation: session?.currentGeneration(),
  };
};

const assertAuthContextCurrent = ({ session, generation }: ApiAuthContext): void => {
  if (session && generation !== undefined && !session.isGenerationCurrent(generation)) {
    throw new ApiAuthStaleError();
  }
};

const sendApiRequest = ({
  path,
  json,
  csrf,
  headersInit,
  body,
  requestInit,
  session,
}: ApiRequestContext): Promise<Response> => {
  const headers = new Headers(headersInit ?? {});
  headers.set('Accept', 'application/json');

  if (json !== undefined) {
    headers.set('Content-Type', 'application/json');
  }

  if (session && shouldAttachAuthHeader(path)) {
    const { accessToken } = session.getState();
    if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
  }

  const csrfMode = csrf ?? 'auto';
  if (csrfMode === 'auto' && shouldInjectCsrfHeader(path)) {
    const csrfToken = getCsrfToken();
    if (csrfToken) headers.set('X-CSRF-Token', csrfToken);
  }

  return fetch(buildUrl(path), {
    ...requestInit,
    headers,
    credentials: 'include',
    body: json !== undefined ? JSON.stringify(json) : body,
  });
};

const readApiResponse = async <T>(
  response: Response,
  responseType: ApiResponseType,
  assertCurrent: () => void,
): Promise<T | string | undefined> => {
  const contentType = response.headers.get('content-type') ?? '';
  const rawText = await response.text();
  assertCurrent();

  if (!response.ok) {
    const parsed = shouldTreatAsJson(contentType)
      ? (parseJsonTextSafe(rawText) as ErrorResponse | undefined)
      : undefined;
    const message = parsed?.message ?? response.statusText ?? 'Request failed';
    throw new ApiError(message, response.status, parsed?.code, parsed ?? rawText);
  }

  emitApiWarning(response.headers.get(API_WARNING_HEADER) ?? '');

  if (responseType === 'text') {
    return rawText;
  }

  if (responseType === 'empty') {
    if (rawText.trim()) {
      throw new ApiError('Expected empty response', response.status, 'unexpected_response_body', {
        body: rawText,
      });
    }
    return undefined;
  }

  if (!rawText.trim()) {
    if (responseType === 'jsonOrEmpty') return undefined;
    throw new ApiError('Expected JSON response', response.status, 'empty_response', {
      status: response.status,
    });
  }

  if (!shouldTreatAsJson(contentType)) {
    throw new ApiError('Expected JSON response', response.status, 'invalid_response_content_type', {
      contentType,
      body: rawText,
    });
  }

  try {
    return parseJsonText(rawText) as T;
  } catch (error) {
    throw new ApiError('Invalid JSON response', response.status, 'invalid_json_response', {
      body: rawText,
      cause: error instanceof Error ? error.message : String(error),
    });
  }
};

export function apiFetch(
  path: string,
  options: ApiRequestOptions & { responseType: 'text' },
): Promise<string>;
export function apiFetch(
  path: string,
  options: ApiRequestOptions & { responseType: 'empty' },
): Promise<void>;
export function apiFetch<T>(
  path: string,
  options: ApiRequestOptions & { responseType: 'jsonOrEmpty' },
): Promise<T | undefined>;
export function apiFetch<T = unknown>(
  path: string,
  options?: ApiRequestOptions & { responseType?: 'json' },
): Promise<T>;
export async function apiFetch<T = unknown>(
  path: string,
  options: ApiRequestOptions = {},
): Promise<T | string | undefined> {
  const {
    json,
    auth,
    csrf,
    retryOn401,
    responseType = 'json',
    headers: headersInit,
    body,
    timeoutMs,
    signal,
    ...requestInit
  } = options;

  const method = String(requestInit.method ?? 'GET').toUpperCase();
  const authMode = auth ?? 'auto';
  const authContext = resolveAuthContext(path, method, authMode);
  const timeout = withRequestTimeout(signal, timeoutMs);
  const requestContext: ApiRequestContext = {
    path,
    json,
    csrf,
    headersInit,
    body,
    requestInit: { ...requestInit, signal: timeout.signal },
    session: authContext.session,
  };

  try {
    let response = await sendApiRequest(requestContext);
    assertAuthContextCurrent(authContext);

    const shouldRetryOn401 = retryOn401 ?? true;
    if (
      authContext.shouldUseAuth &&
      shouldRetryOn401 &&
      response.status === 401 &&
      authContext.session &&
      authContext.generation !== undefined &&
      authContext.session.isGenerationCurrent(authContext.generation) &&
      shouldAttemptRefresh(path, authContext.session)
    ) {
      await refreshSession('retry401');
      assertAuthContextCurrent(authContext);
      response = await sendApiRequest(requestContext);
      assertAuthContextCurrent(authContext);
    }

    return await readApiResponse<T>(response, responseType, () =>
      assertAuthContextCurrent(authContext),
    );
  } finally {
    timeout.clear();
  }
}

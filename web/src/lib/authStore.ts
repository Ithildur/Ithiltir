export interface AuthState {
  status: 'unknown' | 'bootstrapping' | 'authenticated' | 'guest';
  accessToken: string | null;
  expiresAt: string | null;
  csrfToken: string | null;
}

export type LoginPersistence = 'session' | 'persistent';

const loginPersistenceKey = 'auth.login_persistence';

let state: AuthState = {
  status: 'unknown',
  accessToken: null,
  expiresAt: null,
  csrfToken: null,
};

const listeners = new Set<() => void>();

export const getAuthState = (): AuthState => state;

export const subscribeAuthState = (listener: () => void): (() => void) => {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
};

const emit = () => {
  for (const listener of listeners) listener();
};

export const replaceAuthState = (next: AuthState): void => {
  state = next;
  emit();
};

export const patchAuthState = (patch: Partial<AuthState>): void => {
  state = { ...state, ...patch };
  emit();
};

export const clearAuthState = (): void => {
  state = { status: 'guest', accessToken: null, expiresAt: null, csrfToken: null };
  emit();
};

export const readLoginPersistence = (): LoginPersistence | null => {
  if (typeof window === 'undefined') return null;
  try {
    const persistent = window.localStorage.getItem(loginPersistenceKey);
    if (persistent === 'persistent') return 'persistent';

    const session = window.sessionStorage.getItem(loginPersistenceKey);
    if (session === 'session') return 'session';
  } catch {
    return null;
  }
  return null;
};

export const readRememberLogin = (): boolean => {
  return readLoginPersistence() === 'persistent';
};

export const writeLoginPersistence = (persistence: LoginPersistence | null): void => {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.removeItem(loginPersistenceKey);
    window.sessionStorage.removeItem(loginPersistenceKey);

    if (persistence === 'persistent') {
      window.localStorage.setItem(loginPersistenceKey, persistence);
      return;
    }
    if (persistence === 'session') {
      window.sessionStorage.setItem(loginPersistenceKey, persistence);
    }
  } catch {
    // ignore storage errors
  }
};

export const writeRememberLogin = (remember: boolean): void => {
  writeLoginPersistence(remember ? 'persistent' : 'session');
};

export const readCookie = (name: string): string | null => {
  if (typeof document === 'undefined') return null;
  const cookie = document.cookie;
  if (!cookie) return null;

  const prefix = `${encodeURIComponent(name)}=`;
  const parts = cookie.split(/;\s*/);
  for (const part of parts) {
    if (!part.startsWith(prefix)) continue;
    const value = part.slice(prefix.length);
    try {
      return decodeURIComponent(value);
    } catch {
      return value;
    }
  }
  return null;
};

export const getCsrfToken = (): string | null => {
  // Single source of truth: cookie. Avoid caching CSRF in memory, because refresh rotates it.
  const fromCookie = readCookie('csrf')?.trim();
  return fromCookie || null;
};

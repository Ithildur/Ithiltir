type LoginPersistence = 'session' | 'persistent';

const loginPersistenceKey = 'auth.login_persistence';

type BrowserStorage = 'local' | 'session';

const browserStorage = (kind: BrowserStorage): Storage | null => {
  if (typeof window === 'undefined') return null;
  try {
    return kind === 'local' ? window.localStorage : window.sessionStorage;
  } catch {
    return null;
  }
};

const readStorageItem = (kind: BrowserStorage, key: string): string | null => {
  const storage = browserStorage(kind);
  if (!storage) return null;
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
};

const removeStorageItem = (kind: BrowserStorage, key: string): void => {
  const storage = browserStorage(kind);
  if (!storage) return;
  try {
    storage.removeItem(key);
  } catch {
    // Storage can be unavailable in private or hardened browser modes.
  }
};

const writeStorageItem = (kind: BrowserStorage, key: string, value: string): void => {
  const storage = browserStorage(kind);
  if (!storage) return;
  try {
    storage.setItem(key, value);
  } catch {
    // Storage can be unavailable in private or hardened browser modes.
  }
};

export const readLoginPersistence = (): LoginPersistence | null => {
  const persistent = readStorageItem('local', loginPersistenceKey);
  if (persistent === 'persistent') return 'persistent';

  const session = readStorageItem('session', loginPersistenceKey);
  if (session === 'session') return 'session';
  return null;
};

export const writeLoginPersistence = (persistence: LoginPersistence | null): void => {
  removeStorageItem('local', loginPersistenceKey);
  removeStorageItem('session', loginPersistenceKey);

  if (persistence === 'persistent') {
    writeStorageItem('local', loginPersistenceKey, persistence);
    return;
  }
  if (persistence === 'session') {
    writeStorageItem('session', loginPersistenceKey, persistence);
  }
};

const readCookie = (name: string): string | null => {
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
  const fromCookie = readCookie('csrf')?.trim();
  return fromCookie || null;
};

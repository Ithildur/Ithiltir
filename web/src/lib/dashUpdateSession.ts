const updateReloadKey = 'dash.update_reload';
const updateReloadMaxAgeMs = 10 * 60_000;

export type DashUpdateReload = {
  version: string;
  recordedAt: number;
};

const sessionStorage = (): Storage | null => {
  if (typeof window === 'undefined') return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
};

const onDashUpdatePage = (): boolean => {
  if (typeof window === 'undefined') return false;
  const path = window.location.pathname.replace(/\/+$/, '');
  if (path !== '/admin') return false;
  const params = new URLSearchParams(window.location.search);
  return params.get('tab') === 'system' && params.get('system_tab') === 'dash_update';
};

export const clearDashUpdateReload = (): void => {
  const storage = sessionStorage();
  if (!storage) return;
  try {
    storage.removeItem(updateReloadKey);
  } catch {
    // Storage can be unavailable in private or hardened browser modes.
  }
};

export const rememberDashUpdateReload = (version: string): void => {
  const normalized = version.trim();
  if (!normalized || !onDashUpdatePage()) return;
  const storage = sessionStorage();
  if (!storage) return;
  try {
    const value: DashUpdateReload = { version: normalized, recordedAt: Date.now() };
    storage.setItem(updateReloadKey, JSON.stringify(value));
  } catch {
    // Storage can be unavailable in private or hardened browser modes.
  }
};

export const readDashUpdateReload = (): DashUpdateReload | null => {
  const storage = sessionStorage();
  if (!storage) return null;
  try {
    const raw = storage.getItem(updateReloadKey);
    if (!raw) return null;
    const value = JSON.parse(raw) as Partial<DashUpdateReload>;
    const age = Date.now() - (value.recordedAt ?? 0);
    if (
      typeof value.version !== 'string' ||
      !value.version.trim() ||
      typeof value.recordedAt !== 'number' ||
      age < 0 ||
      age > updateReloadMaxAgeMs
    ) {
      clearDashUpdateReload();
      return null;
    }
    return { version: value.version.trim(), recordedAt: value.recordedAt };
  } catch {
    clearDashUpdateReload();
    return null;
  }
};

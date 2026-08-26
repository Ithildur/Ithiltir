const updateReloadKey = 'dash.update_reload';
const updateReloadMaxAgeMs = 10 * 60_000;
const updateTargetKey = 'dash.update_target';

export const dashUpdateTargetEvent = 'dash:update-target';

export type DashUpdateReload = {
  version: string;
  recordedAt: number;
};

export type DashUpdateTarget = {
  version: string;
  recordedAt: number;
  status: 'pending' | 'failed';
};

let volatileTarget: DashUpdateTarget | null = null;

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

const emitDashUpdateTarget = (): void => {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(dashUpdateTargetEvent));
};

export const rememberDashUpdateTarget = (version: string): void => {
  const normalized = version.trim();
  if (!normalized) return;

  const value: DashUpdateTarget = {
    version: normalized,
    recordedAt: Date.now(),
    status: 'pending',
  };
  volatileTarget = value;
  const storage = sessionStorage();
  if (storage) {
    try {
      storage.setItem(updateTargetKey, JSON.stringify(value));
    } catch {
      // The in-memory value still keeps the active page coordinated.
    }
  }
  emitDashUpdateTarget();
};

export const readDashUpdateTarget = (): DashUpdateTarget | null => {
  if (volatileTarget) return volatileTarget;

  const storage = sessionStorage();
  if (!storage) return null;
  try {
    const raw = storage.getItem(updateTargetKey);
    if (!raw) return null;
    const value = JSON.parse(raw) as Partial<DashUpdateTarget>;
    if (
      typeof value.version !== 'string' ||
      !value.version.trim() ||
      typeof value.recordedAt !== 'number' ||
      !Number.isFinite(value.recordedAt) ||
      (value.status !== 'pending' && value.status !== 'failed')
    ) {
      storage.removeItem(updateTargetKey);
      return null;
    }
    volatileTarget = {
      version: value.version.trim(),
      recordedAt: value.recordedAt,
      status: value.status,
    };
    return volatileTarget;
  } catch {
    try {
      storage.removeItem(updateTargetKey);
    } catch {
      // Storage can become unavailable after an earlier successful read.
    }
    return null;
  }
};

export const failDashUpdateTarget = (expectedVersion?: string): boolean => {
  const expected = expectedVersion?.trim() ?? '';
  const current = readDashUpdateTarget();
  if (expected && current?.version !== expected) return false;
  if (!current) return false;
  if (current.status === 'failed') return true;

  const next: DashUpdateTarget = { ...current, status: 'failed' };
  volatileTarget = next;
  const storage = sessionStorage();
  if (storage) {
    try {
      storage.setItem(updateTargetKey, JSON.stringify(next));
    } catch {
      // The in-memory value still keeps the active page coordinated.
    }
  }
  emitDashUpdateTarget();
  return true;
};

export const clearDashUpdateTarget = (expectedVersion?: string): boolean => {
  const expected = expectedVersion?.trim() ?? '';
  const current = readDashUpdateTarget();
  if (expected && current?.version !== expected) return false;
  if (!current) return false;

  volatileTarget = null;
  const storage = sessionStorage();
  if (storage) {
    try {
      storage.removeItem(updateTargetKey);
    } catch {
      // The active page is still cleared through the in-memory value and event.
    }
  }
  emitDashUpdateTarget();
  return true;
};

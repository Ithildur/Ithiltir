import React from 'react';
import { apiFetch, refreshSession } from '@lib/api';
import { useI18n } from '@i18n';
import { useTopBanner } from '@components/ui/TopBannerStack';
import {
  clearAuthState,
  getAuthState,
  getCsrfToken,
  readLoginPersistence,
  replaceAuthState,
  subscribeAuthState,
  writeLoginPersistence,
} from '@lib/authStore';
import type { LoginResponse } from '@app-types/api';

interface AuthContextValue {
  token: string | null;
  expiresAt: string | null;
  status: 'unknown' | 'bootstrapping' | 'authenticated' | 'guest';
  isAuthenticated: boolean;
  login: (password: string, remember: boolean) => Promise<void>;
  bootstrap: () => Promise<boolean>;
  logout: () => void;
}

const AuthContext = React.createContext<AuthContextValue | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const snapshot = React.useSyncExternalStore(subscribeAuthState, getAuthState, getAuthState);
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const refreshTimerRef = React.useRef<number | null>(null);

  const logout = React.useCallback(() => {
    const tokenToRevoke = getAuthState().accessToken;
    const csrfToUse = getCsrfToken();
    writeLoginPersistence(null);
    clearAuthState();

    if (!tokenToRevoke && !csrfToUse) return;
    void apiFetch('/auth/logout', {
      method: 'POST',
      auth: 'none',
      csrf: 'none',
      retryOn401: false,
      headers: {
        ...(tokenToRevoke ? { Authorization: `Bearer ${tokenToRevoke}` } : {}),
        ...(csrfToUse ? { 'X-CSRF-Token': csrfToUse } : {}),
      },
    }).catch((error) => {
      console.warn('Failed to revoke jwt on logout.', error);
    });
  }, []);

  const bootstrap = React.useCallback(async (): Promise<boolean> => {
    const current = getAuthState();
    if (current.status === 'authenticated' && current.accessToken) return true;
    if (!readLoginPersistence()) {
      clearAuthState();
      return false;
    }

    try {
      await refreshSession('bootstrap');
      return Boolean(getAuthState().accessToken);
    } catch {
      return false;
    }
  }, []);

  const login = React.useCallback(async (password: string, remember: boolean) => {
    const result = await apiFetch<LoginResponse>('/auth/login', {
      method: 'POST',
      json: { password, persistence: remember ? 'persistent' : 'session' },
    });
    writeLoginPersistence(remember ? 'persistent' : 'session');
    replaceAuthState({
      status: 'authenticated',
      accessToken: result.access_token,
      expiresAt: result.expires_at,
      // CSRF is rotated and stored in cookie; avoid caching in memory.
      csrfToken: null,
    });
  }, []);

  React.useEffect(() => {
    if (refreshTimerRef.current !== null) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }

    if (snapshot.status !== 'authenticated' || !snapshot.accessToken || !snapshot.expiresAt) {
      return;
    }

    const expiryMs = Date.parse(snapshot.expiresAt);
    if (Number.isNaN(expiryMs)) return;

    const triggerAt = expiryMs - 60_000;
    const delayMs = triggerAt - Date.now();
    const timeoutMs = delayMs <= 0 ? 0 : delayMs;

    const runRefresh = async () => {
      try {
        await refreshSession('retry401');
      } catch (error) {
        console.error('Scheduled token refresh failed', error);
        pushBanner(t('auth_refresh_failed'), { tone: 'error', durationMs: 4000 });
      }
    };

    refreshTimerRef.current = window.setTimeout(runRefresh, timeoutMs);

    return () => {
      if (refreshTimerRef.current !== null) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [pushBanner, snapshot.accessToken, snapshot.expiresAt, snapshot.status, t]);

  const token = snapshot.accessToken;
  const isAuthenticated = snapshot.status === 'authenticated' && Boolean(token);
  const contextValue = React.useMemo<AuthContextValue>(
    () => ({
      token,
      expiresAt: snapshot.expiresAt,
      status: snapshot.status,
      isAuthenticated,
      bootstrap,
      login,
      logout,
    }),
    [bootstrap, isAuthenticated, login, logout, snapshot.expiresAt, snapshot.status, token],
  );

  return <AuthContext.Provider value={contextValue}>{children}</AuthContext.Provider>;
};

export const useAuth = (): AuthContextValue => {
  const ctx = React.useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
};

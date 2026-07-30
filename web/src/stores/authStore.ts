import { create } from 'zustand';
import type { LoginResponse } from '@app-types/api';
import type { AuthState } from '@app-types/auth';
import {
  ApiError,
  apiControlTimeoutMs,
  apiFetch,
  bindApiAuthSession,
  refreshSession,
} from '@lib/api';
import { getCsrfToken, readLoginPersistence, writeLoginPersistence } from '@lib/authSession';
import { resetPrivateStores } from './privateStores';

type AuthStoreState = AuthState;

const initialAuthState: AuthState = {
  status: 'unknown',
  accessToken: null,
  expiresAt: null,
};

const pickAuthState = (state: AuthState): AuthState => ({
  status: state.status,
  accessToken: state.accessToken,
  expiresAt: state.expiresAt,
});

export const useAuthStore = create<AuthStoreState>()(() => initialAuthState);

let generation = 0;

const getAuthState = (): AuthState => pickAuthState(useAuthStore.getState());

const bumpAuthGeneration = (): number => {
  generation += 1;
  return generation;
};

const isAuthGenerationCurrent = (target: number): boolean => target === generation;

const replaceAuthState = (next: AuthState): void => {
  useAuthStore.setState(pickAuthState(next));
};

const patchAuthState = (patch: Partial<AuthState>): void => {
  useAuthStore.setState(patch);
};

const clearAuthState = (): void => {
  useAuthStore.setState({
    status: 'guest',
    accessToken: null,
    expiresAt: null,
  });
};

const clearLocalSession = (): void => {
  writeLoginPersistence(null);
  clearAuthState();
  resetPrivateStores();
};

const ignoreLogoutRevokeError = (): void => {
  // Local session is already cleared; remote revoke is best-effort only.
};

const expireAuthState = (): void => {
  bumpAuthGeneration();
  clearLocalSession();
};

let authApiSessionInstalled = false;

export const installAuthApiSession = (): void => {
  if (authApiSessionInstalled) return;
  bindApiAuthSession({
    getState: getAuthState,
    patch: patchAuthState,
    expire: expireAuthState,
    currentGeneration: () => generation,
    isGenerationCurrent: isAuthGenerationCurrent,
  });
  authApiSessionInstalled = true;
};

export const logout = (): void => {
  const tokenToRevoke = getAuthState().accessToken;
  const csrfToUse = getCsrfToken();
  bumpAuthGeneration();
  clearLocalSession();

  if (!tokenToRevoke && !csrfToUse) return;
  void apiFetch('/auth/logout', {
    method: 'POST',
    keepalive: true,
    auth: 'none',
    csrf: 'none',
    retryOn401: false,
    responseType: 'empty',
    headers: {
      ...(tokenToRevoke ? { Authorization: `Bearer ${tokenToRevoke}` } : {}),
      ...(csrfToUse ? { 'X-CSRF-Token': csrfToUse } : {}),
    },
  }).catch(ignoreLogoutRevokeError);
};

export const bootstrap = async (): Promise<void> => {
  const current = getAuthState();
  if (current.status === 'authenticated' && current.accessToken) return;
  if (!readLoginPersistence()) {
    clearAuthState();
    return;
  }

  try {
    await refreshSession('bootstrap');
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return;
    throw error;
  }
};

export const refresh = async (): Promise<void> => {
  await refreshSession('retry401');
};

export const login = async (password: string, remember: boolean): Promise<void> => {
  const result = await apiFetch<LoginResponse>('/auth/login', {
    method: 'POST',
    json: { password, persistence: remember ? 'persistent' : 'session' },
    timeoutMs: apiControlTimeoutMs,
  });
  bumpAuthGeneration();
  resetPrivateStores();
  writeLoginPersistence(remember ? 'persistent' : 'session');
  replaceAuthState({
    status: 'authenticated',
    accessToken: result.access_token,
    expiresAt: result.expires_at,
  });
};

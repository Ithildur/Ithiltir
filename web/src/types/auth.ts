type AuthStatus = 'unknown' | 'bootstrapping' | 'authenticated' | 'guest';

export interface AuthState {
  status: AuthStatus;
  accessToken: string | null;
  expiresAt: string | null;
}

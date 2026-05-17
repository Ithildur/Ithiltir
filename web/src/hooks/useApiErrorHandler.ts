import React from 'react';
import { useAuth } from '@context/AuthContext';
import { ApiError } from '@lib/api';
import type { TranslationKey } from '@i18n';
import { useI18n } from '@i18n';
import { useTopBanner } from '@components/ui/TopBannerStack';

export type ApiErrorFallback =
  | string
  | {
      key: TranslationKey;
      vars?: Record<string, string | number>;
    };

const errorKeyByCode: Partial<Record<string, TranslationKey>> = {
  auth_cache_error: 'error_sync_failed_retry',
  guest_visible_cache_error: 'error_sync_failed_retry',
  redis_cache_error: 'error_sync_failed_retry',
  redis_error: 'error_state_unavailable_retry',
};

export type ApiErrorHandler = (error: unknown, fallback: ApiErrorFallback) => void;

const fallbackText = (
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string,
  fallback: ApiErrorFallback,
): string => (typeof fallback === 'string' ? fallback : t(fallback.key, fallback.vars));

export const useApiErrorHandler = (): ApiErrorHandler => {
  const { logout } = useAuth();
  const pushBanner = useTopBanner();
  const { t } = useI18n();
  const currentRef = React.useRef({ logout, pushBanner, t });

  React.useEffect(() => {
    currentRef.current = { logout, pushBanner, t };
  }, [logout, pushBanner, t]);

  return React.useCallback((error: unknown, fallback: ApiErrorFallback) => {
    const { logout, pushBanner, t } = currentRef.current;
    const fallbackMessage = fallbackText(t, fallback);

    if (error instanceof ApiError) {
      if (error.status === 401) {
        pushBanner(t('auth_session_expired'), { tone: 'warning' });
        logout();
        return;
      }
      const key = error.code ? errorKeyByCode[error.code] : undefined;
      pushBanner(key ? t(key) : error.message || fallbackMessage, { tone: 'error' });
      return;
    }
    pushBanner(fallbackMessage, { tone: 'error' });
  }, []);
};

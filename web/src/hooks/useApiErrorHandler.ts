import React from 'react';
import { logout } from '@stores/authStore';
import { ApiError, isApiAuthStaleError } from '@lib/api';
import type { TranslationKey } from '@i18n';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';

type ApiErrorFallback =
  | string
  | {
      key: TranslationKey;
      vars?: Record<string, string | number>;
    };

const errorKeyByCode: Partial<Record<string, TranslationKey>> = {
  auth_cache_error: 'error_sync_failed_retry',
  guest_visible_cache_error: 'error_sync_failed_retry',
  node_upgrade_unsupported: 'admin_nodes_auto_update_requires_manual',
  redis_cache_error: 'error_sync_failed_retry',
  redis_error: 'error_state_unavailable_retry',
  traffic_rebuild_running: 'traffic_rebuild_running_other',
  traffic_rebuild_requires_billing: 'admin_node_traffic_rebuild_requires_billing',
};

type ApiErrorHandler = (error: unknown, fallback: ApiErrorFallback) => void;

const fallbackText = (
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string,
  fallback: ApiErrorFallback,
): string => (typeof fallback === 'string' ? fallback : t(fallback.key, fallback.vars));

export const useApiErrorHandler = (): ApiErrorHandler => {
  const { t } = useI18n();
  const currentRef = React.useRef({ t });

  React.useEffect(() => {
    currentRef.current = { t };
  }, [t]);

  return React.useCallback((error: unknown, fallback: ApiErrorFallback) => {
    const { t } = currentRef.current;
    const fallbackMessage = fallbackText(t, fallback);

    if (isApiAuthStaleError(error)) return;

    if (error instanceof ApiError) {
      if (error.status === 401) {
        pushTopBanner(t('auth_session_expired'), { tone: 'warning' });
        logout();
        return;
      }
      const key = error.code ? errorKeyByCode[error.code] : undefined;
      pushTopBanner(key ? t(key) : error.message || fallbackMessage, { tone: 'error' });
      return;
    }
    pushTopBanner(fallbackMessage, { tone: 'error' });
  }, []);
};

import React from 'react';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { bootstrap, refresh, useAuthStore } from '@stores/authStore';

let bootstrapErrorShown = false;

export const AuthRuntime: React.FC = () => {
  const { t } = useI18n();
  const status = useAuthStore((state) => state.status);
  const token = useAuthStore((state) => state.accessToken);
  const expiresAt = useAuthStore((state) => state.expiresAt);
  const refreshTimerRef = React.useRef<number | null>(null);
  const refreshFailedMessageRef = React.useRef(t('auth_refresh_failed'));

  React.useEffect(() => {
    refreshFailedMessageRef.current = t('auth_refresh_failed');
  }, [t]);

  React.useEffect(() => {
    if (status === 'authenticated') {
      bootstrapErrorShown = false;
    }
    if (status !== 'unknown') return;

    void bootstrap().catch(() => {
      if (bootstrapErrorShown) return;
      bootstrapErrorShown = true;
      pushTopBanner(refreshFailedMessageRef.current, { tone: 'error', durationMs: 4000 });
    });
  }, [status]);

  React.useEffect(() => {
    if (refreshTimerRef.current !== null) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }

    if (status !== 'authenticated' || !token || !expiresAt) {
      return;
    }

    const expiryMs = Date.parse(expiresAt);
    if (Number.isNaN(expiryMs)) return;

    const triggerAt = expiryMs - 60_000;
    const delayMs = triggerAt - Date.now();
    const timeoutMs = delayMs <= 0 ? 0 : delayMs;

    const runRefresh = async () => {
      try {
        await refresh();
      } catch {
        pushTopBanner(refreshFailedMessageRef.current, { tone: 'error', durationMs: 4000 });
      }
    };

    refreshTimerRef.current = window.setTimeout(runRefresh, timeoutMs);

    return () => {
      if (refreshTimerRef.current !== null) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [expiresAt, status, token]);

  return null;
};

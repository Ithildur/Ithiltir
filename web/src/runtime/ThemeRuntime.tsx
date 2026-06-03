import React from 'react';
import { useI18n } from '@i18n';
import { themeRefreshEvent } from '@lib/themePackageRuntime';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { applyThemeMode, subscribeSystemTheme, writeThemeMode } from '@stores/themeModeBridge';
import { refreshTheme, resolveTheme, useThemeStore } from '@stores/themeStore';
import { isAbortError } from '@utils/errors';

export const ThemeRuntime: React.FC = () => {
  const { t } = useI18n();
  const mode = useThemeStore((state) => state.themeMode);
  const loadFailedMessageRef = React.useRef(t('theme_runtime_load_failed'));

  React.useEffect(() => {
    loadFailedMessageRef.current = t('theme_runtime_load_failed');
  }, [t]);

  React.useEffect(() => {
    const controller = new AbortController();
    void resolveTheme(controller.signal).catch((error) => {
      if (isAbortError(error)) return;
      pushTopBanner(loadFailedMessageRef.current, { tone: 'warning', durationMs: 4000 });
    });
    return () => {
      controller.abort();
    };
  }, []);

  React.useEffect(() => {
    const onRefresh = () => {
      void refreshTheme().catch(() => {
        pushTopBanner(loadFailedMessageRef.current, { tone: 'warning', durationMs: 4000 });
      });
    };

    window.addEventListener(themeRefreshEvent, onRefresh);
    return () => {
      window.removeEventListener(themeRefreshEvent, onRefresh);
    };
  }, []);

  React.useEffect(() => {
    applyThemeMode(mode);
    writeThemeMode(mode);

    if (mode !== 'system') return;
    return subscribeSystemTheme(() => applyThemeMode('system'));
  }, [mode]);

  return null;
};

import React from 'react';
import type { TranslationKey } from '@i18n';
import { useI18n } from '@i18n';
import { API_WARNING_EVENT } from '@lib/api';
import TopBannerStack from '@components/ui/TopBannerStack';
import { useTopBannerStore } from '@stores/topBannerStore';
import { clearTopBanners, closeTopBanner, pushTopBanner } from '@runtime/topBannerRuntime';

const warningKeyByCode: Partial<Record<string, TranslationKey>> = {
  redis_cache_error: 'warning_redis_cache_error',
  theme_active_broken: 'warning_theme_active_broken',
  theme_active_missing: 'warning_theme_active_missing',
};

export const TopBannerHost: React.FC = () => {
  const { t } = useI18n();
  const banners = useTopBannerStore((state) => state.topBanners);

  React.useEffect(
    () => () => {
      clearTopBanners();
    },
    [],
  );

  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    const onWarning = (event: Event) => {
      const detail = (event as CustomEvent<{ code?: string }>).detail;
      const code = typeof detail?.code === 'string' ? detail.code.trim() : '';
      if (!code) return;

      const key = warningKeyByCode[code];
      pushTopBanner(key ? t(key) : t('warning_api_notice', { code }), {
        tone: 'warning',
        durationMs: 4500,
      });
    };

    window.addEventListener(API_WARNING_EVENT, onWarning as EventListener);
    return () => {
      window.removeEventListener(API_WARNING_EVENT, onWarning as EventListener);
    };
  }, [t]);

  return <TopBannerStack banners={banners} onClose={closeTopBanner} />;
};

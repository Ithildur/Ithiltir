import React from 'react';
import type { SiteBrand } from '@app-types/site';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { refreshBrand, useSiteBrandStore } from '@stores/siteBrandStore';
import { isAbortError } from '@utils/errors';

const faviconType = (logoURL: string): string => {
  const value = logoURL.toLowerCase();
  if (value.startsWith('data:')) {
    const end = value.indexOf(';');
    return end > 'data:'.length ? value.slice('data:'.length, end) : 'image/png';
  }
  if (value.endsWith('.svg')) return 'image/svg+xml';
  if (value.endsWith('.ico')) return 'image/x-icon';
  if (value.endsWith('.webp')) return 'image/webp';
  if (value.endsWith('.jpg') || value.endsWith('.jpeg')) return 'image/jpeg';
  return 'image/png';
};

const applyDocumentBrand = (brand: SiteBrand): void => {
  if (typeof document === 'undefined') return;

  document.title = brand.page_title;

  let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!icon) {
    icon = document.createElement('link');
    icon.rel = 'icon';
    document.head.appendChild(icon);
  }
  icon.type = faviconType(brand.logo_url);
  icon.href = brand.logo_url;
};

export const SiteBrandRuntime: React.FC = () => {
  const { t } = useI18n();
  const brand = useSiteBrandStore((state) => state.brand);
  const loadFailedMessageRef = React.useRef(t('brand_runtime_load_failed'));

  React.useEffect(() => {
    loadFailedMessageRef.current = t('brand_runtime_load_failed');
  }, [t]);

  React.useEffect(() => {
    const controller = new AbortController();
    refreshBrand({ signal: controller.signal }).catch((error) => {
      if (isAbortError(error)) return;
      pushTopBanner(loadFailedMessageRef.current, { tone: 'warning', durationMs: 4000 });
    });
    return () => controller.abort();
  }, []);

  React.useEffect(() => {
    applyDocumentBrand(brand);
  }, [brand]);

  return null;
};

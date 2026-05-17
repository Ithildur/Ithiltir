import React from 'react';
import type { SiteBrand } from '@app-types/site';
import { defaultSiteBrand, fetchSiteBrand, normalizeSiteBrand } from '@lib/siteBrandApi';

interface SiteBrandContextValue {
  brand: SiteBrand;
  refreshBrand: () => Promise<SiteBrand>;
  setBrand: (brand: Partial<SiteBrand>) => SiteBrand;
}

const SiteBrandContext = React.createContext<SiteBrandContextValue>({
  brand: defaultSiteBrand,
  refreshBrand: async () => defaultSiteBrand,
  setBrand: () => defaultSiteBrand,
});

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

export const SiteBrandProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [brand, setBrandState] = React.useState<SiteBrand>(defaultSiteBrand);

  const setBrand = React.useCallback((input: Partial<SiteBrand>): SiteBrand => {
    const next = normalizeSiteBrand(input);
    setBrandState(next);
    return next;
  }, []);

  const refreshBrand = React.useCallback(async (): Promise<SiteBrand> => {
    return setBrand(await fetchSiteBrand());
  }, [setBrand]);

  React.useEffect(() => {
    const controller = new AbortController();
    fetchSiteBrand({ signal: controller.signal })
      .then(setBrand)
      .catch((error) => {
        if (error instanceof DOMException && error.name === 'AbortError') return;
      });
    return () => controller.abort();
  }, [setBrand]);

  React.useEffect(() => {
    applyDocumentBrand(brand);
  }, [brand]);

  const value = React.useMemo(
    () => ({ brand, refreshBrand, setBrand }),
    [brand, refreshBrand, setBrand],
  );

  return <SiteBrandContext.Provider value={value}>{children}</SiteBrandContext.Provider>;
};

export const useSiteBrand = () => React.useContext(SiteBrandContext);

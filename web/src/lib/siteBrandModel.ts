import type { SiteBrand } from '@app-types/site';

export const defaultSiteBrand: SiteBrand = {
  logo_url: '/brandlogo.svg',
  page_title: 'Ithiltir Monitor Dashboard',
  topbar_text: 'Ithiltir Control',
};

export const displayLogoURL = (value: string): string => {
  if (
    typeof window !== 'undefined' &&
    window.location.protocol === 'https:' &&
    value.toLowerCase().startsWith('http://')
  ) {
    return defaultSiteBrand.logo_url;
  }
  return value;
};

export const normalizeSiteBrand = (input: Partial<SiteBrand> | null | undefined): SiteBrand => ({
  logo_url: input?.logo_url?.trim() || defaultSiteBrand.logo_url,
  page_title: input?.page_title?.trim() || defaultSiteBrand.page_title,
  topbar_text: input?.topbar_text?.trim() || defaultSiteBrand.topbar_text,
});

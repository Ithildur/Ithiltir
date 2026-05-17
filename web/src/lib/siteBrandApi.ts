import type { SiteBrand } from '@app-types/site';
import { apiFetch } from './api';

export const defaultSiteBrand: SiteBrand = {
  logo_url: '/brandlogo.svg',
  page_title: 'Ithiltir Monitor Dashboard',
  topbar_text: 'Ithiltir Control',
};

export const normalizeSiteBrand = (input: Partial<SiteBrand> | null | undefined): SiteBrand => ({
  logo_url: input?.logo_url?.trim() || defaultSiteBrand.logo_url,
  page_title: input?.page_title?.trim() || defaultSiteBrand.page_title,
  topbar_text: input?.topbar_text?.trim() || defaultSiteBrand.topbar_text,
});

export const fetchSiteBrand = (params: { signal?: AbortSignal } = {}) =>
  apiFetch<SiteBrand>('/front/brand', {
    method: 'GET',
    auth: 'none',
    retryOn401: false,
    signal: params.signal,
  });

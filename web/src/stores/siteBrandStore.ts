import type { SiteBrand } from '@app-types/site';
import { fetchSiteBrand } from '@lib/siteBrandApi';
import { defaultSiteBrand, normalizeSiteBrand } from '@lib/siteBrandModel';
import { create } from 'zustand';
import { createSeqGate } from '@utils/seqGate';

export interface SiteBrandState {
  brand: SiteBrand;
}

export const useSiteBrandStore = create<SiteBrandState>()(() => ({
  brand: defaultSiteBrand,
}));

const getSiteBrandState = (): SiteBrandState => useSiteBrandStore.getState();

const brandGate = createSeqGate();

const applyBrand = (brand: Partial<SiteBrand>): SiteBrand => {
  const next = normalizeSiteBrand(brand);
  useSiteBrandStore.setState({ brand: next });
  return next;
};

export const setBrand = (brand: Partial<SiteBrand>): SiteBrand => {
  brandGate.invalidate();
  return applyBrand(brand);
};

export const refreshBrand = async (params?: { signal?: AbortSignal }): Promise<SiteBrand> => {
  const seq = brandGate.next();
  const brand = await fetchSiteBrand(params);
  return brandGate.isCurrent(seq) ? applyBrand(brand) : getSiteBrandState().brand;
};

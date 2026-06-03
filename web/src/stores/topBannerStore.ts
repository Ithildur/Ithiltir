import { create } from 'zustand';
import type { BannerItem } from '@app-types/topBanner';

type NewBannerItem = Omit<BannerItem, 'id' | 'closing'>;

export interface TopBannerState {
  topBanners: BannerItem[];
}

export const useTopBannerStore = create<TopBannerState>()(() => ({
  topBanners: [],
}));

let nextBannerId = 0;

export const getTopBanners = (): BannerItem[] => useTopBannerStore.getState().topBanners;

export const addTopBannerState = (item: NewBannerItem, evictID?: number): number => {
  const id = nextBannerId + 1;
  nextBannerId = id;
  useTopBannerStore.setState((state) => ({
    topBanners: [
      ...state.topBanners.filter((banner) => banner.id !== evictID),
      { ...item, id, closing: false },
    ],
  }));
  return id;
};

export const markTopBannerClosingState = (id: number): boolean => {
  const current = getTopBanners().find((item) => item.id === id);
  if (!current) return false;
  if (current.closing) return true;

  useTopBannerStore.setState((state) => ({
    topBanners: state.topBanners.map((item) =>
      item.id === id ? { ...item, closing: true } : item,
    ),
  }));
  return true;
};

export const removeTopBannerState = (id: number): void => {
  if (!getTopBanners().some((item) => item.id === id)) return;
  useTopBannerStore.setState((state) => ({
    topBanners: state.topBanners.filter((item) => item.id !== id),
  }));
};

export const clearTopBannersState = (): void => {
  if (getTopBanners().length === 0) return;
  useTopBannerStore.setState({ topBanners: [] });
};

export type { BannerItem, BannerOptions, BannerTone, PushBanner } from '@app-types/topBanner';

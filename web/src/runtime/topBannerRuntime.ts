import type { BannerItem, BannerOptions } from '@app-types/topBanner';
import {
  addTopBannerState,
  clearTopBannersState,
  getTopBanners,
  markTopBannerClosingState,
  removeTopBannerState,
} from '@stores/topBannerStore';

const maxBanners = 4;
const closeDelayMs = 240;

const autoTimers = new Map<number, number>();
const removeTimers = new Map<number, number>();

const clearBannerTimers = (id: number): void => {
  const autoTimer = autoTimers.get(id);
  if (autoTimer !== undefined) {
    window.clearTimeout(autoTimer);
    autoTimers.delete(id);
  }

  const removeTimer = removeTimers.get(id);
  if (removeTimer !== undefined) {
    window.clearTimeout(removeTimer);
    removeTimers.delete(id);
  }
};

const clearTimers = (): void => {
  for (const id of [...autoTimers.keys()]) clearBannerTimers(id);
  for (const id of [...removeTimers.keys()]) clearBannerTimers(id);
};

const removeBanner = (id: number): void => {
  clearBannerTimers(id);
  removeTopBannerState(id);
};

const evictBannerID = (banners: BannerItem[]): number | null => {
  if (banners.length < maxBanners) return null;
  const autoIndex = banners.findIndex((banner) => banner.durationMs !== null);
  return banners[autoIndex === -1 ? 0 : autoIndex].id;
};

export function pushTopBanner(message: string, options?: BannerOptions): number {
  const durationMs = options?.durationMs === undefined ? 3000 : options.durationMs;
  const banners = getTopBanners();
  const evictID = evictBannerID(banners);

  if (evictID !== null) clearBannerTimers(evictID);
  const id = addTopBannerState(
    {
      message,
      tone: options?.tone ?? 'info',
      durationMs,
    },
    evictID ?? undefined,
  );

  if (durationMs !== null) {
    autoTimers.set(
      id,
      window.setTimeout(() => closeTopBanner(id), durationMs),
    );
  }

  return id;
}

export function closeTopBanner(id: number): void {
  const autoTimer = autoTimers.get(id);
  if (autoTimer !== undefined) {
    window.clearTimeout(autoTimer);
    autoTimers.delete(id);
  }

  if (removeTimers.has(id)) return;
  if (!markTopBannerClosingState(id)) {
    clearBannerTimers(id);
    return;
  }

  removeTimers.set(
    id,
    window.setTimeout(() => removeBanner(id), closeDelayMs),
  );
}

export function clearTopBanners(): void {
  clearTimers();
  clearTopBannersState();
}

export type BannerTone = 'info' | 'warning' | 'error';

export interface BannerItem {
  id: number;
  message: string;
  tone: BannerTone;
  closing: boolean;
  durationMs: number | null;
}

export interface BannerOptions {
  tone?: BannerTone;
  durationMs?: number | null;
}

export type PushBanner = (message: string, options?: BannerOptions) => number;

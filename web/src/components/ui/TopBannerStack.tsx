import React from 'react';
import { createPortal } from 'react-dom';
import AlertTriangle from 'lucide-react/dist/esm/icons/alert-triangle';
import InfoIcon from 'lucide-react/dist/esm/icons/info';
import X from 'lucide-react/dist/esm/icons/x';
import XCircle from 'lucide-react/dist/esm/icons/x-circle';
import type { LucideIcon } from 'lucide-react';
import type { TranslationKey } from '@i18n';
import { useI18n } from '@i18n';
import { API_WARNING_EVENT } from '@lib/api';

export type BannerTone = 'info' | 'warning' | 'error';

interface BannerItem {
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
export type PushBannerWithControls = PushBanner & { close: (id: number) => void };

const TopBannerContext = React.createContext<PushBannerWithControls | null>(null);

const warningKeyByCode: Partial<Record<string, TranslationKey>> = {
  alert_reconcile_delayed: 'warning_alert_reconcile_delayed',
  redis_cache_error: 'warning_redis_cache_error',
  theme_active_broken: 'warning_theme_active_broken',
  theme_active_missing: 'warning_theme_active_missing',
};

export const useTopBanner = (): PushBannerWithControls => {
  const ctx = React.useContext(TopBannerContext);
  if (!ctx) {
    throw new Error('useTopBanner must be used within TopBannerProvider');
  }
  return ctx;
};

const toneStyles: Record<
  BannerTone,
  { bg: string; border: string; accent: string; icon: LucideIcon; iconColor: string }
> = {
  info: {
    bg: 'bg-(--theme-bg-success-subtle) dark:bg-(--theme-bg-success-translucent) text-(--theme-fg-default) dark:text-(--theme-fg-strong)',
    border: 'border-(--theme-border-success-muted) dark:border-(--theme-fg-success-on-muted)',
    accent: 'bg-(--theme-bg-success-accent)',
    icon: InfoIcon,
    iconColor: 'text-(--theme-fg-success-strong) dark:text-(--theme-fg-success-muted)',
  },
  warning: {
    bg: 'bg-(--theme-bg-warning-subtle) dark:bg-(--theme-bg-warning-translucent) text-(--theme-fg-default) dark:text-(--theme-fg-strong)',
    border: 'border-(--theme-border-warning-muted) dark:border-(--theme-fg-warning-strong)',
    accent: 'bg-(--theme-bg-warning-accent)',
    icon: AlertTriangle,
    iconColor: 'text-(--theme-fg-warning-muted) dark:text-(--theme-fg-warning)',
  },
  error: {
    bg: 'bg-(--theme-bg-danger-subtle) dark:bg-(--theme-bg-danger-translucent) text-(--theme-fg-default) dark:text-(--theme-fg-strong)',
    border: 'border-(--theme-border-danger-muted) dark:border-(--theme-fg-danger-soft)',
    accent: 'bg-(--theme-fg-danger)',
    icon: XCircle,
    iconColor: 'text-(--theme-fg-danger-muted) dark:text-(--theme-fg-danger)',
  },
};

interface BannerCardProps {
  banner: BannerItem;
  onClose: (id: number) => void;
}

const BannerCard: React.FC<BannerCardProps> = ({ banner, onClose }) => {
  const config = toneStyles[banner.tone];
  const Icon = config.icon;
  const { t } = useI18n();

  return (
    <div
      className={`pointer-events-auto transition-all duration-300 animate-in slide-in-from-top-2 ${
        banner.closing ? 'opacity-0 -translate-y-2' : 'opacity-100 translate-y-0'
      }`}
      style={{ maxWidth: '92vw' }}
    >
      <div
        className={`relative overflow-hidden rounded-lg border shadow-lg theme-shadow-soft backdrop-blur-3xl ${config.bg} ${config.border} inline-flex`}
      >
        <div className={`absolute inset-y-0 left-0 w-1 ${config.accent}`} />
        <div className="flex items-center gap-3 pl-3.5 pr-2 py-2.5 text-sm">
          <div className={`${config.iconColor} flex items-center`}>
            <Icon size={18} />
          </div>
          <div className="flex-1 font-medium text-(--theme-fg-default) dark:text-(--theme-fg-strong) pt-px">
            {banner.message}
          </div>
          <button
            type="button"
            onClick={() => onClose(banner.id)}
            className="rounded-md p-1 text-(--theme-fg-subtle) hover:text-(--theme-fg-default) dark:text-(--theme-fg-control-muted) dark:hover:text-(--theme-fg-control-hover) transition-colors hover:bg-(--theme-surface-inverse-hover) dark:hover:bg-(--theme-surface-inverse-hover)"
            aria-label={t('banner_close')}
          >
            <X size={16} />
          </button>
        </div>
      </div>
    </div>
  );
};

interface TopBannerStackProps {
  banners: BannerItem[];
  onClose: (id: number) => void;
}

const TopBannerStack: React.FC<TopBannerStackProps> = ({ banners, onClose }) => {
  if (banners.length === 0 || typeof document === 'undefined') {
    return null;
  }

  return createPortal(
    <div
      className="fixed top-20 inset-x-0 flex justify-center pointer-events-none z-70 px-4 sm:px-6"
      aria-live="polite"
      aria-atomic="true"
    >
      <div className="flex flex-col gap-2 items-center">
        {banners.map((banner) => (
          <BannerCard key={banner.id} banner={banner} onClose={onClose} />
        ))}
      </div>
    </div>,
    document.body,
  );
};

export const TopBannerProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const [banners, setBanners] = React.useState<BannerItem[]>([]);
  const timersRef = React.useRef<Map<number, number>>(new Map());
  const idRef = React.useRef(0);
  const prevBannerIdsRef = React.useRef<Set<number>>(new Set());

  const clearTimer = React.useCallback((id: number) => {
    const timer = timersRef.current.get(id);
    if (timer) {
      window.clearTimeout(timer);
      timersRef.current.delete(id);
    }
  }, []);

  const finalizeRemoval = React.useCallback(
    (id: number) => {
      setBanners((prev) => prev.filter((item) => item.id !== id));
      clearTimer(id);
    },
    [clearTimer],
  );

  const startClose = React.useCallback(
    (id: number) => {
      clearTimer(id);
      setBanners((prev) =>
        prev.map((item) => (item.id === id ? { ...item, closing: true } : item)),
      );
      window.setTimeout(() => finalizeRemoval(id), 240);
    },
    [clearTimer, finalizeRemoval],
  );

  const pushBanner = React.useCallback<PushBanner>(
    (message, options) => {
      const id = ++idRef.current;
      const durationMs = options?.durationMs ?? 3000;

      setBanners((prev) => {
        const next = [
          ...prev,
          { id, message, tone: options?.tone ?? 'info', closing: false, durationMs },
        ];
        if (next.length > 4) {
          const removalIndex = next.findIndex((item) => item.durationMs !== null);
          const index = removalIndex === -1 ? 0 : removalIndex;
          return next.filter((_, i) => i !== index);
        }
        return next;
      });

      if (durationMs !== null) {
        const timer = window.setTimeout(() => startClose(id), durationMs);
        timersRef.current.set(id, timer);
      }

      return id;
    },
    [startClose],
  );

  const pushBannerWithControls = React.useMemo<PushBannerWithControls>(() => {
    const fn = ((message: string, options?: BannerOptions) =>
      pushBanner(message, options)) as PushBannerWithControls;
    fn.close = startClose;
    return fn;
  }, [pushBanner, startClose]);

  React.useEffect(
    () => () => {
      timersRef.current.forEach((timeoutId) => window.clearTimeout(timeoutId));
      timersRef.current.clear();
    },
    [],
  );

  React.useEffect(() => {
    if (typeof window === 'undefined') return;

    const onWarning = (event: Event) => {
      const detail = (event as CustomEvent<{ code?: string }>).detail;
      const code = typeof detail?.code === 'string' ? detail.code.trim() : '';
      if (!code) return;

      const key = warningKeyByCode[code] ?? 'warning_redis_cache_error';
      pushBannerWithControls(t(key), { tone: 'warning', durationMs: 4500 });
    };

    window.addEventListener(API_WARNING_EVENT, onWarning as EventListener);
    return () => {
      window.removeEventListener(API_WARNING_EVENT, onWarning as EventListener);
    };
  }, [pushBannerWithControls, t]);

  React.useEffect(() => {
    const currentIds = new Set(banners.map((banner) => banner.id));
    prevBannerIdsRef.current.forEach((id) => {
      if (!currentIds.has(id)) {
        clearTimer(id);
      }
    });
    prevBannerIdsRef.current = currentIds;
  }, [banners, clearTimer]);

  return (
    <TopBannerContext.Provider value={pushBannerWithControls}>
      {children}
      <TopBannerStack banners={banners} onClose={startClose} />
    </TopBannerContext.Provider>
  );
};

export default TopBannerStack;
